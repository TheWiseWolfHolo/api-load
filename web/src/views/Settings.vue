<script setup lang="ts">
import { settingsApi, type SettingCategory } from "@/api/settings";
import {
  NButton,
  NCard,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NIcon,
  NInput,
  NInputNumber,
  NSpace,
  NTooltip,
  useMessage,
} from "naive-ui";
import { ref } from "vue";

const settingList = ref<SettingCategory[]>([]);
const formRef = ref();
const form = ref<Record<string, string | number>>({});
const isSaving = ref(false);
const message = useMessage();

fetchSettings();

async function fetchSettings() {
  try {
    const data = await settingsApi.getSettings();
    settingList.value = data || [];
    initForm();
  } catch (_error) {
    message.error("获取设置失败");
  }
}

function initForm() {
  form.value = settingList.value.reduce((acc: Record<string, string | number>, category) => {
    category.settings?.forEach(setting => {
      acc[setting.key] = setting.value;
    });
    return acc;
  }, {});
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
</script>

<template>
  <n-space vertical size="large">
    <n-form ref="formRef" :model="form" label-placement="top">
      <n-space vertical size="large">
        <n-card
          size="small"
          v-for="category in settingList"
          :key="category.category_name"
          :title="category.category_name"
          hoverable
          bordered
        >
          <n-grid :x-gap="24" :y-gap="24" responsive="screen" cols="1 s:2 m:2 l:3 xl:4">
            <n-grid-item v-for="item in category.settings" :key="item.key">
              <n-form-item
                :path="item.key"
                :rule="{ required: true, message: `请输入 ${item.name}` }"
              >
                <template #label>
                  <n-space align="center" :size="4" :wrap-item="false">
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <n-icon :size="16" style="cursor: help; color: #9ca3af">
                          <svg viewBox="0 0 24 24" fill="currentColor">
                            <path
                              d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z"
                            />
                          </svg>
                        </n-icon>
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
                  placeholder="请输入数值"
                  clearable
                  style="width: 100%"
                />
                <n-input
                  v-else
                  v-model:value="form[item.key] as string"
                  placeholder="请输入内容"
                  clearable
                />
              </n-form-item>
            </n-grid-item>
          </n-grid>
        </n-card>
      </n-space>
    </n-form>

    <div
      v-if="settingList.length > 0"
      style="display: flex; justify-content: center; padding-top: 12px"
    >
      <n-button
        type="primary"
        size="large"
        :loading="isSaving"
        :disabled="isSaving"
        @click="handleSubmit"
        style="min-width: 200px"
      >
        <template #icon>
          <n-icon>
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path
                d="M17 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14c1.1 0 2-.9 2-2V7l-4-4zm-5 16c-1.66 0-3-1.34-3-3s1.34-3 3-3 3 1.34 3 3-1.34 3-3 3zm3-10H5V5h10v4z"
              />
            </svg>
          </n-icon>
        </template>
        {{ isSaving ? "保存中..." : "保存设置" }}
      </n-button>
    </div>
  </n-space>
</template>
