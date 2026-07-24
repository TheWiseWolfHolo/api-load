<script setup lang="ts">
import { resourcePoolsApi } from "@/api/resourcePools";
import ResourceManager from "@/components/resource-pools/ResourceManager.vue";
import type { ResourcePool, ResourcePoolInput, UpstreamResourceInput } from "@/types/models";
import {
  AddOutline,
  ChevronDownOutline,
  ChevronForwardOutline,
  CreateOutline,
  DocumentTextOutline,
  RefreshOutline,
  TrashOutline,
} from "@vicons/ionicons5";
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSkeleton,
  NTag,
  useMessage,
  type FormRules,
} from "naive-ui";
import { computed, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n();
const message = useMessage();
const pools = ref<ResourcePool[]>([]);
const loading = ref(true);
const poolModalVisible = ref(false);
const resourcesModalVisible = ref(false);
const savingPool = ref(false);
const savingResources = ref(false);
const editingPoolID = ref<number | null>(null);
const targetPool = ref<ResourcePool | null>(null);
const expandedPoolIDs = ref(new Set<number>());
const managerRefreshTokens = reactive<Record<number, number>>({});
const poolFormRef = ref();
const resourceUpstreamURL = ref("");
const resourceKeysText = ref("");
const resourcePriority = ref(10);
const resourceWeight = ref(1);
const resourceImportMode = ref<"batch" | "config">("batch");
const resourceImportContent = ref("");

const parsedResourceKeys = computed(() =>
  resourceKeysText.value
    .split(/[\r\n,]+/)
    .map(key => key.trim())
    .filter(Boolean)
);
const uniqueResourceKeys = computed(() => [...new Set(parsedResourceKeys.value)]);
const duplicateResourceKeyCount = computed(
  () => parsedResourceKeys.value.length - uniqueResourceKeys.value.length
);

const poolForm = reactive<ResourcePoolInput>({
  name: "",
  description: "",
  strategy: "round_robin",
  affinity_ttl_seconds: 3600,
  busy_wait_milliseconds: 2000,
  auto_restore_schedule: "",
});

const poolRules: FormRules = {
  name: [
    { required: true, message: () => t("resourcePools.nameRequired"), trigger: ["input", "blur"] },
  ],
  affinity_ttl_seconds: [
    {
      type: "number",
      min: 60,
      max: 604800,
      message: () => t("resourcePools.ttlRange"),
      trigger: ["input", "blur"],
    },
  ],
  busy_wait_milliseconds: [
    {
      type: "number",
      min: 0,
      max: 10000,
      message: () => t("resourcePools.waitRange"),
      trigger: ["input", "blur"],
    },
  ],
};

void loadPools();

async function loadPools() {
  loading.value = true;
  try {
    pools.value = await resourcePoolsApi.listPools();
  } finally {
    loading.value = false;
  }
}

function openPoolEditor(pool?: ResourcePool) {
  editingPoolID.value = pool?.id ?? null;
  Object.assign(poolForm, {
    name: pool?.name ?? "",
    description: pool?.description ?? "",
    strategy: "round_robin",
    affinity_ttl_seconds: pool?.affinity_ttl_seconds ?? 3600,
    busy_wait_milliseconds: pool?.busy_wait_milliseconds ?? 2000,
    auto_restore_schedule: pool?.auto_restore_schedule ?? "",
  });
  poolModalVisible.value = true;
}

async function savePool() {
  if (savingPool.value) {
    return;
  }
  await poolFormRef.value?.validate();
  savingPool.value = true;
  try {
    if (editingPoolID.value) {
      await resourcePoolsApi.updatePool(editingPoolID.value, poolForm);
    } else {
      await resourcePoolsApi.createPool(poolForm);
    }
    poolModalVisible.value = false;
    await loadPools();
  } finally {
    savingPool.value = false;
  }
}

function openResourceImporter(pool: ResourcePool, mode: "batch" | "config" = "batch") {
  targetPool.value = pool;
  resourceImportMode.value = mode;
  resourceUpstreamURL.value = "";
  resourceKeysText.value = "";
  resourcePriority.value = 10;
  resourceWeight.value = 1;
  resourceImportContent.value = "";
  resourcesModalVisible.value = true;
}

function parseResources(): UpstreamResourceInput[] | null {
  const upstreamURL = resourceUpstreamURL.value.trim();
  try {
    const parsed = new URL(upstreamURL);
    if (!/^https?:$/.test(parsed.protocol)) {
      throw new Error("invalid upstream URL");
    }
  } catch {
    message.error(t("resourcePools.invalidUpstream"));
    return null;
  }
  if (uniqueResourceKeys.value.length === 0) {
    message.warning(t("resourcePools.resourcesRequired"));
    return null;
  }
  return uniqueResourceKeys.value.map(key => ({
    name: "",
    upstream_url: upstreamURL,
    key,
    priority: resourcePriority.value,
    weight: resourceWeight.value,
  }));
}

async function addResources() {
  if (!targetPool.value || savingResources.value) {
    return;
  }
  savingResources.value = true;
  try {
    const poolID = targetPool.value.id;
    if (resourceImportMode.value === "config") {
      if (!resourceImportContent.value.trim()) {
        message.warning(t("resourcePools.importContentRequired"));
        return;
      }
      await resourcePoolsApi.importResources(poolID, resourceImportContent.value);
    } else {
      const resources = parseResources();
      if (!resources) {
        return;
      }
      await resourcePoolsApi.addResources(poolID, resources);
    }
    resourcesModalVisible.value = false;
    resourceUpstreamURL.value = "";
    resourceKeysText.value = "";
    await loadPools();
    expandedPoolIDs.value = new Set([...expandedPoolIDs.value, poolID]);
    managerRefreshTokens[poolID] = (managerRefreshTokens[poolID] ?? 0) + 1;
  } finally {
    savingResources.value = false;
  }
}

async function deletePool(pool: ResourcePool) {
  await resourcePoolsApi.deletePool(pool.id);
  await loadPools();
}

function toggleResourceManager(poolID: number) {
  const next = new Set(expandedPoolIDs.value);
  if (next.has(poolID)) {
    next.delete(poolID);
  } else {
    next.add(poolID);
  }
  expandedPoolIDs.value = next;
}

function handleResourcesDeleted(pool: ResourcePool, count: number) {
  pool.resource_count = Math.max(0, pool.resource_count - count);
}

function formatTTL(seconds: number): string {
  if (seconds % 3600 === 0) {
    return t("resourcePools.hours", { value: seconds / 3600 });
  }
  if (seconds % 60 === 0) {
    return t("resourcePools.minutes", { value: seconds / 60 });
  }
  return t("resourcePools.seconds", { value: seconds });
}
</script>

<template>
  <div class="resource-pools-page">
    <header class="page-header">
      <div>
        <h1>{{ t("resourcePools.title") }}</h1>
        <p>{{ t("resourcePools.subtitle") }}</p>
      </div>
      <div class="header-actions">
        <n-button secondary :loading="loading" @click="loadPools">
          <template #icon><n-icon :component="RefreshOutline" /></template>
          {{ t("common.refresh") }}
        </n-button>
        <n-button type="primary" @click="openPoolEditor()">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t("resourcePools.createPool") }}
        </n-button>
      </div>
    </header>

    <n-alert type="info" :bordered="false" class="routing-note">
      {{ t("resourcePools.routingNote") }}
    </n-alert>

    <div v-if="loading" class="loading-stack" aria-live="polite">
      <n-skeleton v-for="index in 2" :key="index" height="190px" :sharp="false" />
    </div>

    <n-empty
      v-else-if="pools.length === 0"
      class="empty-state"
      :description="t('resourcePools.empty')"
    >
      <template #extra>
        <n-button type="primary" @click="openPoolEditor()">
          {{ t("resourcePools.createFirst") }}
        </n-button>
      </template>
    </n-empty>

    <div v-else class="pool-list">
      <section v-for="pool in pools" :key="pool.id" class="pool-panel">
        <div class="pool-header">
          <div class="pool-identity">
            <div class="pool-title-row">
              <h2>{{ pool.name }}</h2>
              <n-tag size="small" :bordered="false">{{ pool.resource_count }}</n-tag>
            </div>
            <p v-if="pool.description">{{ pool.description }}</p>
            <p v-else class="muted">{{ t("resourcePools.noDescription") }}</p>
          </div>
          <div class="pool-actions">
            <n-button
              v-if="pool.resource_count > 0"
              size="small"
              secondary
              @click="toggleResourceManager(pool.id)"
            >
              <template #icon>
                <n-icon
                  :component="
                    expandedPoolIDs.has(pool.id) ? ChevronDownOutline : ChevronForwardOutline
                  "
                />
              </template>
              {{
                expandedPoolIDs.has(pool.id)
                  ? t("resourcePools.hideResources")
                  : t("resourcePools.manageResources")
              }}
            </n-button>
            <n-button size="small" @click="openResourceImporter(pool)">
              <template #icon><n-icon :component="AddOutline" /></template>
              {{ t("resourcePools.addResources") }}
            </n-button>
            <n-button size="small" quaternary @click="openResourceImporter(pool, 'config')">
              <template #icon><n-icon :component="DocumentTextOutline" /></template>
              {{ t("resourcePools.importConfig") }}
            </n-button>
            <n-button size="small" quaternary @click="openPoolEditor(pool)">
              <template #icon><n-icon :component="CreateOutline" /></template>
              {{ t("common.edit") }}
            </n-button>
            <n-popconfirm @positive-click="deletePool(pool)">
              <template #trigger>
                <n-button
                  size="small"
                  quaternary
                  type="error"
                  :aria-label="t('resourcePools.deletePool')"
                >
                  <template #icon><n-icon :component="TrashOutline" /></template>
                </n-button>
              </template>
              {{ t("resourcePools.deletePoolConfirm") }}
            </n-popconfirm>
          </div>
        </div>

        <dl class="pool-policy">
          <div>
            <dt>{{ t("resourcePools.affinityTTL") }}</dt>
            <dd>{{ formatTTL(pool.affinity_ttl_seconds) }}</dd>
          </div>
          <div>
            <dt>{{ t("resourcePools.busyWait") }}</dt>
            <dd>{{ pool.busy_wait_milliseconds }} ms</dd>
          </div>
          <div>
            <dt>{{ t("resourcePools.strategy") }}</dt>
            <dd>{{ t("resourcePools.roundRobin") }}</dd>
          </div>
          <div>
            <dt>{{ t("resourcePools.autoRestoreSchedule") }}</dt>
            <dd>{{ pool.auto_restore_schedule || t("resourcePools.autoRestoreDisabled") }}</dd>
          </div>
        </dl>

        <resource-manager
          v-if="expandedPoolIDs.has(pool.id)"
          :pool-id="pool.id"
          :refresh-token="managerRefreshTokens[pool.id] ?? 0"
          @resources-deleted="count => handleResourcesDeleted(pool, count)"
        />
        <n-empty
          v-else-if="pool.resource_count === 0"
          size="small"
          class="pool-empty"
          :description="t('resourcePools.noResources')"
        >
          <template #extra>
            <n-button size="small" @click="openResourceImporter(pool)">
              {{ t("resourcePools.addFirstResource") }}
            </n-button>
          </template>
        </n-empty>
      </section>
    </div>

    <n-modal v-model:show="poolModalVisible">
      <n-card
        class="editor-card"
        :bordered="false"
        :title="editingPoolID ? t('resourcePools.editPool') : t('resourcePools.createPool')"
        role="dialog"
        aria-modal="true"
      >
        <n-form ref="poolFormRef" :model="poolForm" :rules="poolRules" label-placement="top">
          <n-form-item :label="t('resourcePools.name')" path="name">
            <n-input
              v-model:value="poolForm.name"
              :placeholder="t('resourcePools.namePlaceholder')"
            />
          </n-form-item>
          <n-form-item :label="t('common.description')" path="description">
            <n-input v-model:value="poolForm.description" type="textarea" :rows="2" />
          </n-form-item>
          <div class="timing-fields">
            <n-form-item :label="t('resourcePools.affinityTTLSeconds')" path="affinity_ttl_seconds">
              <n-input-number
                v-model:value="poolForm.affinity_ttl_seconds"
                :min="60"
                :max="604800"
              />
            </n-form-item>
            <n-form-item
              :label="t('resourcePools.busyWaitMilliseconds')"
              path="busy_wait_milliseconds"
            >
              <n-input-number
                v-model:value="poolForm.busy_wait_milliseconds"
                :min="0"
                :max="10000"
              />
            </n-form-item>
          </div>
          <n-form-item :label="t('resourcePools.autoRestoreSchedule')" path="auto_restore_schedule">
            <n-input
              v-model:value="poolForm.auto_restore_schedule"
              :placeholder="t('resourcePools.autoRestoreSchedulePlaceholder')"
            />
            <template #feedback>{{ t("resourcePools.autoRestoreScheduleHint") }}</template>
          </n-form-item>
        </n-form>
        <template #footer>
          <div class="modal-actions">
            <n-button @click="poolModalVisible = false">{{ t("common.cancel") }}</n-button>
            <n-button type="primary" :loading="savingPool" @click="savePool">
              {{ t("common.save") }}
            </n-button>
          </div>
        </template>
      </n-card>
    </n-modal>

    <n-modal v-model:show="resourcesModalVisible">
      <n-card
        class="editor-card resource-importer"
        :bordered="false"
        :title="
          resourceImportMode === 'batch'
            ? t('resourcePools.addResources')
            : t('resourcePools.importConfig')
        "
      >
        <p class="import-help">
          {{
            resourceImportMode === "batch"
              ? t("resourcePools.importHelp")
              : t("resourcePools.importConfigHelp")
          }}
        </p>
        <n-form
          v-if="resourceImportMode === 'batch'"
          label-placement="top"
          class="resource-import-form"
        >
          <n-form-item :label="t('resourcePools.upstreamURL')" required>
            <n-input
              v-model:value="resourceUpstreamURL"
              :placeholder="t('resourcePools.upstreamPlaceholder')"
              spellcheck="false"
            />
          </n-form-item>
          <n-form-item :label="t('resourcePools.bulkKeys')" required>
            <div class="key-input-stack">
              <n-input
                v-model:value="resourceKeysText"
                type="textarea"
                :rows="9"
                :placeholder="t('resourcePools.keysPlaceholder')"
                spellcheck="false"
              />
              <div class="key-input-meta" aria-live="polite">
                <span>{{ t("resourcePools.keySeparatorHelp") }}</span>
                <strong v-if="uniqueResourceKeys.length">
                  {{ t("resourcePools.keysDetected", { count: uniqueResourceKeys.length }) }}
                </strong>
              </div>
              <small v-if="duplicateResourceKeyCount" class="duplicate-note">
                {{
                  t("resourcePools.duplicateKeysIgnored", {
                    count: duplicateResourceKeyCount,
                  })
                }}
              </small>
            </div>
          </n-form-item>
          <div class="timing-fields scheduling-fields">
            <n-form-item :label="t('resourcePools.priority')">
              <n-input-number v-model:value="resourcePriority" :min="1" :max="1000" />
            </n-form-item>
            <n-form-item :label="t('resourcePools.weight')">
              <n-input-number v-model:value="resourceWeight" :min="1" :max="1000" />
            </n-form-item>
          </div>
          <p class="scheduling-help">{{ t("resourcePools.schedulingHelp") }}</p>
        </n-form>
        <n-input
          v-else
          v-model:value="resourceImportContent"
          class="config-import-input"
          type="textarea"
          :rows="12"
          :placeholder="t('resourcePools.importConfigPlaceholder')"
          spellcheck="false"
        />
        <n-alert type="warning" :bordered="false" class="secret-note">
          {{ t("resourcePools.secretNote") }}
        </n-alert>
        <template #footer>
          <div class="modal-actions">
            <n-button @click="resourcesModalVisible = false">{{ t("common.cancel") }}</n-button>
            <n-button type="primary" :loading="savingResources" @click="addResources">
              {{
                resourceImportMode === "batch"
                  ? t("resourcePools.importResources")
                  : t("resourcePools.importConfigAction")
              }}
            </n-button>
          </div>
        </template>
      </n-card>
    </n-modal>
  </div>
