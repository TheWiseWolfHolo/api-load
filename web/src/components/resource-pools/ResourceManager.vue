<script setup lang="ts">
import { resourcePoolsApi } from "@/api/resourcePools";
import type { ResourceStatus, UpstreamResource } from "@/types/models";
import {
  CreateOutline,
  PauseOutline,
  PlayOutline,
  RefreshOutline,
  SearchOutline,
  TrashOutline,
} from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NCheckbox,
  NEmpty,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NPagination,
  NPopconfirm,
  NSelect,
  NSpin,
  NTag,
  useMessage,
} from "naive-ui";
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{
  poolId: number;
  refreshToken?: number;
}>();

const emit = defineEmits<{
  resourcesDeleted: [count: number];
}>();

const { t } = useI18n();
const message = useMessage();
const resources = ref<UpstreamResource[]>([]);
const loading = ref(false);
const mutating = ref(false);
const search = ref("");
const status = ref<ResourceStatus | "">("");
const page = ref(1);
const pageSize = ref(20);
const totalItems = ref(0);
const totalPages = ref(0);
const selectedIDs = ref<number[]>([]);
const editVisible = ref(false);
const deleteKeysVisible = ref(false);
const deleteKeysText = ref("");
const editingResource = ref<UpstreamResource | null>(null);
const editForm = reactive({ name: "", upstream_url: "", key: "" });
let searchTimer: ReturnType<typeof setTimeout> | undefined;

const statusOptions = computed(() => [
  { label: t("common.all"), value: "" },
  { label: t("resourcePools.status_active"), value: "active" },
  { label: t("resourcePools.status_invalid"), value: "invalid" },
  { label: t("resourcePools.status_disabled"), value: "disabled" },
]);

const selectedSet = computed(() => new Set(selectedIDs.value));
const allPageSelected = computed(
  () => resources.value.length > 0 && resources.value.every(item => selectedSet.value.has(item.id))
);
const somePageSelected = computed(
  () => !allPageSelected.value && resources.value.some(item => selectedSet.value.has(item.id))
);
const parsedDeleteKeys = computed(() => [
  ...new Set(
    deleteKeysText.value
      .split(/[\r\n,]+/)
      .map(key => key.trim())
      .filter(Boolean)
  ),
]);

onMounted(loadResources);
onBeforeUnmount(() => clearTimeout(searchTimer));

watch(
  () => props.refreshToken,
  () => {
    page.value = 1;
    void loadResources();
  }
);

watch(search, () => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    page.value = 1;
    void loadResources();
  }, 300);
});

async function loadResources() {
  loading.value = true;
  selectedIDs.value = [];
  try {
    const result = await resourcePoolsApi.listResources(props.poolId, {
      page: page.value,
      page_size: pageSize.value,
      search: search.value.trim(),
      status: status.value,
    });
    resources.value = result.items ?? [];
    totalItems.value = result.pagination.total_items;
    totalPages.value = result.pagination.total_pages;
    if (resources.value.length === 0 && page.value > 1 && totalItems.value > 0) {
      page.value--;
      await loadResources();
    }
  } finally {
    loading.value = false;
  }
}

function changePage(value: number) {
  page.value = value;
  void loadResources();
}

function changePageSize(value: number) {
  pageSize.value = value;
  page.value = 1;
  void loadResources();
}

function changeStatus(value: ResourceStatus | "") {
  status.value = value;
  page.value = 1;
  void loadResources();
}

function toggleResource(resourceId: number, checked: boolean) {
  const next = new Set(selectedIDs.value);
  if (checked) {
    next.add(resourceId);
  } else {
    next.delete(resourceId);
  }
  selectedIDs.value = [...next];
}

function togglePage(checked: boolean) {
  const next = new Set(selectedIDs.value);
  for (const resource of resources.value) {
    if (checked) {
      next.add(resource.id);
    } else {
      next.delete(resource.id);
    }
  }
  selectedIDs.value = [...next];
}

function openEditor(resource: UpstreamResource) {
  editingResource.value = resource;
  Object.assign(editForm, {
    name: resource.name,
    upstream_url: resource.upstream_url,
    key: "",
  });
  editVisible.value = true;
}

