<script setup lang="ts">
import { resourcePoolsApi } from "@/api/resourcePools";
import type { ResourceStatus, ResourceValidationGroup, UpstreamResource } from "@/types/models";
import {
  CreateOutline,
  DownloadOutline,
  PulseOutline,
  RefreshOutline,
  SearchOutline,
  SettingsOutline,
  TrashOutline,
} from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NCheckbox,
  NDropdown,
  NEmpty,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NModal,
  NPagination,
  NPopconfirm,
  NSelect,
  NSpin,
  NSwitch,
  NTag,
  useMessage,
} from "naive-ui";
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{ poolId: number; refreshToken?: number }>();
const emit = defineEmits<{ resourcesDeleted: [count: number] }>();
const { t } = useI18n();
const message = useMessage();

const resources = ref<UpstreamResource[]>([]);
const validationGroups = ref<ResourceValidationGroup[]>([]);
const loading = ref(false);
const validationGroupsLoading = ref(false);
const testingResourceID = ref<number | null>(null);
const mutating = ref(false);
const search = ref("");
const health = ref<ResourceStatus | "">("");
const availability = ref<"" | "enabled" | "disabled">("");
const page = ref(1);
const pageSize = ref(20);
const totalItems = ref(0);
const totalPages = ref(0);
const selectedIDs = ref<number[]>([]);
const editVisible = ref(false);
const scheduleVisible = ref(false);
const deleteKeysVisible = ref(false);
const deleteKeysText = ref("");
const editingResource = ref<UpstreamResource | null>(null);
const editForm = reactive({
  name: "",
  upstream_url: "",
  key: "",
  enabled: true,
  priority: 10,
  weight: 1,
});
const scheduleForm = reactive({ priority: 10, weight: 1 });
let searchTimer: ReturnType<typeof setTimeout> | undefined;

const healthOptions = computed(() => [
  { label: t("common.all"), value: "" },
  { label: t("resourcePools.status_active"), value: "active" },
  { label: t("resourcePools.status_invalid"), value: "invalid" },
]);
const availabilityOptions = computed(() => [
  { label: t("resourcePools.availabilityAll"), value: "" },
  { label: t("resourcePools.enabledOnly"), value: "enabled" },
  { label: t("resourcePools.disabledOnly"), value: "disabled" },
]);
const exportOptions = computed(() => [
  { label: t("resourcePools.exportFullJSONL"), key: "full-jsonl" },
  { label: t("resourcePools.exportFullCSV"), key: "full-csv" },
  {
    label: t("resourcePools.exportKeysOnly"),
    key: "keys-menu",
    children: [
      { label: t("resourcePools.exportAllKeys"), key: "keys-all" },
      { label: t("resourcePools.exportValidKeys"), key: "keys-active" },
      { label: t("resourcePools.exportInvalidKeys"), key: "keys-invalid" },
      { label: t("resourcePools.exportDisabledKeys"), key: "keys-disabled" },
    ],
  },
]);
const validationRouteOptions = computed(() =>
  validationGroups.value.map(group => ({
    label: `${group.display_name || group.name} · ${group.channel_type}`,
    key: group.id,
  }))
);
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

onMounted(() => {
  void loadResources();
  void loadValidationGroups();
});
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
      status: health.value,
      enabled:
        availability.value === "" ? undefined : availability.value === "enabled" ? true : false,
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

async function loadValidationGroups() {
  validationGroupsLoading.value = true;
  try {
    validationGroups.value = await resourcePoolsApi.listValidationGroups(props.poolId);
  } finally {
    validationGroupsLoading.value = false;
  }
}

function reloadFromFirstPage() {
  page.value = 1;
  void loadResources();
}
function changePage(value: number) {
  page.value = value;
  void loadResources();
}
function changePageSize(value: number) {
  pageSize.value = value;
  reloadFromFirstPage();
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
    enabled: resource.enabled,
    priority: resource.priority,
    weight: resource.weight,
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
      enabled: editForm.enabled,
      priority: editForm.priority,
      weight: editForm.weight,
      ...(editForm.key.trim() ? { key: editForm.key.trim() } : {}),
    });
    editVisible.value = false;
    await loadResources();
  } finally {
    mutating.value = false;
  }
}