</template>

<style scoped>
.resource-pools-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
  max-width: 1280px;
  margin: 0 auto;
}

.page-header,
.pool-header,
.header-actions,
.pool-actions,
.modal-actions {
  display: flex;
  align-items: center;
}

.page-header,
.pool-header {
  justify-content: space-between;
  gap: 20px;
}

.page-header h1 {
  margin: 0;
  color: var(--text-primary);
  font-size: 1.65rem;
  line-height: 1.25;
  text-wrap: balance;
}

.page-header p,
.pool-identity p,
.import-help {
  color: var(--text-secondary);
}

.page-header p {
  margin-top: 4px;
  max-width: 68ch;
}

.header-actions,
.pool-actions,
.modal-actions {
  gap: 8px;
}

.modal-actions {
  justify-content: flex-end;
}

.routing-note {
  background: var(--primary-color-suppl);
}

.loading-stack,
.pool-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.empty-state {
  padding: 72px 24px;
  background: var(--card-bg-solid);
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius-xl);
}

.pool-panel {
  background: var(--card-bg-solid);
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius-xl);
  overflow: hidden;
}

.pool-header {
  padding: 18px 20px 14px;
}

.pool-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pool-title-row h2 {
  margin: 0;
  font-size: 1.1rem;
  color: var(--text-primary);
}

