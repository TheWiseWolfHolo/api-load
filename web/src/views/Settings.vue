<script setup lang="ts">
import { settingsApi, type SettingCategory } from "@/api/settings";
import { NTooltip } from "naive-ui";
import { ref } from "vue";

const settingList = ref<SettingCategory[]>([]);
const formRef = ref();
const form = ref<Record<string, string | number>>({});
const isSaving = ref(false);

fetchSettings();

async function fetchSettings() {
  const data = await settingsApi.getSettings();
  settingList.value = data || [];
  initForm();
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
  <div>
    <n-form ref="formRef" :model="form" label-placement="left" label-width="110">
      <n-card
        v-for="(category, cIndex) in settingList"
        :key="cIndex"
        :bordered="false"
        :title="category.category_name"
      >
        <n-space>
          <n-form-item
            v-for="item in category.settings"
            :key="item.key"
            :path="item.key"
            style="margin-right: 10px"
            :rule="{
              required: true,
              message: `请输入${item.name}`,
            }"
          >
            <template #label>
              <n-tooltip trigger="hover">
                <template #trigger>
                  <span>{{ item.name }}</span>
                </template>
                <span>{{ item.description }}</span>
              </n-tooltip>
            </template>
            <n-input-number
              v-if="item.type === 'int'"
              v-model:value="form[item.key]"
              :min="item.min_value! >= 0 ? item.min_value : undefined"
              style="width: 120px"
              placeholder=""
              clearable
            />
            <n-input
              v-else
              v-model:value="form[item.key]"
              style="width: 120px"
              placeholder=""
              clearable
            />
          </n-form-item>
        </n-space>
      </n-card>
    </n-form>
  </div>
  <n-flex justify="center">
    <n-button
      v-show="settingList.length > 0"
      type="primary"
      style="width: 200px"
      :loading="isSaving"
      :disabled="isSaving"
      @click="handleSubmit"
    >
      保存设置
    </n-button>
  </n-flex>
</template>
