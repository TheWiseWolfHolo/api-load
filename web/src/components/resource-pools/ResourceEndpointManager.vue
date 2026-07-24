<script setup lang="ts">
import { resourcePoolsApi } from "@/api/resourcePools";
import { settingsApi } from "@/api/settings";
import type { ChannelType, ResourcePoolEndpoint, ResourcePoolEndpointInput } from "@/types/models";
import { AddOutline, CreateOutline, RefreshOutline, TrashOutline } from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NEmpty,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpin,
  NSwitch,
  NTag,
  useMessage,
} from "naive-ui";
import { computed, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{ poolId: number }>();
const emit = defineEmits<{ countChanged: [count: number] }>();
const { t } = useI18n();
const message = useMessage();

const endpoints = ref<ResourcePoolEndpoint[]>([]);
const channelTypes = ref<string[]>([]);
const loading = ref(false);
const saving = ref(false);
const modalVisible = ref(false);
const editingID = ref<number | null>(null);
const form = reactive({
  name: "",
  channel_type: "openai",
  base_url: "",
  enabled: true,
});
const channelOptions = computed(() => {
  const options: Array<{ label: string; value: string; disabled?: boolean }> =
    channelTypes.value.map(channelType => ({ label: channelType, value: channelType }));
  if (form.channel_type && !channelTypes.value.includes(form.channel_type)) {
    options.unshift({
      label: `${form.channel_type} · ${t("resourcePools.legacyEndpointNeedsChannel")}`,
      value: form.channel_type,
      disabled: true,
    });
  }
  return options;
});

onMounted(async () => {
  await Promise.all([loadEndpoints(), loadChannelTypes()]);
});

async function loadEndpoints() {
  loading.value = true;
  try {
    endpoints.value = await resourcePoolsApi.listEndpoints(props.poolId);
    emit("countChanged", endpoints.value.length);
  } finally {
    loading.value = false;
  }
}

async function loadChannelTypes() {
  channelTypes.value = await settingsApi.getChannelTypes();
}

function openEditor(endpoint?: ResourcePoolEndpoint) {
  editingID.value = endpoint?.id ?? null;
  Object.assign(form, {
    name: endpoint?.name ?? "",
    channel_type: endpoint?.channel_type ?? "openai",
    base_url: endpoint?.base_url ?? "",
    enabled: endpoint?.enabled ?? true,
  });
  modalVisible.value = true;
}

async function saveEndpoint() {
  const name = form.name.trim();
  const baseURL = form.base_url.trim().replace(/\/+$/, "");
  if (!name || !baseURL || saving.value) {
    message.warning(t("resourcePools.endpointRequired"));
    return;
  }
  try {
    const parsed = new URL(baseURL);
    if (!["http:", "https:"].includes(parsed.protocol)) {
      throw new Error("unsupported scheme");
    }
  } catch {
    message.error(t("resourcePools.invalidUpstream"));
    return;
  }
  saving.value = true;
  try {
    const payload: ResourcePoolEndpointInput = {
      name,
      channel_type: form.channel_type as ChannelType,
      base_url: baseURL,
      enabled: form.enabled,
    };
    if (editingID.value) {
      await resourcePoolsApi.updateEndpoint(props.poolId, editingID.value, payload);
    } else {
      await resourcePoolsApi.createEndpoint(props.poolId, payload);
    }
    modalVisible.value = false;
    await loadEndpoints();
  } finally {
    saving.value = false;
  }
}

async function toggleEndpoint(endpoint: ResourcePoolEndpoint) {
  await resourcePoolsApi.updateEndpoint(props.poolId, endpoint.id, {
    enabled: !endpoint.enabled,
  });
  await loadEndpoints();
}

async function deleteEndpoint(endpoint: ResourcePoolEndpoint) {
  await resourcePoolsApi.deleteEndpoint(props.poolId, endpoint.id);
  await loadEndpoints();
}
</script>

<template>
  <section class="endpoint-manager">
    <header class="endpoint-header">
      <div>
        <h3>{{ t("resourcePools.endpoints") }}</h3>
        <p>{{ t("resourcePools.endpointsHelp") }}</p>
      </div>
      <div class="endpoint-actions">
        <n-button quaternary :loading="loading" @click="loadEndpoints">
          <template #icon><n-icon :component="RefreshOutline" /></template>
        </n-button>
        <n-button secondary type="primary" @click="openEditor()">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t("resourcePools.addEndpoint") }}
        </n-button>
      </div>
    </header>

    <n-spin :show="loading">
      <div v-if="endpoints.length" class="endpoint-list">
        <article v-for="endpoint in endpoints" :key="endpoint.id" class="endpoint-row">
          <div class="endpoint-identity">
            <div class="endpoint-title">
              <strong>{{ endpoint.name }}</strong>
              <n-tag size="small" :bordered="false">{{ endpoint.channel_type }}</n-tag>
              <n-tag v-if="!endpoint.enabled" size="small" type="warning">
                {{ t("common.disabled") }}
              </n-tag>
            </div>
            <code :title="endpoint.base_url">{{ endpoint.base_url }}</code>
          </div>
          <div class="endpoint-row-actions">
            <n-button size="tiny" secondary @click="toggleEndpoint(endpoint)">
              {{ endpoint.enabled ? t("common.disable") : t("resourcePools.enable") }}
            </n-button>
            <n-button size="tiny" quaternary @click="openEditor(endpoint)">
              <template #icon><n-icon :component="CreateOutline" /></template>
            </n-button>
            <n-popconfirm @positive-click="deleteEndpoint(endpoint)">
              <template #trigger>
                <n-button size="tiny" quaternary type="error">
                  <template #icon><n-icon :component="TrashOutline" /></template>
                </n-button>
              </template>
              {{ t("resourcePools.deleteEndpointConfirm") }}
            </n-popconfirm>
          </div>
        </article>
      </div>
      <n-empty
        v-else
        size="small"
        class="endpoint-empty"
        :description="t('resourcePools.noEndpoints')"
      >
        <template #extra>
          <n-button size="small" type="primary" @click="openEditor()">
            {{ t("resourcePools.addFirstEndpoint") }}
          </n-button>
        </template>
      </n-empty>
    </n-spin>

    <n-modal v-model:show="modalVisible">
      <n-card
        class="endpoint-modal"
        :bordered="false"
        :title="editingID ? t('resourcePools.editEndpoint') : t('resourcePools.addEndpoint')"
      >
        <n-form label-placement="top">
          <n-form-item :label="t('resourcePools.endpointName')" required>
            <n-input
              v-model:value="form.name"
              :placeholder="t('resourcePools.endpointNamePlaceholder')"
            />
          </n-form-item>
          <n-form-item :label="t('resourcePools.endpointChannel')" required>
            <n-select v-model:value="form.channel_type" :options="channelOptions" filterable />
          </n-form-item>
          <n-form-item :label="t('resourcePools.endpointBaseURL')" required>
            <n-input
              v-model:value="form.base_url"
              :placeholder="t('resourcePools.endpointBaseURLPlaceholder')"
              spellcheck="false"
            />
            <template #feedback>{{ t("resourcePools.endpointBaseURLHelp") }}</template>
          </n-form-item>
          <n-form-item :label="t('resourcePools.enabledState')">
            <n-switch v-model:value="form.enabled" />
          </n-form-item>
        </n-form>
        <template #footer>
          <div class="modal-actions">
            <n-button @click="modalVisible = false">{{ t("common.cancel") }}</n-button>
            <n-button type="primary" :loading="saving" @click="saveEndpoint">
              {{ t("common.save") }}
            </n-button>
          </div>
        </template>
      </n-card>
    </n-modal>
  </section>
