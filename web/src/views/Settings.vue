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
  <div class="settings-container">
    <div class="settings-header">
      <h2 class="settings-title">系统设置</h2>
      <p class="settings-subtitle">配置系统参数和选项</p>
    </div>

    <div class="settings-content">
      <n-form ref="formRef" :model="form" label-placement="top" class="settings-form">
        <div v-for="(category, cIndex) in settingList" :key="cIndex" class="settings-category">
          <n-card class="category-card modern-card" :bordered="false" size="small">
            <template #header>
              <div class="category-header">
                <h3 class="category-title">{{ category.category_name }}</h3>
                <div class="category-divider" />
              </div>
            </template>

            <div class="settings-grid">
              <n-form-item
                v-for="item in category.settings"
                :key="item.key"
                :path="item.key"
                class="setting-item"
                :rule="{
                  required: true,
                  message: `请输入${item.name}`,
                }"
              >
                <template #label>
                  <div class="setting-label">
                    <span class="label-text">{{ item.name }}</span>
                    <n-tooltip trigger="hover" placement="top">
                      <template #trigger>
                        <div class="label-help">
                          <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                            <path
                              d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z"
                            />
                          </svg>
                        </div>
                      </template>
                      <div class="tooltip-content">{{ item.description }}</div>
                    </n-tooltip>
                  </div>
                </template>

                <n-input-number
                  v-if="item.type === 'int'"
                  v-model:value="form[item.key]"
                  :min="item.min_value! >= 0 ? item.min_value : undefined"
                  class="modern-input setting-input"
                  placeholder="请输入数值"
                  clearable
                />
                <n-input
                  v-else
                  v-model:value="form[item.key]"
                  class="modern-input setting-input"
                  placeholder="请输入内容"
                  clearable
                />
              </n-form-item>
            </div>
          </n-card>
        </div>
      </n-form>

      <div class="settings-actions">
        <n-button
          v-show="settingList.length > 0"
          type="primary"
          size="large"
          class="save-button modern-button"
          :loading="isSaving"
          :disabled="isSaving"
          @click="handleSubmit"
        >
          <template #icon>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
              <path
                d="M17 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14c1.1 0 2-.9 2-2V7l-4-4zm-5 16c-1.66 0-3-1.34-3-3s1.34-3 3-3 3 1.34 3 3-1.34 3-3 3zm3-10H5V5h10v4z"
              />
            </svg>
          </template>
          {{ isSaving ? "保存中..." : "保存设置" }}
        </n-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-container {
  max-width: 1000px;
  margin: 0 auto;
}

.settings-header {
  margin-bottom: 32px;
  text-align: center;
}

.settings-title {
  font-size: 2.25rem;
  font-weight: 700;
  background: var(--primary-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0 0 8px 0;
  letter-spacing: -0.5px;
}

.settings-subtitle {
  font-size: 1.1rem;
  color: #64748b;
  margin: 0;
  font-weight: 500;
}

.settings-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.settings-category {
  animation: fadeInUp 0.6s ease-out both;
  margin-bottom: 24px;
}

.settings-category:nth-child(2) {
  animation-delay: 0.1s;
}

.settings-category:nth-child(3) {
  animation-delay: 0.2s;
}

.settings-category:nth-child(4) {
  animation-delay: 0.3s;
}

.category-card {
  background: rgba(255, 255, 255, 0.98);
}

.category-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.category-title {
  font-size: 1.3rem;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.category-divider {
  height: 3px;
  background: var(--primary-gradient);
  border-radius: 2px;
  width: 60px;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
  gap: 12px 10px;
}

.setting-item {
  margin-bottom: 0;
}

.setting-label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.label-text {
  font-weight: 600;
  color: #374151;
  font-size: 0.95rem;
}

.label-help {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgba(102, 126, 234, 0.1);
  color: #667eea;
  cursor: help;
  transition: all 0.2s ease;
}

.label-help:hover {
  background: rgba(102, 126, 234, 0.2);
  transform: scale(1.1);
}

.tooltip-content {
  max-width: 250px;
  font-size: 0.875rem;
  line-height: 1.5;
}

.setting-input {
  width: 100%;
}

.settings-actions {
  display: flex;
  justify-content: center;
  padding-top: 24px;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
}

.save-button {
  min-width: 200px;
  background: var(--primary-gradient);
  border: none;
  font-weight: 600;
  letter-spacing: 0.5px;
  height: 48px;
  font-size: 1rem;
}

.save-button:hover {
  background: linear-gradient(135deg, #5a6fd8 0%, #6a4190 100%);
  transform: translateY(-1px);
  box-shadow: 0 8px 25px rgba(102, 126, 234, 0.3);
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 768px) {
  .settings-title {
    font-size: 1.75rem;
  }

  .settings-grid {
    grid-template-columns: 1fr;
    gap: 20px;
  }

  .save-button {
    width: 100%;
  }
}

:deep(.n-form-item-label) {
  padding: 0;
}

:deep(.n-input) {
  --n-border-radius: 12px;
}

:deep(.n-input-number) {
  --n-border-radius: 12px;
}

:deep(.n-card-header) {
  padding-bottom: 5px;
}
</style>