async function saveResource() {
  if (!editingResource.value || !editForm.upstream_url.trim() || mutating.value) {
    return;
  }
  mutating.value = true;
  try {
    await resourcePoolsApi.updateResource(props.poolId, editingResource.value.id, {
      name: editForm.name.trim(),
      upstream_url: editForm.upstream_url.trim(),
      ...(editForm.key.trim() ? { key: editForm.key.trim() } : {}),
    });
    editVisible.value = false;
    await loadResources();
  } finally {
    mutating.value = false;
  }
}

async function updateOneStatus(resource: UpstreamResource) {
  const nextStatus = resource.status === "active" ? "disabled" : "active";
  mutating.value = true;
  try {
    await resourcePoolsApi.updateResourceStatus(props.poolId, resource.id, nextStatus);
    await loadResources();
  } finally {
    mutating.value = false;
  }
}

async function updateSelectedStatus(nextStatus: "active" | "disabled") {
  if (!selectedIDs.value.length || mutating.value) {
    return;
  }
  mutating.value = true;
  try {
    await resourcePoolsApi.bulkUpdateResourceStatus(props.poolId, selectedIDs.value, nextStatus);
    await loadResources();
  } finally {
    mutating.value = false;
  }
}

async function deleteSelected() {
  if (!selectedIDs.value.length || mutating.value) {
    return;
  }
  await runBulkDelete({ resource_ids: selectedIDs.value });
}

async function deleteOne(resourceId: number) {
  if (mutating.value) {
    return;
  }
  await runBulkDelete({ resource_ids: [resourceId] });
}

async function deleteByKeys() {
  if (!parsedDeleteKeys.value.length || mutating.value) {
    return;
  }
  const succeeded = await runBulkDelete({ keys: parsedDeleteKeys.value });
  if (succeeded) {
    deleteKeysVisible.value = false;
    deleteKeysText.value = "";
  }
}

async function runBulkDelete(payload: { resource_ids?: number[]; keys?: string[] }) {
  mutating.value = true;
  try {
    const result = await resourcePoolsApi.bulkDeleteResources(props.poolId, payload);
    if (result.deleted_count > 0) {
      emit("resourcesDeleted", result.deleted_count);
    }
    if (result.blocked_count > 0) {
      message.warning(t("resourcePools.deleteBlocked", { count: result.blocked_count }));
    }
    if (result.missing_key_count > 0) {
      message.warning(t("resourcePools.deleteKeysMissing", { count: result.missing_key_count }));
    }
    await loadResources();
    return true;
  } finally {
    mutating.value = false;
  }
}

function statusType(value: ResourceStatus): "success" | "warning" | "error" {
  if (value === "active") {
    return "success";
  }
  if (value === "invalid") {
    return "error";
  }
  return "warning";
}

function formatDate(value?: string): string {
  if (!value) {
    return "—";
  }
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value)
  );
}
</script>