</template>

<style scoped>
.endpoint-manager {
  padding: 18px 20px;
  border-top: 1px solid var(--border-color-light);
}
.endpoint-header,
.endpoint-title,
.endpoint-actions,
.endpoint-row,
.endpoint-row-actions,
.modal-actions {
  display: flex;
  align-items: center;
}
.endpoint-header {
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}
.endpoint-header h3 {
  margin: 0 0 3px;
  color: var(--text-primary);
  font-size: 0.95rem;
}
.endpoint-header p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 0.78rem;
}
.endpoint-actions,
.endpoint-row-actions,
.endpoint-title,
.modal-actions {
  gap: 8px;
}
.endpoint-list {
  overflow: hidden;
  border: 1px solid var(--border-color-light);
  border-radius: 10px;
}
.endpoint-row {
  justify-content: space-between;
  gap: 18px;
  padding: 11px 14px;
}
.endpoint-row + .endpoint-row {
  border-top: 1px solid var(--border-color-light);
}
.endpoint-identity {
  min-width: 0;
}
.endpoint-identity code {
  display: block;
  overflow: hidden;
  margin-top: 4px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.78rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.endpoint-row-actions {
  flex-shrink: 0;
}
.endpoint-empty {
  padding: 18px;
}
.endpoint-modal {
  width: min(560px, calc(100vw - 28px));
}
.modal-actions {
  justify-content: flex-end;
}
@media (max-width: 720px) {
  .endpoint-header,
  .endpoint-row {
    align-items: flex-start;
    flex-direction: column;
  }
  .endpoint-actions,
  .endpoint-row-actions {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
