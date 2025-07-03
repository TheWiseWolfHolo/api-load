<script setup lang="ts">
import http from "@/utils/http";
import type { FormValidationError } from "naive-ui";
import { ref } from "vue";

interface Setting {
  key: string;
  name: string;
  value: string | number;
  type: "int" | "string";
  min_value?: number;
}

interface SettingCategory {
  category_name: string;
  settings: Setting[];
}

const settingList = ref<SettingCategory[]>([]);
const loading = ref(false);
const formRef = ref();
const form = ref<Record<string, string | number>>({});

fetchSettings();

async function fetchSettings() {
  loading.value = true;
  try {
    const response = await http.get("/settings");
    settingList.value = response.data || [];
    initForm();
  } finally {
    loading.value = false;
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

function handleSubmit() {
  if (loading.value) {
    return;
  }

  formRef.value.validate(async (errors: Array<FormValidationError> | undefined) => {
    if (errors) {
      return;
    }

    try {
      loading.value = true;
      await http.post("/settings", form.value);
      fetchSettings();
    } finally {
      loading.value = false;
    }
  });
}
</script>

<template>
  <n-spin :show="loading">
    <n-form
      ref="formRef"
      :model="form"
      label-placement="left"
      label-width="110"
      :disabled="loading"
    >
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
            :label="item.name"
            :path="item.key"
            style="margin-right: 10px"
            :rule="{
              required: true,
              message: `请输入${item.name}`,
            }"
          >
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
  </n-spin>
  <n-flex justify="center">
    <n-button
      v-show="settingList.length > 0"
      type="primary"
      style="width: 200px"
      :loading="loading"
      @click="handleSubmit"
    >
      保存设置
    </n-button>
  </n-flex>
</template>