async function toggleEnabled(resource: UpstreamResource) {
  mutating.value = true;
  try {
    await resourcePoolsApi.updateResource(props.poolId, resource.id, {
      name: resource.name,
      upstream_url: resource.upstream_url,
      enabled: !resource.enabled,
    });
    await loadResources();
  } finally {
    mutating.value = false;
  }
}
async function restoreHealth(resource: UpstreamResource) {
  mutating.value = true;
  try {
    await resourcePoolsApi.updateResource(props.poolId, resource.id, {
      name: resource.name,
      upstream_url: resource.upstream_url,
      status: "active",
    });
    await loadResources();
  } finally {
    mutating.value = false;
  }
}
async function updateSelectedEnabled(enabled: boolean) {
  if (!selectedIDs.value.length || mutating.value) {
    return;
  }
  mutating.value = true;
  try {
    await resourcePoolsApi.bulkUpdateResources(props.poolId, selectedIDs.value, { enabled });
    await loadResources();
  } finally {
    mutating.value = false;
  }
}
function openScheduleEditor() {
  const selected = resources.value.find(item => selectedSet.value.has(item.id));
  scheduleForm.priority = selected?.priority ?? 10;
  scheduleForm.weight = selected?.weight ?? 1;
  scheduleVisible.value = true;
}
async function saveSelectedSchedule() {
  if (!selectedIDs.value.length || mutating.value) {
    return;
  }
  mutating.value = true;
  try {
    await resourcePoolsApi.bulkUpdateResources(props.poolId, selectedIDs.value, {
      priority: scheduleForm.priority,
      weight: scheduleForm.weight,
    });
    scheduleVisible.value = false;
    await loadResources();
  } finally {
    mutating.value = false;
  }
}

