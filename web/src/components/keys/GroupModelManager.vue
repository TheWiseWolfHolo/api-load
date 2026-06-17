<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group } from "@/types/models";
import { Add, Refresh, Save, Search } from "@vicons/ionicons5";
import {
  NButton,
  NCheckbox,
  NEmpty,
  NIcon,
  NInput,
  NInputGroup,
  NSpace,
  NSpin,
  NTag,
  useMessage,
} from "naive-ui";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{
  group: Group | null;
}>();

const emit = defineEmits<{
  (e: "saved", models: string[]): void;
}>();

const { t } = useI18n();
const message = useMessage();
const loading = ref(false);
const discovering = ref(false);
const saving = ref(false);
const savedModels = ref<string[]>([]);
const discoveredModels = ref<string[]>([]);
const selectedModels = ref<Set<string>>(new Set());
const searchText = ref("");
const manualModel = ref("");

const availableModels = computed(() => {
  const seen = new Set<string>();
  for (const model of [...savedModels.value, ...discoveredModels.value]) {
    const trimmed = model.trim();
    if (trimmed) {
      seen.add(trimmed);
    }
  }
  return [...seen].sort((a, b) => a.localeCompare(b));
});

const filteredModels = computed(() => {
  const needle = searchText.value.trim().toLowerCase();
  if (!needle) {
    return availableModels.value;
  }
  return availableModels.value.filter(model => model.toLowerCase().includes(needle));
});

const selectedCount = computed(() => selectedModels.value.size);

watch(
  () => props.group?.id,
  () => {
    loadModels();
  },
  { immediate: true }
);

async function loadModels() {
  if (!props.group?.id || props.group.group_type === "aggregate") {
    savedModels.value = [];
    discoveredModels.value = [];
    selectedModels.value = new Set();
    return;
  }
  loading.value = true;
  try {
    const models = await keysApi.getGroupModels(props.group.id);
    savedModels.value = models;
    discoveredModels.value = [];
    selectedModels.value = new Set(models);
  } finally {
    loading.value = false;
  }
}

async function discoverModels() {
  if (!props.group?.id) {
    return;
  }
  discovering.value = true;
  try {
    const models = await keysApi.discoverGroupModels(props.group.id);
    discoveredModels.value = models;
    selectedModels.value = new Set([...selectedModels.value, ...models]);
    message.success(t("keys.modelDiscoverySuccess", { count: models.length }));
  } catch {
    message.error(t("keys.modelDiscoveryFailed"));
  } finally {
    discovering.value = false;
  }
}

function addManualModel() {
  const model = manualModel.value.trim();
  if (!model) {
    return;
  }
  selectedModels.value = new Set([...selectedModels.value, model]);
  discoveredModels.value = [...new Set([...discoveredModels.value, model])];
  manualModel.value = "";
}

function toggleModel(model: string, checked: boolean) {
  const next = new Set(selectedModels.value);
  if (checked) {
    next.add(model);
  } else {
    next.delete(model);
  }
  selectedModels.value = next;
}

function selectAllVisible() {
  selectedModels.value = new Set([...selectedModels.value, ...filteredModels.value]);
}

function invertVisible() {
  const next = new Set(selectedModels.value);
  for (const model of filteredModels.value) {
    if (next.has(model)) {
      next.delete(model);
    } else {
      next.add(model);
    }
  }
  selectedModels.value = next;
}

async function saveModels() {
  if (!props.group?.id) {
    return;
  }
  saving.value = true;
  try {
    const models = [...selectedModels.value].sort((a, b) => a.localeCompare(b));
    const saved = await keysApi.saveGroupModels(props.group.id, models);
    savedModels.value = saved;
    selectedModels.value = new Set(saved);
    emit("saved", saved);
    message.success(t("keys.modelSaveSuccess", { count: saved.length }));
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div class="model-manager">
    <div class="model-toolbar">
      <n-input
        v-model:value="searchText"
        clearable
        size="small"
        :placeholder="t('keys.searchModels')"
      >
        <template #prefix>
          <n-icon :component="Search" />
        </template>
      </n-input>
      <n-button size="small" secondary @click="discoverModels" :loading="discovering">
        <template #icon>
          <n-icon :component="Refresh" />
        </template>
        {{ t("keys.discoverModels") }}
      </n-button>
      <n-button size="small" type="primary" @click="saveModels" :loading="saving">
        <template #icon>
          <n-icon :component="Save" />
        </template>
        {{ t("common.save") }}
      </n-button>
    </div>

    <n-input-group class="manual-model-row">
      <n-input
        v-model:value="manualModel"
        size="small"
        :placeholder="t('keys.manualModelPlaceholder')"
        @keyup.enter="addManualModel"
      />
      <n-button size="small" @click="addManualModel">
        <template #icon>
          <n-icon :component="Add" />
        </template>
      </n-button>
    </n-input-group>

    <div class="model-actions">
      <n-space size="small">
        <n-button size="tiny" tertiary @click="selectAllVisible">
          {{ t("keys.selectAll") }}
        </n-button>
        <n-button size="tiny" tertiary @click="invertVisible">
          {{ t("keys.invertSelection") }}
        </n-button>
        <n-tag size="small" type="info">
          {{ t("keys.savedModelCount", { count: selectedCount }) }}
        </n-tag>
      </n-space>
    </div>

    <n-spin :show="loading">
      <div v-if="filteredModels.length" class="model-grid">
        <label v-for="model in filteredModels" :key="model" class="model-option">
          <n-checkbox
            :checked="selectedModels.has(model)"
            @update:checked="checked => toggleModel(model, Boolean(checked))"
          />
          <span>{{ model }}</span>
        </label>
      </div>
      <n-empty v-else size="small" :description="t('keys.noModels')" />
    </n-spin>
  </div>
</template>

<style scoped>
.model-manager {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.model-toolbar,
.manual-model-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 8px;
}

.model-actions {
  display: flex;
  justify-content: flex-end;
}

.model-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 8px;
}

.model-option {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding: 4px 8px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  color: var(--text-primary);
  font-family: monospace;
  word-break: break-all;
}

@media (max-width: 768px) {
  .model-toolbar {
    grid-template-columns: 1fr;
  }
}
</style>
