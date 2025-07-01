<template>
  <div class="key-table-container">
    <!-- 工具栏 -->
    <div class="table-toolbar mb-4">
      <div class="flex justify-between items-center">
        <div class="flex space-x-2">
          <el-button type="primary" @click="handleAdd">
            <el-icon><Plus /></el-icon>
            添加密钥
          </el-button>
          <el-button @click="handleBatchImport"> 批量导入 </el-button>
          <el-dropdown
            @command="handleBatchOperation"
            v-if="selectedKeys.length > 0"
          >
            <el-button>
              批量操作<el-icon class="el-icon--right"><arrow-down /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="enable">批量启用</el-dropdown-item>
                <el-dropdown-item command="disable">批量禁用</el-dropdown-item>
                <el-dropdown-item command="delete" divided
                  >批量删除</el-dropdown-item
                >
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <div class="flex space-x-2">
          <SearchInput
            v-model="searchKeyword"
            placeholder="搜索密钥..."
            @search="handleSearch"
          />
        </div>
      </div>
    </div>

    <!-- 数据表格 -->
    <DataTable
      :data="filteredKeys"
      :columns="tableColumns"
      :loading="loading"
      selectable
      @selection-change="handleSelectionChange"
    >
      <!-- 密钥值列 - 脱敏显示 -->
      <template #key_value="{ row }">
        <div class="key-value-cell flex items-center space-x-2">
          <span class="font-mono text-sm">
            {{ row.showKey ? row.key_value : maskKey(row.key_value) }}
          </span>
          <el-button size="small" text @click="toggleKeyVisibility(row)">
            <el-icon>
              <component :is="row.showKey ? 'Hide' : 'View'" />
            </el-icon>
          </el-button>
          <el-button
            size="small"
            text
            @click="copyKey(row.key_value)"
            title="复制密钥"
          >
            <el-icon><CopyDocument /></el-icon>
          </el-button>
        </div>
      </template>

      <!-- 状态列 -->
      <template #status="{ row }">
        <StatusBadge :status="row.status" />
      </template>

      <!-- 使用统计列 -->
      <template #usage="{ row }">
        <el-tooltip placement="top">
          <div class="text-center">
            <div class="text-sm font-medium">
              {{ formatNumber(row.request_count) }}
            </div>
            <div class="text-xs text-gray-500" v-if="row.failure_count > 0">
              失败: {{ formatNumber(row.failure_count) }}
            </div>
          </div>
          <template #content>
            <div>总请求: {{ row.request_count }}</div>
            <div>失败次数: {{ row.failure_count }}</div>
            <div v-if="row.last_used_at">
              最后使用: {{ formatTime(row.last_used_at) }}
            </div>
          </template>
        </el-tooltip>
      </template>

      <!-- 操作列 -->
      <template #actions="{ row }">
        <div class="flex space-x-1">
          <el-button size="small" @click="handleEdit(row)"> 编辑 </el-button>
          <el-button
            size="small"
            :type="row.status === 'active' ? 'warning' : 'success'"
            @click="toggleKeyStatus(row)"
          >
            {{ row.status === "active" ? "禁用" : "启用" }}
          </el-button>
          <el-button size="small" type="danger" @click="handleDelete(row)">
            删除
          </el-button>
        </div>
      </template>
    </DataTable>

    <!-- 密钥表单对话框 -->
    <KeyForm
      v-model:visible="formVisible"
      :key-data="currentKey"
      :group-id="currentGroupId"
      @save="handleSave"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, defineProps, withDefaults } from "vue";
import {
  ElButton,
  ElIcon,
  ElDropdown,
  ElDropdownMenu,
  ElDropdownItem,
  ElTooltip,
  ElMessage,
  ElMessageBox,
} from "element-plus";
import { Plus, ArrowDown, CopyDocument } from "@element-plus/icons-vue";
import DataTable from "@/components/common/DataTable.vue";
import StatusBadge from "@/components/common/StatusBadge.vue";
import SearchInput from "@/components/common/SearchInput.vue";
import KeyForm from "./KeyForm.vue";
import type { APIKey } from "@/types/models";
import { maskKey, formatNumber } from "@/types/models";