async function deleteSelected() {
  if (selectedIDs.value.length && !mutating.value) {
    await runBulkDelete({ resource_ids: selectedIDs.value });
  }
}
async function deleteOne(resourceId: number) {
  if (!mutating.value) {
    await runBulkDelete({ resource_ids: [resourceId] });
  }
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

function handleExport(key: string | number) {
  const value = String(key);
  if (value === "full-jsonl" || value === "full-csv") {
    resourcePoolsApi.exportResources(props.poolId, {
      content: "full",
      format: value === "full-jsonl" ? "jsonl" : "csv",
    });
    return;
  }
  if (value.startsWith("keys-")) {
    const status = value.slice(5) as "all" | "active" | "invalid" | "disabled";
    resourcePoolsApi.exportResources(props.poolId, { content: "keys", format: "txt", status });
  }
}
async function testResource(resource: UpstreamResource, selectedGroupID?: number) {
  const groupID = selectedGroupID ?? validationGroups.value[0]?.id;
  if (!groupID || testingResourceID.value !== null) {
    return;
  }
  testingResourceID.value = resource.id;
  try {
    const result = await resourcePoolsApi.testResource(props.poolId, resource.id, groupID);
    if (result.is_valid) {
      message.success(t("resourcePools.testSuccess", { duration: result.duration_ms }));
    } else {
      message.error(
        t("resourcePools.testFailed", {
          error: result.error || t("resourcePools.testUnknownError"),
        })
      );
    }
    await loadResources();
  } finally {
    testingResourceID.value = null;
  }
}
function isResourceTestDisabled(): boolean {
  return (
    mutating.value ||
    validationGroupsLoading.value ||
    validationGroups.value.length === 0 ||
    testingResourceID.value !== null
  );
}
function healthType(value: ResourceStatus): "success" | "error" {
  return value === "active" ? "success" : "error";
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
      <div class="toolbar-summary">
        <strong>{{ t("resourcePools.totalResources", { count: totalItems }) }}</strong>
        <span>{{ t("resourcePools.schedulerHint") }}</span>
      </div>
      <n-input
        v-model:value="search"
        clearable
        class="resource-search"
        :placeholder="t('resourcePools.searchPlaceholder')"
      >
        <template #prefix><n-icon :component="SearchOutline" /></template>
      </n-input>
      <n-select
        v-model:value="health"
        class="filter-select"
        :options="healthOptions"
        @update:value="reloadFromFirstPage"
      />
      <n-select
        v-model:value="availability"
        class="filter-select"
        :options="availabilityOptions"
        @update:value="reloadFromFirstPage"
      />
      <n-dropdown :options="exportOptions" trigger="click" @select="handleExport">
        <n-button secondary>
          <template #icon><n-icon :component="DownloadOutline" /></template>
          {{ t("resourcePools.exportResources") }}
        </n-button>
      </n-dropdown>
      <n-button quaternary :loading="loading" @click="loadResources">
        <template #icon><n-icon :component="RefreshOutline" /></template>
      </n-button>
      <n-button secondary type="error" @click="deleteKeysVisible = true">
        {{ t("resourcePools.deleteByKeys") }}
      </n-button>
    </div>

    <div v-if="selectedIDs.length" class="selection-bar" aria-live="polite">
      <strong>{{ t("resourcePools.selectedCount", { count: selectedIDs.length }) }}</strong>
      <div class="selection-actions">
        <n-button size="small" :disabled="mutating" @click="updateSelectedEnabled(true)">
          {{ t("resourcePools.bulkEnable") }}
        </n-button>
        <n-button size="small" :disabled="mutating" @click="updateSelectedEnabled(false)">
          {{ t("resourcePools.bulkDisable") }}
        </n-button>
        <n-button size="small" :disabled="mutating" @click="openScheduleEditor">
          <template #icon><n-icon :component="SettingsOutline" /></template>
          {{ t("resourcePools.setScheduling") }}
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
            <span>{{ t("resourcePools.scheduling") }}</span>
            <span>{{ t("resourcePools.status") }}</span>
            <span>{{ t("resourcePools.usage") }}</span>
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
            <div class="stacked resource-name" role="cell">
              <strong>{{ resource.name || `#${resource.id}` }}</strong>
              <code>{{ resource.masked_key }}</code>
            </div>
            <div class="resource-url" role="cell" :title="resource.upstream_url">
              {{ resource.upstream_url }}
            </div>
            <div class="stacked compact-data" role="cell">
              <span>{{ t("resourcePools.priorityValue", { value: resource.priority }) }}</span>
              <span>{{ t("resourcePools.weightValue", { value: resource.weight }) }}</span>
            </div>
            <div class="stacked status-stack" role="cell">
              <div class="tag-row">
                <n-tag size="small" :type="healthType(resource.status)">
                  {{ t(`resourcePools.status_${resource.status}`) }}
                </n-tag>
                <n-tag v-if="!resource.enabled" size="small" type="warning">
                  {{ t("resourcePools.manualDisabled") }}
                </n-tag>
              </div>
              <small v-if="resource.global_cooldown_until">
                {{
                  t("resourcePools.cooldownUntil", {
                    value: formatDate(resource.global_cooldown_until),
                  })
                }}
              </small>
              <small v-else-if="resource.disabled_reason">{{ resource.disabled_reason }}</small>
            </div>
            <div class="stacked compact-data" role="cell">
              <strong>
                {{ t("resourcePools.callsValue", { value: resource.request_count }) }}
              </strong>
              <span>
                {{ t("resourcePools.failuresValue", { value: resource.total_failure_count }) }}
              </span>
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
              <n-button size="tiny" secondary :disabled="mutating" @click="toggleEnabled(resource)">
                {{ resource.enabled ? t("common.disable") : t("resourcePools.enable") }}
              </n-button>
              <n-dropdown
                v-if="validationGroups.length > 1"
                :options="validationRouteOptions"
                trigger="click"
                @select="groupId => testResource(resource, Number(groupId))"
              >
                <n-button
                  size="tiny"
                  secondary
                  :disabled="isResourceTestDisabled()"
                  :loading="testingResourceID === resource.id"
                >
                  <template #icon><n-icon :component="PulseOutline" /></template>
                  {{ t("resourcePools.testKey") }}
                </n-button>
              </n-dropdown>
              <n-button
                v-else
                size="tiny"
                secondary
                :title="
                  validationGroups.length === 0 ? t('resourcePools.testRequiresGroup') : undefined
                "
                :disabled="isResourceTestDisabled()"
                :loading="testingResourceID === resource.id"
                @click="testResource(resource)"
              >
                <template #icon><n-icon :component="PulseOutline" /></template>
                {{ t("resourcePools.testKey") }}
              </n-button>
              <n-button
                v-if="resource.status === 'invalid'"
                size="tiny"
                secondary
                type="success"
                :disabled="mutating"
                @click="restoreHealth(resource)"
              >
                {{ t("resourcePools.restoreHealth") }}
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
          <div class="form-grid">
            <n-form-item :label="t('resourcePools.resourceName')">
              <n-input
                v-model:value="editForm.name"
                :placeholder="t('resourcePools.resourceNamePlaceholder')"
              />
            </n-form-item>
            <n-form-item :label="t('resourcePools.enabledState')">
              <n-switch v-model:value="editForm.enabled" />
            </n-form-item>
          </div>
          <n-form-item :label="t('resourcePools.upstreamURL')" required>
            <n-input v-model:value="editForm.upstream_url" spellcheck="false" />
          </n-form-item>
          <div class="form-grid two-equal">
            <n-form-item :label="t('resourcePools.priority')">
              <n-input-number v-model:value="editForm.priority" :min="1" :max="1000" />
            </n-form-item>
            <n-form-item :label="t('resourcePools.weight')">
              <n-input-number v-model:value="editForm.weight" :min="1" :max="1000" />
            </n-form-item>
          </div>
          <p class="field-help">{{ t("resourcePools.schedulingHelp") }}</p>
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

    <n-modal v-model:show="scheduleVisible">
      <n-card
        class="manager-modal compact-modal"
        :bordered="false"
        :title="t('resourcePools.setScheduling')"
      >
        <p class="modal-help">
          {{ t("resourcePools.batchSchedulingHelp", { count: selectedIDs.length }) }}
        </p>
        <div class="form-grid two-equal">
          <n-form-item :label="t('resourcePools.priority')">
            <n-input-number v-model:value="scheduleForm.priority" :min="1" :max="1000" />
          </n-form-item>
          <n-form-item :label="t('resourcePools.weight')">
            <n-input-number v-model:value="scheduleForm.weight" :min="1" :max="1000" />
          </n-form-item>
        </div>
        <p class="field-help">{{ t("resourcePools.schedulingHelp") }}</p>
        <template #footer>
          <div class="modal-actions">
            <n-button @click="scheduleVisible = false">{{ t("common.cancel") }}</n-button>
            <n-button type="primary" :loading="mutating" @click="saveSelectedSchedule">
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
.selection-actions,
.pagination-row,
.modal-actions,
.tag-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.manager-toolbar {
  flex-wrap: wrap;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border-color-light);
}
.toolbar-summary {
  display: flex;
  flex-direction: column;
  min-width: 148px;
  color: var(--text-primary);
}
.toolbar-summary span {
  color: var(--text-secondary);
  font-size: 0.75rem;
}
.resource-search {
  flex: 1 1 280px;
  max-width: 480px;
}
.filter-select {
  width: 132px;
}
.selection-bar {
  justify-content: space-between;
  flex-wrap: wrap;
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
  min-width: 1240px;
}
.resource-row {
  display: grid;
  grid-template-columns:
    28px minmax(140px, 0.9fr) minmax(210px, 1.4fr) 105px minmax(170px, 1fr)
    110px 145px minmax(180px, auto);
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
.stacked {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 3px;
  min-width: 0;
}
.resource-name code,
.resource-url,
.resource-row time,
.status-stack small,
.compact-data {
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
.compact-data strong {
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}
.resource-actions {
  display: flex;
  flex-wrap: wrap;
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
  justify-content: flex-end;
  flex-wrap: wrap;
  min-height: 58px;
  padding: 10px 20px;
  border-top: 1px solid var(--border-color-light);
}
.manager-modal {
  width: min(600px, calc(100vw - 28px));
}
.compact-modal {
  width: min(480px, calc(100vw - 28px));
}
.modal-actions {
  justify-content: flex-end;
}
.modal-help,
.detected-count,
.field-help {
  color: var(--text-secondary);
}
.modal-help {
  margin-top: 0;
}
.detected-count {
  margin-bottom: 0;
  font-size: 0.8rem;
}
.field-help {
  margin: -8px 0 18px;
  font-size: 0.78rem;
}
.form-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 120px;
  gap: 16px;
}
.form-grid.two-equal {
  grid-template-columns: 1fr 1fr;
}
.form-grid :deep(.n-input-number) {
  width: 100%;
}
@media (max-width: 720px) {
  .manager-toolbar {
    align-items: stretch;
  }
  .resource-search,
  .filter-select {
    width: 100%;
    max-width: none;
  }
  .selection-bar {
    align-items: flex-start;
    flex-direction: column;
  }
  .form-grid,
  .form-grid.two-equal {
    grid-template-columns: 1fr;
    gap: 0;
  }
}
</style>
