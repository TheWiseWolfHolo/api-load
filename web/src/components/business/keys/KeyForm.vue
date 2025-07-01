<template>
  <el-dialog
    v-model="dialogVisible"
    :title="isEdit ? '编辑密钥' : '添加密钥'"
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
      <el-form-item label="API密钥" prop="key_value">
        <el-input
          v-model="formData.key_value"
          type="textarea"
          :rows="3"
          placeholder="请输入完整的API密钥"
          :disabled="isEdit"
        />
        <div class="form-tip">
          <span v-if="isEdit">编辑时无法修改密钥值</span>
          <span v-else>请输入完整的API密钥，支持粘贴多行文本</span>
        </div>
      </el-form-item>

      <el-form-item label="密钥状态" prop="status">
        <el-radio-group v-model="formData.status">
          <el-radio value="active">启用</el-radio>
          <el-radio value="inactive">禁用</el-radio>
        </el-radio-group>
      </el-form-item>

      <el-form-item label="备注">
        <el-input
          v-model="formData.remark"
          placeholder="可选：为此密钥添加备注信息"
          maxlength="200"
          show-word-limit
        />
      </el-form-item>

      <!-- 批量导入模式 -->
      <el-collapse v-if="!isEdit" v-model="activeCollapse">
        <el-collapse-item title="批量导入密钥" name="batch">
          <div class="batch-import-section">
            <el-alert
              title="批量导入说明"
              type="info"
              :closable="false"
              show-icon
            >
              <template #default>
                <p>每行一个密钥，系统会自动分割并创建多个密钥记录</p>
                <p>支持以下格式：</p>
                <ul class="format-list">
                  <li>• sk-xxxxxxxxxxxxxxxxxxxx</li>
                  <li>• sk-proj-xxxxxxxxxxxxxxxxxxxx</li>
                  <li>• 其他格式的API密钥</li>
                </ul>
              </template>
            </el-alert>

            <el-form-item label="批量密钥" style="margin-top: 16px">
              <el-input
                v-model="batchKeys"
                type="textarea"
                :rows="8"
                placeholder="请粘贴多个密钥，每行一个"
                @input="handleBatchKeysChange"
              />
              <div class="batch-info" v-if="parsedBatchKeys.length > 0">
                检测到 {{ parsedBatchKeys.length }} 个密钥
              </div>
            </el-form-item>
          </div>
        </el-collapse-item>
      </el-collapse>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="handleCancel">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">
          {{ submitButtonText }}
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
  ElRadioGroup,
  ElRadio,
  ElButton,
  ElCollapse,
  ElCollapseItem,
  ElAlert,
  ElMessage,
  type FormInstance,
  type FormRules,
} from "element-plus";
import type { APIKey } from "@/types/models";

interface Props {
  visible: boolean;
  keyData?: APIKey | null;
  groupId?: number;
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  keyData: null,
  groupId: undefined,
});

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "save", data: any): void;
}>();

const formRef = ref<FormInstance>();
const dialogVisible = ref(props.visible);
const submitting = ref(false);
const activeCollapse = ref<string[]>([]);
const batchKeys = ref("");

// 计算属性
const isEdit = computed(() => !!props.keyData);

const submitButtonText = computed(() => {
  if (submitting.value) {
    return isEdit.value ? "保存中..." : "创建中...";
  }
  if (parsedBatchKeys.value.length > 1) {
    return `批量创建 ${parsedBatchKeys.value.length} 个密钥`;
  }
  return isEdit.value ? "保存" : "创建密钥";
});

// 表单数据
const formData = reactive<{
  key_value: string;
  status: "active" | "inactive";
  remark: string;
}>({
  key_value: "",
  status: "active",
  remark: "",
});

// 表单验证规则
const formRules: FormRules = {
  key_value: [
    { required: true, message: "请输入API密钥", trigger: "blur" },
    { min: 10, message: "密钥长度至少10位", trigger: "blur" },
  ],
  status: [{ required: true, message: "请选择密钥状态", trigger: "change" }],
};

// 批量密钥解析
const parsedBatchKeys = computed(() => {
  if (!batchKeys.value.trim()) {
    return formData.key_value ? [formData.key_value] : [];
  }

  return batchKeys.value
    .split("\n")
    .map((key) => key.trim())
    .filter((key) => key.length > 0)
    .filter((key, index, arr) => arr.indexOf(key) === index); // 去重
});

// 监听器
watch(
  () => props.visible,
  (val) => {
    dialogVisible.value = val;
    if (val) {
      resetForm();
      if (props.keyData) {
        loadKeyData();
      }
    }
  }
);

watch(dialogVisible, (val) => {
  emit("update:visible", val);
});

// 方法
const resetForm = () => {
  formData.key_value = "";
  formData.status = "active";
  formData.remark = "";
  batchKeys.value = "";
  activeCollapse.value = [];
  submitting.value = false;

  nextTick(() => {
    formRef.value?.clearValidate();
  });
};

const loadKeyData = () => {
  if (props.keyData) {
    formData.key_value = props.keyData.key_value;
    formData.status =
      props.keyData.status === "error" ? "inactive" : props.keyData.status;
    formData.remark = (props.keyData as any).remark || "";
  }
};

const handleBatchKeysChange = () => {
  // 如果有批量密钥输入，清空单个密钥输入
  if (batchKeys.value.trim()) {
    formData.key_value = "";
  }
};

const validateForm = async (): Promise<boolean> => {
  if (!formRef.value) return false;

  try {
    await formRef.value.validate();

    // 检查是否有密钥数据
    if (parsedBatchKeys.value.length === 0) {
      ElMessage.error("请输入至少一个密钥");
      return false;
    }

    // 验证密钥格式
    const invalidKeys = parsedBatchKeys.value.filter((key) => key.length < 10);
    if (invalidKeys.length > 0) {
      ElMessage.error(
        `检测到 ${invalidKeys.length} 个无效密钥，密钥长度至少10位`
      );
      return false;
    }

    return true;
  } catch {
    return false;
  }
};

const handleSubmit = async () => {
  if (!(await validateForm())) return;

  submitting.value = true;

  try {
    const saveData = {
      status: formData.status,
      remark: formData.remark,
      group_id: props.groupId,
    };

    if (isEdit.value) {
      // 编辑模式
      emit("save", {
        ...saveData,
        id: props.keyData!.id,
        key_value: formData.key_value,
      });
    } else if (parsedBatchKeys.value.length === 1) {
      // 单个密钥创建
      emit("save", {
        ...saveData,
        key_value: parsedBatchKeys.value[0],
      });
    } else {
      // 批量创建
      emit("save", {
        ...saveData,
        keys: parsedBatchKeys.value,
        batch: true,
      });
    }

    ElMessage.success(
      isEdit.value
        ? "密钥更新成功"
        : `成功创建 ${parsedBatchKeys.value.length} 个密钥`
    );

    handleClose();
  } catch (error) {
    console.error("Save key failed:", error);
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
.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.batch-import-section {
  margin-top: 16px;
}

.format-list {
  margin: 8px 0;
  padding-left: 16px;
}

.format-list li {
  margin: 4px 0;
  color: var(--el-text-color-regular);
  font-family: monospace;
}

.batch-info {
  margin-top: 8px;
  padding: 8px 12px;
  background-color: var(--el-color-success-light-9);
  border: 1px solid var(--el-color-success-light-7);
  border-radius: 4px;
  color: var(--el-color-success);
  font-size: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

:deep(.el-collapse-item__header) {
  font-weight: 500;
}

:deep(.el-collapse-item__content) {
  padding-bottom: 0;
}
</style>