<template>
  <div class="resource-manager">
    <div class="manager-toolbar">
      <n-input
        v-model:value="search"
        clearable
        class="resource-search"
        :placeholder="t('resourcePools.searchPlaceholder')"
      >
        <template #prefix><n-icon :component="SearchOutline" /></template>
      </n-input>
      <n-select
        :value="status"
        class="status-filter"
        :options="statusOptions"
        @update:value="changeStatus"
      />
      <n-button quaternary :loading="loading" @click="loadResources">
        <template #icon><n-icon :component="RefreshOutline" /></template>
        {{ t("common.refresh") }}
      </n-button>
      <n-button secondary type="error" @click="deleteKeysVisible = true">
        {{ t("resourcePools.deleteByKeys") }}
      </n-button>
    </div>

    <div v-if="selectedIDs.length" class="selection-bar" aria-live="polite">
      <strong>{{ t("resourcePools.selectedCount", { count: selectedIDs.length }) }}</strong>
      <div>
        <n-button size="small" :disabled="mutating" @click="updateSelectedStatus('active')">
          {{ t("resourcePools.bulkEnable") }}
        </n-button>
        <n-button size="small" :disabled="mutating" @click="updateSelectedStatus('disabled')">
          {{ t("resourcePools.bulkDisable") }}
        </n-button>
        <n-popconfirm @positive-click="deleteSelected">
          <template #trigger>
            <n-button size="small" type="error" :disabled="mutating">
              {{ t("common.delete") }}
            </n-button>
          </template>
          {{ t("resourcePools.deleteSelectedConfirm", { count: selectedIDs.length }) }}
        </n-popconfirm>
      </div>
    </div>

    <n-spin :show="loading">
      <div v-if="resources.length" class="resource-table-wrap">
        <div class="resource-table" role="table">
          <div class="resource-row resource-heading" role="row">
            <n-checkbox
              :checked="allPageSelected"
              :indeterminate="somePageSelected"
              :aria-label="t('resourcePools.selectPage')"
              @update:checked="togglePage"
            />
            <span>{{ t("resourcePools.resource") }}</span>
            <span>{{ t("resourcePools.upstream") }}</span>
            <span>{{ t("resourcePools.status") }}</span>
            <span>{{ t("resourcePools.lastUsed") }}</span>
            <span class="actions-heading">{{ t("common.actions") }}</span>
          </div>
          <div v-for="resource in resources" :key="resource.id" class="resource-row" role="row">
            <n-checkbox
              :checked="selectedSet.has(resource.id)"
              :aria-label="
                t('resourcePools.selectResource', { name: resource.name || resource.id })
              "
              @update:checked="checked => toggleResource(resource.id, checked)"
            />
            <div class="resource-name" role="cell">
              <strong>{{ resource.name || `#${resource.id}` }}</strong>
              <code>{{ resource.masked_key }}</code>
            </div>
            <div class="resource-url" role="cell" :title="resource.upstream_url">
              {{ resource.upstream_url }}
            </div>
            <div class="resource-status" role="cell">
              <n-tag size="small" :type="statusType(resource.status)">
                {{ t(`resourcePools.status_${resource.status}`) }}
              </n-tag>
              <small v-if="resource.global_cooldown_until">
                {{
                  t("resourcePools.cooldownUntil", {
                    value: formatDate(resource.global_cooldown_until),
                  })
                }}
              </small>
              <small v-else-if="resource.disabled_reason">{{ resource.disabled_reason }}</small>
            </div>
            <time role="cell">{{ formatDate(resource.last_used_at) }}</time>
            <div class="resource-actions" role="cell">
              <n-button
                size="tiny"
                quaternary
                :aria-label="t('common.edit')"
                @click="openEditor(resource)"
              >
                <template #icon><n-icon :component="CreateOutline" /></template>
              </n-button>
              <n-button
                size="tiny"
                secondary
                :type="resource.status === 'active' ? 'default' : 'success'"
                :disabled="mutating"
                @click="updateOneStatus(resource)"
              >
                <template #icon>
                  <n-icon :component="resource.status === 'active' ? PauseOutline : PlayOutline" />
                </template>
                {{
                  resource.status === "active"
                    ? t("resourcePools.disable")
                    : t("resourcePools.restore")
                }}
              </n-button>
              <n-popconfirm @positive-click="deleteOne(resource.id)">
                <template #trigger>
                  <n-button size="tiny" quaternary type="error" :aria-label="t('common.delete')">
                    <template #icon><n-icon :component="TrashOutline" /></template>
                  </n-button>
                </template>
                {{ t("resourcePools.deleteResourceConfirm") }}
              </n-popconfirm>
            </div>
          </div>
        </div>
      </div>
      <n-empty
        v-else
        class="manager-empty"
        size="small"
        :description="t('resourcePools.noMatchingResources')"
      />
    </n-spin>

    <div class="pagination-row">
      <span>{{ t("resourcePools.totalResources", { count: totalItems }) }}</span>
      <n-pagination
        :page="page"
        :page-size="pageSize"
        :page-count="Math.max(totalPages, 1)"
        :page-sizes="[20, 50, 100]"
        show-size-picker
        @update:page="changePage"
        @update:page-size="changePageSize"
      />
    </div>

    <n-modal v-model:show="editVisible">
      <n-card class="manager-modal" :bordered="false" :title="t('resourcePools.editResource')">
        <n-form label-placement="top">
          <n-form-item :label="t('resourcePools.resourceName')">
            <n-input
              v-model:value="editForm.name"
              :placeholder="t('resourcePools.resourceNamePlaceholder')"
            />
          </n-form-item>
          <n-form-item :label="t('resourcePools.upstreamURL')" required>
            <n-input v-model:value="editForm.upstream_url" spellcheck="false" />
          </n-form-item>
          <n-form-item :label="t('resourcePools.replaceKey')">
            <n-input
              v-model:value="editForm.key"
              type="password"
              show-password-on="click"
              :placeholder="t('resourcePools.keepExistingKey')"
              spellcheck="false"
            />
          </n-form-item>
        </n-form>
        <template #footer>
          <div class="modal-actions">
            <n-button @click="editVisible = false">{{ t("common.cancel") }}</n-button>
            <n-button type="primary" :loading="mutating" @click="saveResource">
              {{ t("common.save") }}
            </n-button>
          </div>
        </template>
      </n-card>
    </n-modal>

    <n-modal v-model:show="deleteKeysVisible">
      <n-card class="manager-modal" :bordered="false" :title="t('resourcePools.deleteByKeys')">
        <p class="modal-help">{{ t("resourcePools.deleteByKeysHelp") }}</p>
        <n-input
          v-model:value="deleteKeysText"
          type="textarea"
          :rows="9"
          :placeholder="t('resourcePools.deleteKeysPlaceholder')"
          spellcheck="false"
        />
        <p class="detected-count">
          {{ t("resourcePools.keysDetected", { count: parsedDeleteKeys.length }) }}
        </p>
        <template #footer>
          <div class="modal-actions">
            <n-button @click="deleteKeysVisible = false">{{ t("common.cancel") }}</n-button>
            <n-button
              type="error"
              :disabled="parsedDeleteKeys.length === 0"
              :loading="mutating"
              @click="deleteByKeys"
            >
              {{ t("resourcePools.confirmDeleteKeys") }}
            </n-button>
          </div>
        </template>
      </n-card>
    </n-modal>
  </div>
