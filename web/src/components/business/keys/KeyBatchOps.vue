<template>
  <div class="key-batch-ops-container">
    <div class="batch-actions">
      <el-button @click="handleBatchEnable" :disabled="!hasSelection">
        批量启用
      </el-button>
      <el-button
        type="warning"
        @click="handleBatchDisable"
        :disabled="!hasSelection"
      >
        批量禁用
      </el-button>
      <el-button
        type="danger"
        @click="handleBatchDelete"
        :disabled="!hasSelection"
      >
        批量删除
      </el-button>
    </div>
    <el-button type="primary" :icon="Plus" @click="handleAddNew">
      添加密钥
    </el-button>
    <key-form v-model:visible="isFormVisible" :key-data="null" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { useKeyStore } from "@/stores/keyStore";
import KeyForm from "./KeyForm.vue";
import { ElButton, ElMessage, ElMessageBox } from "element-plus";
import { Plus } from "@element-plus/icons-vue";

const keyStore = useKeyStore();
const isFormVisible = ref(false);

const hasSelection = computed(() => keyStore.selectedKeyIds.length > 0);

const handleAddNew = () => {
  isFormVisible.value = true;
};

const createBatchHandler = (action: "启用" | "禁用" | "删除") => {
  const actionMap: {
    [key: string]: { status?: "active" | "inactive"; verb: string };
  } = {
    启用: { status: "active", verb: "启用" },
    禁用: { status: "inactive", verb: "禁用" },
    删除: { verb: "删除" },
  };

  return async () => {
    const selectedIds = keyStore.selectedKeyIds;
    if (selectedIds.length === 0) {
      ElMessage.warning("请至少选择一个密钥");
      return;
    }

    try {
      await ElMessageBox.confirm(
        `确定要${actionMap[action].verb}选中的 ${selectedIds.length} 个密钥吗？`,
        "警告",
        {
          confirmButtonText: `确定${actionMap[action].verb}`,
          cancelButtonText: "取消",
          type: "warning",
        }
      );

      if (action === "删除") {
        await keyStore.batchDelete(selectedIds);
      } else {
        await keyStore.batchUpdateStatus(
          selectedIds,
          actionMap[action].status!
        );
      }
      ElMessage.success(`选中的密钥已${actionMap[action].verb}`);
    } catch (error) {
      if (error !== "cancel") {
        ElMessage.error(`批量${actionMap[action].verb}操作失败`);
      } else {
        ElMessage.info("操作已取消");
      }
    }
  };
};

const handleBatchEnable = createBatchHandler("启用");
const handleBatchDisable = createBatchHandler("禁用");
const handleBatchDelete = createBatchHandler("删除");
</script>

<style scoped>
.key-batch-ops-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.batch-actions {
  display: flex;
  gap: 10px;
}
</style>
