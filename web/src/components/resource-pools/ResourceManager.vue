<script setup lang="ts">
import { resourcePoolsApi } from "@/api/resourcePools";
import ResourceDiagnosticsDrawer from "@/components/resource-pools/ResourceDiagnosticsDrawer.vue";
import type {
  ResourceBalanceSnapshot,
  ResourcePoolEndpoint,
  ResourceStatus,
  ResourceValidationGroup,
  UpstreamResource,
} from "@/types/models";
import {
  CreateOutline,
  DownloadOutline,
  InformationCircleOutline,
  PulseOutline,
  RefreshOutline,
  SearchOutline,
  SettingsOutline,
  TrashOutline,
  WalletOutline,
} from "@vicons/ionicons5";
import {
  NAlert,
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

const props = defineProps<{ poolId: number; poolName: string; refreshToken?: number }>();
const emit = defineEmits<{ resourcesDeleted: [count: number] }>();
const { t } = useI18n();
const message = useMessage();

const resources = ref<UpstreamResource[]>([]);
const validationGroups = ref<ResourceValidationGroup[]>([]);
const endpoints = ref<ResourcePoolEndpoint[]>([]);
const loading = ref(false);
const validationGroupsLoading = ref(false);
const testingResourceID = ref<number | null>(null);
const batchTesting = ref(false);
const selectedValidationGroupID = ref<number | null>(null);
const batchRecoverySummary = ref<{
  requested: number;
  valid: number;
  invalid: number;
  errors: number;
} | null>(null);
const balanceResourceID = ref<number | null>(null);
const balanceVisible = ref(false);
const balanceSnapshot = ref<ResourceBalanceSnapshot | null>(null);
const balanceResourceLabel = ref("");
const mutating = ref(false);
const search = ref("");
const health = ref<ResourceStatus | "">("");
const availability = ref<"" | "enabled" | "disabled">("");
const page = ref(1);
const pageSize = ref(20);
const totalItems = ref(0);
const totalPages = ref(0);
const selectedIDs = ref<number[]>([]);
const diagnosticsVisible = ref(false);
const diagnosticResourceID = ref<number | null>(null);
const editVisible = ref(false);
const scheduleVisible = ref(false);
const deleteKeysVisible = ref(false);
const deleteKeysText = ref("");
const editingResource = ref<UpstreamResource | null>(null);
const editForm = reactive({
  name: "",
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
      { label: t("resourcePools.exportCoolingKeys"), key: "keys-cooling" },
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
const validationRouteSelectOptions = computed(() =>
  validationGroups.value.map(group => ({
    label: `${group.display_name || group.name} · ${group.channel_type}`,
    value: group.id,
  }))
);
const balanceEndpointOptions = computed(() =>
  endpoints.value
    .filter(endpoint => endpoint.enabled && balanceProvider(endpoint.base_url))
    .map(endpoint => ({
      label: `${endpoint.name} · ${balanceProvider(endpoint.base_url)}`,
      key: endpoint.id,
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
const diagnosticResource = computed(
  () => resources.value.find(resource => resource.id === diagnosticResourceID.value) ?? null
);

onMounted(() => {
  void loadResources();
  void loadValidationGroups();
  void loadEndpoints();
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
    if (!validationGroups.value.some(group => group.id === selectedValidationGroupID.value)) {
      selectedValidationGroupID.value = validationGroups.value[0]?.id ?? null;
    }
  } finally {
    validationGroupsLoading.value = false;
  }
}

async function loadEndpoints() {
  try {
    endpoints.value = await resourcePoolsApi.listEndpoints(props.poolId);
  } catch {
    endpoints.value = [];
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
    key: "",
    enabled: resource.enabled,
    priority: resource.priority,
    weight: resource.weight,
  });
  editVisible.value = true;
}
function openDiagnostics(resource: UpstreamResource) {
  diagnosticResourceID.value = resource.id;
  diagnosticsVisible.value = true;
}
async function saveResource() {
  if (!editingResource.value || mutating.value) {
    return;
  }
  mutating.value = true;
  try {
    await resourcePoolsApi.updateResource(props.poolId, editingResource.value.id, {
      name: editForm.name.trim(),
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
      enabled: !resource.enabled,
    });
    await loadResources();
  } finally {
    mutating.value = false;
  }
}
async function forceRestoreHealth(resource: UpstreamResource) {
  mutating.value = true;
  try {
    await resourcePoolsApi.updateResourceStatus(props.poolId, resource.id, "active");
    message.success(t("resourcePools.forceRecoverCompleted", { count: 1 }));
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
async function validateAndRecoverSelected() {
  const groupID = selectedValidationGroupID.value;
  const resourceIDs = [...selectedIDs.value];
  if (!groupID || resourceIDs.length === 0 || batchTesting.value || mutating.value) {
    return;
  }
  const validationGroupID = groupID;

  batchTesting.value = true;
  batchRecoverySummary.value = null;
  const summary = { requested: resourceIDs.length, valid: 0, invalid: 0, errors: 0 };
  let cursor = 0;
  const workerCount = Math.min(4, resourceIDs.length);

  async function validateNext() {
    while (cursor < resourceIDs.length) {
      const resourceID = resourceIDs[cursor++];
      try {
        const result = await resourcePoolsApi.testResource(
          props.poolId,
          resourceID,
          validationGroupID
        );
        if (result.is_valid) {
          summary.valid++;
        } else {
          summary.invalid++;
        }
      } catch {
        summary.errors++;
      }
    }
  }

  try {
    await Promise.all(Array.from({ length: workerCount }, () => validateNext()));
    batchRecoverySummary.value = summary;
    await loadResources();
  } finally {
    batchTesting.value = false;
  }
}
async function forceRestoreSelected() {
  const resourceIDs = [...selectedIDs.value];
  if (resourceIDs.length === 0 || batchTesting.value || mutating.value) {
    return;
  }
  mutating.value = true;
  try {
    const result = await resourcePoolsApi.bulkUpdateResourceStatus(
      props.poolId,
      resourceIDs,
      "active"
    );
    message.success(t("resourcePools.forceRecoverCompleted", { count: result.updated_count }));
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
    resourcePoolsApi.exportResources(
      props.poolId,
      {
        content: "full",
        format: value === "full-jsonl" ? "jsonl" : "csv",
      },
      props.poolName
    );
    return;
  }
  if (value.startsWith("keys-")) {
    const status = value.slice(5) as "all" | "active" | "cooling" | "invalid" | "disabled";
    resourcePoolsApi.exportResources(
      props.poolId,
      { content: "keys", format: "txt", status },
      props.poolName
    );
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
function balanceProvider(baseURL: string): string {
  try {
    const host = new URL(baseURL).hostname.toLowerCase();
    if (host === "api.deepseek.com") {
      return "DeepSeek";
    }
    if (host === "openrouter.ai") {
      return "OpenRouter";
    }
    if (host === "api.siliconflow.com" || host === "api.siliconflow.cn") {
      return "SiliconFlow";
    }
    if (host === "api.moonshot.cn" || host === "api.moonshot.ai") {
      return "Moonshot";
    }
  } catch {
    return "";
  }
  return "";
}
async function inspectBalance(resource: UpstreamResource, endpointID?: number) {
  const selectedEndpointID = endpointID ?? balanceEndpointOptions.value[0]?.key;
  if (!selectedEndpointID || balanceResourceID.value !== null) {
    return;
  }
  balanceResourceID.value = resource.id;
  balanceSnapshot.value = null;
  balanceResourceLabel.value = resource.name || resource.masked_key || `#${resource.id}`;
  try {
    balanceSnapshot.value = await resourcePoolsApi.inspectResourceBalance(
      props.poolId,
      selectedEndpointID,
      resource.id
    );
    balanceVisible.value = true;
  } catch {
    message.error(t("resourcePools.balanceQueryFailed"));
  } finally {
    balanceResourceID.value = null;
  }
}
function balanceKindLabel(kind: string): string {
  const keyByKind: Record<string, string> = {
    total: "balanceTotal",
    granted: "balanceGranted",
    topped_up: "balanceToppedUp",
    limit_remaining: "balanceRemaining",
    limit: "balanceLimit",
    balance: "balanceAvailable",
    charge: "balanceCharge",
    available: "balanceAvailable",
    voucher: "balanceVoucher",
    cash: "balanceCash",
    usage: "balanceUsage",
    usage_daily: "balanceUsageDaily",
  };
  const key = keyByKind[kind];
  return key ? t(`resourcePools.${key}`) : kind;
}
function isResourceTestDisabled(): boolean {
  return (
    mutating.value ||
    batchTesting.value ||
    validationGroupsLoading.value ||
    validationGroups.value.length === 0 ||
    testingResourceID.value !== null
  );
}
function healthType(value: ResourceStatus): "success" | "error" {
  return value === "active" ? "success" : "error";
}
function failureReasonPreview(value: string): string {
  const reason = value.trim();
  if (/failed to send validation request|connection refused|connectex/i.test(reason)) {
    return t("resourcePools.upstreamConnectionFailed");
  }
  return reason.length > 84 ? `${reason.slice(0, 81)}…` : reason;
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

    <div class="state-guide" role="note">
      <div>
        <strong>{{ t("resourcePools.operatorEnablement") }}</strong>
        <span>{{ t("resourcePools.operatorEnablementHelp") }}</span>
      </div>
      <div>
        <strong>{{ t("resourcePools.runtimeHealth") }}</strong>
        <span>{{ t("resourcePools.runtimeHealthHelp") }}</span>
      </div>
    </div>

    <n-alert
      v-if="batchRecoverySummary"
      class="recovery-summary"
      :type="
        batchRecoverySummary.invalid === 0 && batchRecoverySummary.errors === 0
          ? 'success'
          : 'warning'
      "
      :title="t('resourcePools.batchRecoveryComplete')"
      closable
      @close="batchRecoverySummary = null"
    >
      {{
        t("resourcePools.batchRecoverySummary", {
          requested: batchRecoverySummary.requested,
          valid: batchRecoverySummary.valid,
          invalid: batchRecoverySummary.invalid,
          errors: batchRecoverySummary.errors,
        })
      }}
    </n-alert>

    <div v-if="selectedIDs.length" class="selection-bar" aria-live="polite">
      <strong>{{ t("resourcePools.selectedCount", { count: selectedIDs.length }) }}</strong>
      <div class="selection-actions">
        <n-select
          v-if="validationGroups.length > 1"
          v-model:value="selectedValidationGroupID"
          class="validation-route-select"
          size="small"
          :options="validationRouteSelectOptions"
          :placeholder="t('resourcePools.validationRoute')"
        />
        <n-button
          size="small"
          type="primary"
          :disabled="mutating || validationGroupsLoading || validationGroups.length === 0"
          :loading="batchTesting"
          @click="validateAndRecoverSelected"
        >
          <template #icon><n-icon :component="PulseOutline" /></template>
          {{ t("resourcePools.bulkValidateAndRecover") }}
        </n-button>
        <n-button
          size="small"
          :disabled="mutating || batchTesting"
          @click="updateSelectedEnabled(true)"
        >
          {{ t("resourcePools.bulkEnable") }}
        </n-button>
        <n-button
          size="small"
          :disabled="mutating || batchTesting"
          @click="updateSelectedEnabled(false)"
        >
          {{ t("resourcePools.bulkDisable") }}
        </n-button>
        <n-button size="small" :disabled="mutating || batchTesting" @click="openScheduleEditor">
          <template #icon><n-icon :component="SettingsOutline" /></template>
          {{ t("resourcePools.setScheduling") }}
        </n-button>
        <n-popconfirm @positive-click="forceRestoreSelected">
          <template #trigger>
            <n-button size="small" type="warning" :disabled="mutating || batchTesting">
              {{ t("resourcePools.forceRecover") }}
            </n-button>
          </template>
          {{ t("resourcePools.forceRecoverSelectedConfirm", { count: selectedIDs.length }) }}
        </n-popconfirm>
        <n-popconfirm @positive-click="deleteSelected">
          <template #trigger>
            <n-button size="small" type="error" :disabled="mutating || batchTesting">
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
              <small v-else-if="resource.disabled_reason" :title="resource.disabled_reason">
                {{ failureReasonPreview(resource.disabled_reason) }}
              </small>
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
                :aria-label="t('resourcePools.openDiagnostics')"
                :title="t('resourcePools.openDiagnostics')"
                :disabled="batchTesting"
                @click="openDiagnostics(resource)"
              >
                <template #icon><n-icon :component="InformationCircleOutline" /></template>
              </n-button>
              <n-button
                size="tiny"
                quaternary
                :aria-label="t('common.edit')"
                :disabled="batchTesting"
                @click="openEditor(resource)"
              >
                <template #icon><n-icon :component="CreateOutline" /></template>
              </n-button>
              <n-button
                size="tiny"
                secondary
                :disabled="mutating || batchTesting"
                @click="toggleEnabled(resource)"
              >
                {{
                  resource.enabled ? t("resourcePools.pauseScheduling") : t("resourcePools.enable")
                }}
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
                  {{
                    resource.status === "invalid"
                      ? t("resourcePools.validateAndRecover")
                      : t("resourcePools.testKey")
                  }}
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
                {{
                  resource.status === "invalid"
                    ? t("resourcePools.validateAndRecover")
                    : t("resourcePools.testKey")
                }}
              </n-button>
              <n-dropdown
                v-if="balanceEndpointOptions.length > 1"
                :options="balanceEndpointOptions"
                trigger="click"
                @select="endpointId => inspectBalance(resource, Number(endpointId))"
              >
                <n-button
                  size="tiny"
                  secondary
                  :disabled="balanceResourceID !== null"
                  :loading="balanceResourceID === resource.id"
                >
                  <template #icon><n-icon :component="WalletOutline" /></template>
                  {{ t("resourcePools.queryBalance") }}
                </n-button>
              </n-dropdown>
              <n-button
                v-else-if="balanceEndpointOptions.length === 1"
                size="tiny"
                secondary
                :disabled="balanceResourceID !== null"
                :loading="balanceResourceID === resource.id"
                @click="inspectBalance(resource)"
              >
                <template #icon><n-icon :component="WalletOutline" /></template>
                {{ t("resourcePools.queryBalance") }}
              </n-button>
              <n-popconfirm
                v-if="resource.status === 'invalid'"
                @positive-click="forceRestoreHealth(resource)"
              >
                <template #trigger>
                  <n-button
                    size="tiny"
                    quaternary
                    type="warning"
                    :disabled="mutating || batchTesting"
                  >
                    {{ t("resourcePools.forceRecover") }}
                  </n-button>
                </template>
                {{ t("resourcePools.forceRecoverConfirm") }}
              </n-popconfirm>
              <n-popconfirm @positive-click="deleteOne(resource.id)">
                <template #trigger>
                  <n-button
                    size="tiny"
                    quaternary
                    type="error"
                    :aria-label="t('common.delete')"
                    :disabled="batchTesting"
                  >
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

    <n-modal v-model:show="balanceVisible">
      <n-card
        class="manager-modal balance-modal"
        :bordered="false"
        :title="t('resourcePools.balanceTitle')"
      >
        <div v-if="balanceSnapshot" class="balance-content">
          <div class="balance-summary">
            <div>
              <span class="balance-provider">{{ balanceSnapshot.provider }}</span>
              <strong>{{ balanceResourceLabel }}</strong>
            </div>
            <n-tag
              v-if="balanceSnapshot.available !== undefined"
              :type="balanceSnapshot.available ? 'success' : 'error'"
            >
              {{
                balanceSnapshot.available
                  ? t("resourcePools.balanceUsable")
                  : t("resourcePools.balanceUnavailable")
              }}
            </n-tag>
          </div>
          <div class="balance-grid">
            <div
              v-for="(item, index) in balanceSnapshot.balances"
              :key="`${item.kind}-${item.currency}-${index}`"
              class="balance-item"
            >
              <span>{{ balanceKindLabel(item.kind) }}</span>
              <strong>
                {{ item.amount }}
                <small>{{ item.currency }}</small>
              </strong>
            </div>
            <div
              v-for="item in balanceSnapshot.metrics || []"
              :key="item.kind"
              class="balance-item"
            >
              <span>{{ balanceKindLabel(item.kind) }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>
          <small class="balance-checked-at">
            {{
              t("resourcePools.balanceCheckedAt", { value: formatDate(balanceSnapshot.checked_at) })
            }}
          </small>
        </div>
        <template #footer>
          <div class="modal-actions">
            <n-button @click="balanceVisible = false">{{ t("common.close") }}</n-button>
          </div>
        </template>
      </n-card>
    </n-modal>

    <resource-diagnostics-drawer
      v-model:show="diagnosticsVisible"
      :resource="diagnosticResource"
      :validation-groups="validationGroups"
      :validating="testingResourceID === diagnosticResource?.id"
      :mutating="mutating"
      @validate="testResource"
      @force-recover="forceRestoreHealth"
      @toggle-scheduling="toggleEnabled"
    />
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
.state-guide {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0;
  border-bottom: 1px solid var(--border-color-light);
  background: var(--bg-secondary);
}
.state-guide > div {
  display: grid;
  gap: 2px;
  padding: 10px 20px;
}
.state-guide > div + div {
  border-left: 1px solid var(--border-color-light);
}
.state-guide strong {
  color: var(--text-primary);
  font-size: 0.8rem;
}
.state-guide span {
  color: var(--text-secondary);
  font-size: 0.75rem;
}
.recovery-summary {
  margin: 12px 20px 0;
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
.selection-actions {
  flex-wrap: wrap;
}
.validation-route-select {
  width: 210px;
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
    28px minmax(160px, 1fr) 105px minmax(170px, 1fr)
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
.resource-row time,
.status-stack small,
.compact-data {
  color: var(--text-secondary);
  font-size: 0.78rem;
}
.resource-name code {
  font-family: var(--font-mono);
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
.balance-modal {
  width: min(560px, calc(100vw - 28px));
}
.balance-content {
  display: grid;
  gap: 18px;
}
.balance-summary {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.balance-summary > div {
  display: grid;
  gap: 3px;
  min-width: 0;
}
.balance-provider,
.balance-checked-at,
.balance-item span {
  color: var(--text-secondary);
}
.balance-provider {
  text-transform: capitalize;
  font-size: 0.78rem;
}
.balance-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.balance-item {
  display: grid;
  gap: 4px;
  padding: 12px 14px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color-light);
  border-radius: 8px;
}
.balance-item span,
.balance-item small,
.balance-checked-at {
  font-size: 0.75rem;
}
.balance-item strong {
  color: var(--text-primary);
  font-size: 1rem;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
.balance-item small {
  color: var(--text-secondary);
  font-weight: 500;
}
.balance-checked-at {
  text-align: right;
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
  .state-guide {
    grid-template-columns: 1fr;
  }
  .state-guide > div + div {
    border-top: 1px solid var(--border-color-light);
    border-left: 0;
  }
  .selection-actions,
  .validation-route-select {
    width: 100%;
  }
  .form-grid,
  .form-grid.two-equal {
    grid-template-columns: 1fr;
    gap: 0;
  }
  .balance-grid {
    grid-template-columns: 1fr;
  }
}
</style>