</template>

<style scoped>
.resource-manager {
  min-width: 0;
  background: var(--bg-primary);
}

.manager-toolbar,
.selection-bar,
.selection-bar > div,
.pagination-row,
.modal-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.manager-toolbar {
  flex-wrap: wrap;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border-color-light);
}

.resource-search {
  flex: 1 1 320px;
  max-width: 520px;
}

.status-filter {
  width: 140px;
}

.selection-bar {
  justify-content: space-between;
  padding: 9px 20px;
  color: var(--text-primary);
  background: var(--primary-color-suppl);
  border-bottom: 1px solid var(--border-color-light);
}

.resource-table-wrap {
  max-width: 100%;
  overflow-x: auto;
}

.resource-table {
  min-width: 980px;
}

.resource-row {
  display: grid;
  grid-template-columns:
    28px
    minmax(150px, 1fr)
    minmax(240px, 1.6fr)
    minmax(150px, 0.8fr)
    minmax(140px, 0.7fr)
    minmax(185px, auto);
  gap: 14px;
  align-items: center;
  padding: 11px 20px;
  border-bottom: 1px solid var(--border-color-light);
}

.resource-heading {
  color: var(--text-tertiary);
  background: var(--bg-secondary);
  font-size: 0.78rem;
  font-weight: 600;
}

.resource-name,
.resource-status {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 3px;
}

.resource-name code,
.resource-url,
.resource-row time,
.resource-status small {
  color: var(--text-secondary);
  font-size: 0.78rem;
}

.resource-name code {
  font-family: var(--font-mono);
}

.resource-url {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-actions {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
}

.actions-heading {
  text-align: right;
}

.manager-empty {
  padding: 30px 20px;
}

.pagination-row {
  justify-content: space-between;
  flex-wrap: wrap;
  min-height: 58px;
  padding: 10px 20px;
  color: var(--text-secondary);
  border-top: 1px solid var(--border-color-light);
}

.manager-modal {
  width: min(560px, calc(100vw - 28px));
}

.modal-actions {
  justify-content: flex-end;
}

.modal-help,
.detected-count {
  color: var(--text-secondary);
}

.modal-help {
  margin-top: 0;
}

.detected-count {
  margin-bottom: 0;
  font-size: 0.8rem;
}

@media (max-width: 720px) {
  .manager-toolbar {
    align-items: stretch;
  }

  .resource-search,
  .status-filter {
    width: 100%;
    max-width: none;
  }

  .selection-bar {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
