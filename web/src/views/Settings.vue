<script setup lang="ts">
import { settingsApi, type Setting, type SettingCategory } from "@/api/settings";
import ProxyKeysInput from "@/components/common/ProxyKeysInput.vue";
import { setShouldConfirmDisableKey, shouldConfirmDisableKey } from "@/utils/preferences";
import { HelpCircle, Save } from "@vicons/ionicons5";
import {
  NButton,
  NAlert,
  NCard,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NInputNumber,
  NSpace,
  NSwitch,
  NTooltip,
  useMessage,
  type FormItemRule,
} from "naive-ui";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n();

const settingList = ref<SettingCategory[]>([]);
const formRef = ref();
const form = ref<Record<string, string | number | boolean>>({});
const savedForm = ref<Record<string, string | number | boolean>>({});
const isSaving = ref(false);
const message = useMessage();
const confirmDisableKey = ref(shouldConfirmDisableKey());
const hasChanges = computed(() => JSON.stringify(form.value) !== JSON.stringify(savedForm.value));
const changedCount = computed(
  () => Object.keys(form.value).filter(key => form.value[key] !== savedForm.value[key]).length
);

fetchSettings();

async function fetchSettings() {
  try {
    const data = await settingsApi.getSettings();
    settingList.value = data || [];
    initForm();
  } catch (_error) {
    message.error(t("settings.loadFailed"));
  }
}

function initForm() {
  form.value = settingList.value.reduce(
    (acc: Record<string, string | number | boolean>, category) => {
      category.settings?.forEach(setting => {
        acc[setting.key] = setting.value;
      });
      return acc;
    },
    {}
  );
  savedForm.value = { ...form.value };
}

async function handleSubmit() {
  if (isSaving.value) {
    return;
  }

  try {
    await formRef.value.validate();
    isSaving.value = true;
    await settingsApi.updateSettings(form.value);
    await fetchSettings();
  } finally {
    isSaving.value = false;
  }
}

function generateValidationRules(item: Setting): FormItemRule[] {
  const rules: FormItemRule[] = [];
  if (item.required) {
    const rule: FormItemRule = {
      required: true,
      message: t("settings.pleaseInput", { field: item.name }),
      trigger: ["input", "blur"],
    };
    if (item.type === "int") {
      rule.type = "number";
    }
    rules.push(rule);
  }
  if (item.type === "int" && item.min_value !== undefined && item.min_value !== null) {
    rules.push({
      validator: (_rule: FormItemRule, value: number) => {
        if (value === null || value === undefined) {
          return true;
        }
        if (item.min_value !== undefined && item.min_value !== null && value < item.min_value) {
          return new Error(t("settings.minValueError", { value: item.min_value }));
        }
        return true;
      },
      trigger: ["input", "blur"],
    });
  }
  return rules;
}

function handleConfirmDisableKeyChange(value: boolean) {
  confirmDisableKey.value = value;
  setShouldConfirmDisableKey(value);
}

function isBasicSettingsCategory(category: SettingCategory) {
  return category.settings?.some(setting => setting.key === "app_url");
}

function discardChanges() {
  form.value = { ...savedForm.value };
}
</script>

