<template>
  <el-dialog
    v-model="dialogVisible"
    :title="isEdit ? '编辑分组' : '创建分组'"
    width="600px"
    :before-close="handleClose"
    @closed="handleClosed"
  >
    <el-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      label-width="120px"
    >
      <el-form-item label="分组名称" prop="name">
        <el-input
          v-model="formData.name"
          placeholder="请输入分组名称"
          maxlength="50"
          show-word-limit
        />
      </el-form-item>

      <el-form-item label="分组描述">
        <el-input
          v-model="formData.description"
          type="textarea"
          :rows="3"
          placeholder="请输入分组描述（可选）"
          maxlength="200"
          show-word-limit
        />
      </el-form-item>

      <el-form-item label="渠道类型" prop="channel_type">
        <el-radio-group v-model="formData.channel_type">
          <el-radio value="openai">
            <div class="channel-option">
              <span class="channel-name">OpenAI</span>
              <span class="channel-desc">支持 GPT-3.5、GPT-4 等模型</span>
            </div>
          </el-radio>
          <el-radio value="gemini">
            <div class="channel-option">
              <span class="channel-name">Gemini</span>
              <span class="channel-desc">Google 的 Gemini 模型</span>
            </div>
          </el-radio>
        </el-radio-group>
      </el-form-item>

      <el-divider content-position="left">配置设置</el-divider>

      <el-form-item label="上游地址" prop="config.upstream_url">
        <el-input
          v-model="formData.config.upstream_url"
          placeholder="请输入API上游地址"
        />
        <div class="form-tip">
          例如：https://api.openai.com 或
          https://generativelanguage.googleapis.com
        </div>
      </el-form-item>

      <el-form-item label="超时时间">
        <el-input-number
          v-model="formData.config.timeout"
          :min="1000"
          :max="300000"
          :step="1000"
          placeholder="请输入超时时间"
        />
        <span class="input-suffix">毫秒</span>
        <div class="form-tip">请求超时时间，范围：1秒 - 5分钟，默认30秒</div>
      </el-form-item>

      <el-form-item label="最大令牌数">
        <el-input-number
          v-model="formData.config.max_tokens"
          :min="1"
          :max="32000"
          placeholder="请输入最大令牌数"
        />
        <div class="form-tip">单次请求最大令牌数，留空使用模型默认值</div>
      </el-form-item>

      <!-- 高级配置 -->
      <el-collapse v-model="activeCollapse">
        <el-collapse-item title="高级配置" name="advanced">
          <el-form-item label="请求头">
            <div class="config-editor">
              <el-input
                v-model="headersText"
                type="textarea"
                :rows="4"
                placeholder="请输入自定义请求头配置（JSON格式）"
                @blur="validateHeaders"
              />
              <div class="form-tip">
                格式：{"Authorization": "Bearer token", "Custom-Header":
                "value"}
              </div>
            </div>
          </el-form-item>

          <el-form-item label="其他配置">
            <div class="config-editor">
              <el-input
                v-model="otherConfigText"
                type="textarea"
                :rows="4"
                placeholder="请输入其他配置项（JSON格式）"
                @blur="validateOtherConfig"
              />
              <div class="form-tip">其他自定义配置参数，将合并到分组配置中</div>
            </div>
          </el-form-item>
        </el-collapse-item>
      </el-collapse>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="handleCancel">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">
          {{ isEdit ? "保存修改" : "创建分组" }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from "vue";
import {
  ElDialog,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
  ElRadioGroup,
  ElRadio,
  ElButton,
  ElDivider,
  ElCollapse,
  ElCollapseItem,
  ElMessage,
  type FormInstance,
  type FormRules,
} from "element-plus";
import type { Group, GroupConfig } from "@/types/models";

interface Props {
  visible: boolean;
  groupData?: Group | null;
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  groupData: null,
});

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "save", data: any): void;
}>();

const formRef = ref<FormInstance>();
const dialogVisible = ref(props.visible);
const submitting = ref(false);
const activeCollapse = ref<string[]>([]);
const headersText = ref("");
const otherConfigText = ref("");

// 计算属性
const isEdit = computed(() => !!props.groupData);

// 表单数据
const formData = reactive<{
  name: string;
  description: string;
  channel_type: "openai" | "gemini";
  config: GroupConfig;
}>({
  name: "",
  description: "",
  channel_type: "openai",
  config: {
    upstream_url: "",
    timeout: 30000,
    max_tokens: undefined,
  },
});

// 表单验证规则
const formRules: FormRules = {
  name: [
    { required: true, message: "请输入分组名称", trigger: "blur" },
    { min: 2, max: 50, message: "分组名称长度为2-50个字符", trigger: "blur" },
  ],
  channel_type: [
    { required: true, message: "请选择渠道类型", trigger: "change" },
  ],
  "config.upstream_url": [
    { required: true, message: "请输入上游地址", trigger: "blur" },
    {
      pattern: /^https?:\/\/.+/,
      message: "请输入有效的HTTP/HTTPS地址",
      trigger: "blur",
    },
  ],
};

