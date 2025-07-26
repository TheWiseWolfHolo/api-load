<script setup lang="ts">
import { settingsApi, type SettingCategory } from "@/api/settings";
import { copy } from "@/utils/clipboard";
import { Copy, HelpCircle, Key, Save } from "@vicons/ionicons5";
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
  NModal,
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

// 密钥生成弹窗相关
const showKeyGeneratorModal = ref(false);
const keyCount = ref(1);
const isGenerating = ref(false);

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

// 生成随机字符串
function generateRandomString(length: number): string {
  const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
  let result = "";
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

// 生成密钥
function generateKeys(): string[] {
  const keys: string[] = [];
  for (let i = 0; i < keyCount.value; i++) {
    keys.push(`sk-${generateRandomString(48)}`);
  }
  return keys;
}

// 打开密钥生成器弹窗
function openKeyGenerator() {
  showKeyGeneratorModal.value = true;
  keyCount.value = 1;
}

// 确认生成密钥
function confirmGenerateKeys() {
  if (isGenerating.value) {
    return;
  }

  try {
    isGenerating.value = true;
    const newKeys = generateKeys();
    const currentValue = (form.value["proxy_keys"] as string) || "";

    let updatedValue = currentValue.trim();

    // 处理逗号兼容情况
    if (updatedValue && !updatedValue.endsWith(",")) {
      updatedValue += ",";
    }

    // 添加新生成的密钥
    if (updatedValue) {
      updatedValue += newKeys.join(",");
    } else {
      updatedValue = newKeys.join(",");
    }

    form.value["proxy_keys"] = updatedValue;
    showKeyGeneratorModal.value = false;

    message.success(`成功生成 ${keyCount.value} 个密钥`);
  } finally {
    isGenerating.value = false;
  }
}

// 复制代理密钥
async function copyProxyKeys() {
  const proxyKeys = (form.value["proxy_keys"] as string) || "";
  if (!proxyKeys.trim()) {
    message.warning("暂无密钥可复制");
    return;
  }

  // 将逗号分隔的密钥转换为换行分隔
  const formattedKeys = proxyKeys
    .split(",")
    .map(key => key.trim())
    .filter(key => key.length > 0)
    .join("\n");

  const success = await copy(formattedKeys);
  if (success) {
    message.success("密钥已复制到剪贴板");
  } else {
    message.error("复制失败，请手动复制");
  }
}
</script>

<template>
  <n-space vertical>
    <n-form ref="formRef" :model="form" label-placement="top">
      <n-space vertical>
        <n-card
          size="small"
          v-for="category in settingList"
          :key="category.category_name"
          :title="category.category_name"
          hoverable
          bordered
        >
          <n-grid :x-gap="36" :y-gap="0" responsive="screen" cols="1 s:2 m:2 l:3 xl:3">
            <n-grid-item
              v-for="item in category.settings"
              :key="item.key"
              :span="item.key === 'proxy_keys' ? 3 : 1"
            >
              <n-form-item
                :path="item.key"
                :rule="{ required: true, message: `请输入 ${item.name}` }"
              >
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
                  placeholder="请输入数值"
                  clearable
                  style="width: 100%"
                  size="small"
                />
                <n-input
                  v-else
                  v-model:value="form[item.key] as string"
                  placeholder="请输入内容"
                  clearable
                  size="small"
                >
                  <template v-if="item.key === 'proxy_keys'" #suffix>
                    <n-space :size="4" :wrap-item="false">
                      <n-button text type="primary" size="small" @click="openKeyGenerator">
                        <template #icon>
                          <n-icon :component="Key" />
                        </template>
                        生成
                      </n-button>
                      <n-button
                        text
                        type="tertiary"
                        size="small"
                        @click="copyProxyKeys"
                        style="opacity: 0.7"
                      >
                        <template #icon>
                          <n-icon :component="Copy" />
                        </template>
                        复制
                      </n-button>
                    </n-space>
                  </template>
                </n-input>
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
          <n-icon :component="Save" />
        </template>
        {{ isSaving ? "保存中..." : "保存设置" }}
      </n-button>
    </div>

    <!-- 密钥生成器弹窗 -->
    <n-modal
      v-model:show="showKeyGeneratorModal"
      preset="dialog"
      title="生成代理密钥"
      positive-text="确认生成"
      negative-text="取消"
      :positive-button-props="{ loading: isGenerating }"
      @positive-click="confirmGenerateKeys"
    >
      <n-space vertical :size="16">
        <div>
          <p style="margin: 0 0 8px 0; color: #666; font-size: 14px">
            请输入要生成的密钥数量（最大100个）：
          </p>
          <n-input-number
            v-model:value="keyCount"
            :min="1"
            :max="100"
            placeholder="请输入数量"
            style="width: 100%"
            :disabled="isGenerating"
          />
        </div>
        <div style="color: #999; font-size: 12px; line-height: 1.4">
          <p style="margin: 0">密钥格式：sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx</p>
          <p style="margin: 4px 0 0 0">生成的密钥将会插入到当前输入框内容的后面，以逗号分隔</p>
        </div>
      </n-space>
    </n-modal>
  </n-space>
</template>