.pool-identity p {
  margin: 3px 0 0;
  max-width: 70ch;
}

.muted {
  color: var(--text-tertiary) !important;
}

.pool-policy {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 28px;
  margin: 0;
  padding: 10px 20px;
  background: var(--bg-secondary);
  border-top: 1px solid var(--border-color-light);
  border-bottom: 1px solid var(--border-color-light);
}

.pool-policy div {
  display: flex;
  align-items: baseline;
  gap: 7px;
}

.pool-policy dt {
  color: var(--text-tertiary);
  font-size: 0.78rem;
}

.pool-policy dd {
  margin: 0;
  color: var(--text-primary);
  font-size: 0.84rem;
  font-weight: 600;
}

.pool-empty {
  padding: 28px 20px;
}

.editor-card {
  width: min(560px, calc(100vw - 28px));
}

.timing-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.timing-fields :deep(.n-input-number) {
  width: 100%;
}

.resource-import-form {
  margin-top: 14px;
}

.key-input-stack {
  display: flex;
  flex-direction: column;
  gap: 7px;
  width: 100%;
}

.key-input-meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: var(--text-secondary);
  font-size: 0.78rem;
}

.key-input-meta strong {
  flex-shrink: 0;
  color: var(--primary-color);
  font-weight: 600;
}

.duplicate-note {
  color: var(--warning-color);
  font-size: 0.76rem;
}

.secret-note {
  margin-top: 12px;
}

.scheduling-fields {
  margin-top: -4px;
}

.scheduling-help {
  margin: -8px 0 14px;
  color: var(--text-secondary);
  font-size: 0.78rem;
}

.config-import-input {
  margin-top: 14px;
}

@media (max-width: 720px) {
  .page-header,
  .pool-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .header-actions,
  .pool-actions {
    width: 100%;
    flex-wrap: wrap;
  }

  .timing-fields {
    grid-template-columns: 1fr;
    gap: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .resource-pools-page * {
    scroll-behavior: auto;
  }
}
</style>