interface Props {
  keys: APIKey[];
  loading?: boolean;
  groupId?: number;
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  groupId: undefined,
});

const emit = defineEmits<{
  (e: "add"): void;
  (e: "edit", key: APIKey): void;
  (e: "delete", keyId: number): void;
  (e: "toggle-status", key: APIKey): void;
  (e: "batch-operation", operation: string, keys: APIKey[]): void;
}>();

const selectedKeys = ref<APIKey[]>([]);
const searchKeyword = ref("");
const formVisible = ref(false);
const currentKey = ref<APIKey | null>(null);
const currentGroupId = ref<number | undefined>(props.groupId);

// 表格列配置
const tableColumns = [
  { prop: "key_value", label: "API密钥", minWidth: 200 },
  { prop: "status", label: "状态", width: 100 },
  { prop: "usage", label: "使用统计", width: 120 },
  { prop: "created_at", label: "创建时间", width: 150 },
];

// 过滤后的密钥列表
const filteredKeys = computed(() => {
  let keys = props.keys.map((key) => ({
    ...key,
    showKey: false, // 添加显示状态
  }));

  if (searchKeyword.value) {
    const keyword = searchKeyword.value.toLowerCase();
    keys = keys.filter(
      (key) =>
        key.key_value.toLowerCase().includes(keyword) ||
        key.status.toLowerCase().includes(keyword)
    );
  }

  return keys;
});

// 事件处理函数
const handleAdd = () => {
  currentKey.value = null;
  currentGroupId.value = props.groupId;
  formVisible.value = true;
  emit("add");
};

const handleEdit = (key: APIKey) => {
  currentKey.value = key;
  formVisible.value = true;
  emit("edit", key);
};

const handleDelete = async (key: APIKey) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除这个密钥吗？此操作不可恢复。`,
      "确认删除",
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
      }
    );
    emit("delete", key.id);
  } catch {
    // 用户取消删除
  }
};

const toggleKeyStatus = async (key: APIKey) => {
  const action = key.status === "active" ? "禁用" : "启用";
  try {
    await ElMessageBox.confirm(`确定要${action}这个密钥吗？`, `确认${action}`, {
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      type: "warning",
    });
    emit("toggle-status", key);
  } catch {
    // 用户取消操作
  }
};

const handleSelectionChange = (selection: APIKey[]) => {
  selectedKeys.value = selection;
};

const handleBatchImport = () => {
  // TODO: 实现批量导入功能
  ElMessage.info("批量导入功能开发中...");
};

const handleBatchOperation = async (command: string) => {
  if (selectedKeys.value.length === 0) {
    ElMessage.warning("请先选择要操作的密钥");
    return;
  }

  const operationMap = {
    enable: "启用",
    disable: "禁用",
    delete: "删除",
  };

  const operation = operationMap[command as keyof typeof operationMap];

  try {
    await ElMessageBox.confirm(
      `确定要${operation}选中的 ${selectedKeys.value.length} 个密钥吗？`,
      `确认批量${operation}`,
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
      }
    );
    emit("batch-operation", command, selectedKeys.value);
  } catch {
    // 用户取消操作
  }
};

const handleSearch = () => {
  // 搜索逻辑已在computed中处理
};

const handleSave = () => {
  formVisible.value = false;
  // 父组件处理保存逻辑
};

// 工具函数
const toggleKeyVisibility = (key: any) => {
  key.showKey = !key.showKey;
};

const copyKey = async (keyValue: string) => {
  try {
    await navigator.clipboard.writeText(keyValue);
    ElMessage.success("密钥已复制到剪贴板");
  } catch {
    ElMessage.error("复制失败，请手动复制");
  }
};

const formatTime = (timeStr: string) => {
  return new Date(timeStr).toLocaleString("zh-CN");
};
</script>

<style scoped>
.key-table-container {
  width: 100%;
}

.table-toolbar {
  padding: 16px 0;
  border-bottom: 1px solid var(--el-border-color-light);
}

.key-value-cell {
  max-width: 300px;
}

@media (max-width: 768px) {
  .table-toolbar .flex {
    flex-direction: column;
    gap: 12px;
  }
}
</style>
