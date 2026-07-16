<script setup lang="ts">
import { resourcePoolsApi } from "@/api/resourcePools";
import type {
  ResourcePool,
  ResourcePoolInput,
  ResourceStatus,
  UpstreamResourceInput,
} from "@/types/models";
import {
  AddOutline,
  CreateOutline,
  PauseOutline,
  PlayOutline,
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
import { reactive, ref } from "vue";
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
const poolFormRef = ref();
const resourceText = ref("");

const poolForm = reactive<ResourcePoolInput>({
  name: "",
  description: "",
  strategy: "round_robin",
  affinity_ttl_seconds: 3600,
  busy_wait_milliseconds: 2000,
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

function openResourceImporter(pool: ResourcePool) {
  targetPool.value = pool;
  resourceText.value = "";
  resourcesModalVisible.value = true;
}

function parseResourceLines(): UpstreamResourceInput[] | null {
  const resources: UpstreamResourceInput[] = [];
  const lines = resourceText.value.split(/\r?\n/);
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index].trim();
    if (!line) {
      continue;
    }
    const parts = line.split("|").map(part => part.trim());
    let name = "";
    let upstreamUrl = "";
    let key = "";
    if (parts.length === 2) {
      [upstreamUrl, key] = parts;
    } else if (parts.length >= 3) {
      name = parts[0];
      upstreamUrl = parts[1];
      key = parts.slice(2).join("|").trim();
    } else {
      message.error(t("resourcePools.invalidLine", { line: index + 1 }));
      return null;
    }
    try {
      const parsed = new URL(upstreamUrl);
      if (!/^https?:$/.test(parsed.protocol) || !key) {
        throw new Error("invalid resource");
      }
    } catch {
      message.error(t("resourcePools.invalidLine", { line: index + 1 }));
      return null;
    }
    resources.push({ name, upstream_url: upstreamUrl, key });
  }
  if (resources.length === 0) {
    message.warning(t("resourcePools.resourcesRequired"));
    return null;
  }
  return resources;
}

async function addResources() {
  if (!targetPool.value || savingResources.value) {
    return;
  }
  const resources = parseResourceLines();
  if (!resources) {
    return;
  }
  savingResources.value = true;
  try {
    await resourcePoolsApi.addResources(targetPool.value.id, resources);
    resourcesModalVisible.value = false;
    resourceText.value = "";
    await loadPools();
  } finally {
    savingResources.value = false;
  }
}

async function setResourceStatus(
  pool: ResourcePool,
  resourceId: number,
  status: "active" | "disabled"
) {
  await resourcePoolsApi.updateResourceStatus(pool.id, resourceId, status);
  await loadPools();
}

async function deleteResource(pool: ResourcePool, resourceId: number) {
  await resourcePoolsApi.deleteResource(pool.id, resourceId);
  await loadPools();
}

async function deletePool(pool: ResourcePool) {
  await resourcePoolsApi.deletePool(pool.id);
  await loadPools();
}

function statusType(status: ResourceStatus): "success" | "warning" | "error" | "default" {
  if (status === "active") {
    return "success";
  }
  if (status === "invalid") {
    return "error";
  }
  if (status === "disabled") {
    return "warning";
  }
  return "default";
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
            <n-button size="small" @click="openResourceImporter(pool)">
              <template #icon><n-icon :component="AddOutline" /></template>
              {{ t("resourcePools.addResources") }}
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
        </dl>

        <div v-if="pool.resources?.length" class="resource-table" role="table">
          <div class="resource-row resource-heading" role="row">
            <span>{{ t("resourcePools.resource") }}</span>
            <span>{{ t("resourcePools.upstream") }}</span>
            <span>{{ t("resourcePools.status") }}</span>
            <span>{{ t("resourcePools.lastUsed") }}</span>
            <span class="actions-heading">{{ t("common.actions") }}</span>
          </div>
          <div
            v-for="resource in pool.resources"
            :key="resource.id"
            class="resource-row"
            role="row"
          >
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
                v-if="resource.status === 'active'"
                size="tiny"
                secondary
                @click="setResourceStatus(pool, resource.id, 'disabled')"
              >
                <template #icon><n-icon :component="PauseOutline" /></template>
                {{ t("resourcePools.disable") }}
              </n-button>
              <n-button
                v-else
                size="tiny"
                secondary
                type="success"
                @click="setResourceStatus(pool, resource.id, 'active')"
              >
                <template #icon><n-icon :component="PlayOutline" /></template>
                {{ t("resourcePools.restore") }}
              </n-button>
              <n-popconfirm @positive-click="deleteResource(pool, resource.id)">
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
        <n-empty
          v-else
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
        :title="t('resourcePools.addResources')"
      >
        <p class="import-help">{{ t("resourcePools.importHelp") }}</p>
        <pre>{{ t("resourcePools.importExample") }}</pre>
        <n-input
          v-model:value="resourceText"
          type="textarea"
          :rows="9"
          :placeholder="t('resourcePools.importPlaceholder')"
          spellcheck="false"
        />
        <n-alert type="warning" :bordered="false" class="secret-note">
          {{ t("resourcePools.secretNote") }}
        </n-alert>
        <template #footer>
          <div class="modal-actions">
            <n-button @click="resourcesModalVisible = false">{{ t("common.cancel") }}</n-button>
            <n-button type="primary" :loading="savingResources" @click="addResources">
              {{ t("resourcePools.importResources") }}
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

.resource-table {
  overflow-x: auto;
}

.resource-row {
  display: grid;
  grid-template-columns:
    minmax(150px, 1fr) minmax(220px, 1.6fr) minmax(160px, 1fr) minmax(150px, 0.8fr)
    minmax(170px, auto);
  gap: 16px;
  align-items: center;
  min-width: 880px;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border-color-light);
}

.resource-row:last-child {
  border-bottom: 0;
}

.resource-heading {
  color: var(--text-tertiary);
  background: var(--bg-primary);
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
  gap: 5px;
}

.actions-heading {
  text-align: right;
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

.resource-importer pre {
  margin: 8px 0 12px;
  padding: 10px 12px;
  overflow-x: auto;
  color: var(--text-primary);
  background: var(--code-bg);
  border-radius: var(--border-radius-md);
  font-family: var(--font-mono);
  font-size: 0.78rem;
}

.secret-note {
  margin-top: 12px;
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