// 监听器
watch(
  () => props.visible,
  (val) => {
    dialogVisible.value = val;
    if (val) {
      resetForm();
      if (props.groupData) {
        loadGroupData();
      } else {
        setDefaultConfig();
      }
    }
  }
);

watch(dialogVisible, (val) => {
  emit("update:visible", val);
});

watch(
  () => formData.channel_type,
  (newType) => {
    setDefaultConfig(newType);
  }
);

// 方法
const resetForm = () => {
  formData.name = "";
  formData.description = "";
  formData.channel_type = "openai";
  formData.config = {
    upstream_url: "",
    timeout: 30000,
    max_tokens: undefined,
  };
  headersText.value = "";
  otherConfigText.value = "";
  activeCollapse.value = [];
  submitting.value = false;

  nextTick(() => {
    formRef.value?.clearValidate();
  });
};

const setDefaultConfig = (channelType?: "openai" | "gemini") => {
  const type = channelType || formData.channel_type;

  if (!isEdit.value) {
    switch (type) {
      case "openai":
        formData.config.upstream_url = "https://api.openai.com";
        break;
      case "gemini":
        formData.config.upstream_url =
          "https://generativelanguage.googleapis.com";
        break;
    }
  }
};

const loadGroupData = () => {
  if (props.groupData) {
    formData.name = props.groupData.name;
    formData.description = props.groupData.description;
    formData.channel_type = props.groupData.channel_type;
    formData.config = { ...props.groupData.config };

    // 解析高级配置
    if (props.groupData.config.headers) {
      headersText.value = JSON.stringify(
        props.groupData.config.headers,
        null,
        2
      );
    }

    // 提取其他配置（排除已知字段）
    const { upstream_url, timeout, max_tokens, headers, ...otherConfig } =
      props.groupData.config;
    if (Object.keys(otherConfig).length > 0) {
      otherConfigText.value = JSON.stringify(otherConfig, null, 2);
    }
  }
};

const validateHeaders = () => {
  if (!headersText.value.trim()) return;

  try {
    JSON.parse(headersText.value);
  } catch {
    ElMessage.error("请求头配置格式错误，请检查JSON语法");
    return false;
  }
  return true;
};

const validateOtherConfig = () => {
  if (!otherConfigText.value.trim()) return;

  try {
    JSON.parse(otherConfigText.value);
  } catch {
    ElMessage.error("其他配置格式错误，请检查JSON语法");
    return false;
  }
  return true;
};

const validateForm = async (): Promise<boolean> => {
  if (!formRef.value) return false;

  try {
    await formRef.value.validate();

    // 验证高级配置
    if (headersText.value.trim() && !validateHeaders()) {
      return false;
    }

    if (otherConfigText.value.trim() && !validateOtherConfig()) {
      return false;
    }

    return true;
  } catch {
    return false;
  }
};

const buildConfigData = () => {
  const config: GroupConfig = {
    upstream_url: formData.config.upstream_url,
    timeout: formData.config.timeout,
  };

  if (formData.config.max_tokens) {
    config.max_tokens = formData.config.max_tokens;
  }

  // 添加自定义请求头
  if (headersText.value.trim()) {
    try {
      config.headers = JSON.parse(headersText.value);
    } catch {
      // 已在验证中处理
    }
  }

  // 添加其他配置
  if (otherConfigText.value.trim()) {
    try {
      const otherConfig = JSON.parse(otherConfigText.value);
      Object.assign(config, otherConfig);
    } catch {
      // 已在验证中处理
    }
  }

  return config;
};

const handleSubmit = async () => {
  if (!(await validateForm())) return;

  submitting.value = true;

  try {
    const saveData = {
      name: formData.name,
      description: formData.description,
      channel_type: formData.channel_type,
      config: buildConfigData(),
    };

    if (isEdit.value) {
      emit("save", {
        ...saveData,
        id: props.groupData!.id,
      });
    } else {
      emit("save", saveData);
    }

    ElMessage.success(isEdit.value ? "分组更新成功" : "分组创建成功");
    handleClose();
  } catch (error) {
    console.error("Save group failed:", error);
    ElMessage.error("操作失败，请重试");
  } finally {
    submitting.value = false;
  }
};

const handleCancel = () => {
  handleClose();
};

const handleClose = () => {
  if (submitting.value) {
    ElMessage.warning("操作进行中，请稍后");
    return;
  }
  dialogVisible.value = false;
};

const handleClosed = () => {
  resetForm();
};
</script>

<style scoped>
.channel-option {
  display: flex;
  flex-direction: column;
  margin-left: 8px;
}

.channel-name {
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.channel-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}

.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.input-suffix {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.config-editor {
  width: 100%;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

:deep(.el-radio) {
  display: flex;
  align-items: flex-start;
  margin-bottom: 16px;
  margin-right: 30px;
}

:deep(.el-radio__input) {
  margin-top: 2px;
}

:deep(.el-collapse-item__header) {
  font-weight: 500;
}

:deep(.el-input-number) {
  width: 200px;
}
</style>