<template>
  <div class="settings-shell">
    <header class="settings-intro">
      <div>
        <h1>{{ t("settings.systemDefaults") }}</h1>
        <p>{{ t("settings.systemDefaultsHelp") }}</p>
      </div>
    </header>

    <n-alert type="info" :bordered="false" class="inheritance-note">
      {{ t("settings.inheritanceNote") }}
    </n-alert>

    <n-form ref="formRef" :model="form" label-placement="top">
      <n-space vertical>
        <n-card
          size="small"
          v-for="category in settingList"
          :key="category.category_name"
          :title="category.category_name"
          bordered
        >
          <template #header-extra>
            <span class="category-count">
              {{ t("settings.settingCount", { count: category.settings?.length || 0 }) }}
            </span>
          </template>
          <n-grid :x-gap="36" :y-gap="0" responsive="screen" cols="1 s:2 m:2 l:4 xl:4">
            <n-grid-item
              v-for="item in category.settings"
              :key="item.key"
              :span="item.key === 'proxy_keys' ? 3 : 1"
            >
              <n-form-item :path="item.key" :rule="generateValidationRules(item)">
                <template #label>
                  <n-space align="center" :size="4" :wrap-item="false">
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon
                          :component="HelpCircle"
                          :size="16"
                          style="cursor: help; color: #9ca3af"
                        />
                      </template>
                      {{ item.description }}
                    </n-tooltip>
                    <span>{{ item.name }}</span>
                  </n-space>
                </template>

                <n-input-number
                  v-if="item.type === 'int'"
                  v-model:value="form[item.key] as number"
                  :min="
                    item.min_value !== undefined && item.min_value >= 0 ? item.min_value : undefined
                  "
                  :placeholder="t('settings.inputNumber')"
                  clearable
                  style="width: 100%"
                  size="small"
                />
                <n-switch
                  v-else-if="item.type === 'bool'"
                  v-model:value="form[item.key] as boolean"
                  size="small"
                />
                <proxy-keys-input
                  v-else-if="item.key === 'proxy_keys'"
                  v-model="form[item.key] as string"
                  :placeholder="t('settings.inputContent')"
                  size="small"
                />
                <n-input
                  v-else
                  v-model:value="form[item.key] as string"
                  :placeholder="t('settings.inputContent')"
                  clearable
                  size="small"
                />
              </n-form-item>
            </n-grid-item>

            <n-grid-item v-if="isBasicSettingsCategory(category)">
              <n-form-item>
                <template #label>
                  <n-space align="center" :size="4" :wrap-item="false">
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon
                          :component="HelpCircle"
                          :size="16"
                          style="cursor: help; color: #9ca3af"
                        />
                      </template>
                      {{ t("settings.localPreferences") }}
                    </n-tooltip>
                    <span>{{ t("settings.confirmDisableKey") }}</span>
                  </n-space>
                </template>

                <n-switch
                  :value="confirmDisableKey"
                  size="small"
                  @update:value="handleConfirmDisableKeyChange"
                />
              </n-form-item>
            </n-grid-item>
          </n-grid>
        </n-card>
      </n-space>
    </n-form>

    <div v-if="settingList.length > 0" class="settings-savebar">
      <div>
        <strong>
          {{
            hasChanges
              ? t("settings.unsavedChanges", { count: changedCount })
              : t("settings.allChangesSaved")
          }}
        </strong>
        <span>{{ t("settings.savebarHelp") }}</span>
      </div>
      <n-space>
        <n-button :disabled="!hasChanges || isSaving" @click="discardChanges">
          {{ t("settings.discardChanges") }}
        </n-button>
        <n-button
          type="primary"
          :loading="isSaving"
          :disabled="!hasChanges || isSaving"
          @click="handleSubmit"
        >
          <template #icon>
            <n-icon :component="Save" />
          </template>
          {{ isSaving ? t("settings.saving") : t("settings.saveSettings") }}
        </n-button>
      </n-space>
    </div>
  </div>
</template>

<style scoped>
.settings-shell {
  display: grid;
  gap: 16px;
  padding-bottom: 88px;
}
.settings-intro h1 {
  margin: 0;
  color: var(--text-primary);
  font-size: 1.55rem;
}
.settings-intro p {
  max-width: 72ch;
  margin: 5px 0 0;
  color: var(--text-secondary);
  font-size: 0.88rem;
}
.inheritance-note {
  background: var(--primary-color-suppl);
}
.category-count {
  color: var(--text-secondary);
  font-size: 0.75rem;
}
.settings-savebar {
  position: sticky;
  z-index: 80;
  bottom: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 14px;
  background: var(--card-bg-solid);
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius-lg);
  box-shadow: var(--shadow-sm);
}
.settings-savebar > div:first-child {
  display: grid;
  gap: 2px;
}
.settings-savebar strong {
  color: var(--text-primary);
  font-size: 0.82rem;
}
.settings-savebar span {
  color: var(--text-secondary);
  font-size: 0.74rem;
}
@media (max-width: 640px) {
  .settings-savebar {
    align-items: stretch;
    flex-direction: column;
  }
  .settings-savebar :deep(.n-space) {
    justify-content: stretch;
  }
  .settings-savebar :deep(.n-button) {
    flex: 1;
  }
}
</style>
