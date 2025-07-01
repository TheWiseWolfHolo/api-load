<template>
  <div class="groups-view">
    <el-row :gutter="20" class="main-layout">
      <!-- 左侧分组列表 -->
      <el-col :xs="24" :sm="8" :md="6" class="left-panel">
        <div class="left-content">
          <GroupList
            :selected-group-id="selectedGroupId"
            @select-group="handleSelectGroup"
            @add-group="handleAddGroup"
            @edit-group="handleEditGroup"
            @delete-group="handleDeleteGroup"
          />
        </div>
      </el-col>

      <!-- 右侧内容区域 -->
      <el-col :xs="24" :sm="16" :md="18" class="right-panel">
        <div v-if="selectedGroup" class="right-content">
          <!-- 分组信息卡片 -->
          <div class="group-info-card">
            <div class="card-header">
              <div class="group-title">
                <h3>{{ selectedGroup.name }}</h3>
                <el-tag
                  :type="getChannelTypeColor(selectedGroup.channel_type)"
                  size="large"
                >
                  {{ getChannelTypeName(selectedGroup.channel_type) }}
                </el-tag>
              </div>
              <div class="card-actions">
                <el-button @click="handleEditGroup(selectedGroup)">
                  编辑分组
                </el-button>
                <el-button
                  type="danger"
                  @click="handleDeleteGroup(selectedGroup.id)"
                >
                  删除分组
                </el-button>
              </div>
            </div>
            <div class="card-content">
              <p class="group-description">
                {{ selectedGroup.description || "暂无描述" }}
              </p>
              <div class="group-stats">
                <div class="stat-item">
                  <span class="stat-label">密钥总数</span>
                  <span class="stat-value">{{ groupKeys.length }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">有效密钥</span>
                  <span class="stat-value">{{ activeKeysCount }}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">总请求数</span>
                  <span class="stat-value">{{ totalRequests }}</span>
                </div>
              </div>
              <div class="group-config" v-if="selectedGroup.config">
                <h4>配置信息</h4>
                <div
                  class="config-item"
                  v-if="selectedGroup.config.upstream_url"
                >
                  <span class="config-label">上游地址:</span>
                  <span class="config-value">{{
                    selectedGroup.config.upstream_url
                  }}</span>
                </div>
                <div class="config-item" v-if="selectedGroup.config.timeout">
                  <span class="config-label">超时时间:</span>
                  <span class="config-value"
                    >{{ selectedGroup.config.timeout }}ms</span
                  >
                </div>
              </div>
            </div>
          </div>

          <!-- 密钥管理区域 -->
          <div class="keys-section">
            <div class="section-header">
              <h4>密钥管理</h4>
            </div>
            <KeyTable
              :keys="groupKeys"
              :loading="loading"
              :group-id="selectedGroupId"
              @add="handleAddKey"
              @edit="handleEditKey"
              @delete="handleDeleteKey"
              @toggle-status="handleToggleKeyStatus"
              @batch-operation="handleBatchOperation"
            />
          </div>
        </div>

        <!-- 未选择分组的提示 -->
        <div v-else class="empty-state">
          <EmptyState
            message="请选择一个分组来查看详情"
            description="在左侧选择一个分组，或者创建新的分组"
          >
            <el-button type="primary" @click="handleAddGroup">
              创建新分组
            </el-button>
          </EmptyState>
        </div>
      </el-col>
    </el-row>

    <!-- 分组表单对话框 -->
    <GroupForm
      v-model:visible="groupFormVisible"
      :group-data="currentGroup"
      @save="handleSaveGroup"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ElRow, ElCol, ElButton, ElTag, ElMessage } from "element-plus";
import GroupList from "@/components/business/groups/GroupList.vue";
import KeyTable from "@/components/business/keys/KeyTable.vue";
import GroupForm from "@/components/business/groups/GroupForm.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import { useGroupStore } from "@/stores/groupStore";
import { useKeyStore } from "@/stores/keyStore";
import type { Group, APIKey } from "@/types/models";

const groupStore = useGroupStore();
const keyStore = useKeyStore();

const selectedGroupId = ref<number | undefined>();
const groupFormVisible = ref(false);
const currentGroup = ref<Group | null>(null);
const loading = ref(false);

// 计算属性
const selectedGroup = computed(() => {
  if (!selectedGroupId.value) return null;
  return groupStore.groups.find((g) => g.id === selectedGroupId.value);
});

const groupKeys = computed(() => {
  if (!selectedGroup.value) return [];
  return selectedGroup.value.api_keys || [];
});

const activeKeysCount = computed(() => {
  return groupKeys.value.filter((key) => key.status === "active").length;
});

const totalRequests = computed(() => {
  return groupKeys.value.reduce((total, key) => total + key.request_count, 0);
});

// 工具函数
const getChannelTypeColor = (channelType: string) => {
  switch (channelType) {
    case "openai":
      return "success";
    case "gemini":
      return "primary";
    default:
      return "info";
  }
};

const getChannelTypeName = (channelType: string) => {
  switch (channelType) {
    case "openai":
      return "OpenAI";
    case "gemini":
      return "Gemini";
    default:
      return channelType;
  }
};

// 事件处理函数
const handleSelectGroup = (groupId: number) => {
  selectedGroupId.value = groupId;
  // 加载分组的密钥数据
  loadGroupKeys(groupId);
};

const handleAddGroup = () => {
  currentGroup.value = null;
  groupFormVisible.value = true;
};

const handleEditGroup = (group: Group) => {
  currentGroup.value = group;
  groupFormVisible.value = true;
};

const handleDeleteGroup = async (groupId: number) => {
  try {
    await groupStore.deleteGroup(groupId);
    ElMessage.success("分组删除成功");
    if (selectedGroupId.value === groupId) {
      selectedGroupId.value = undefined;
    }
  } catch (error) {
    ElMessage.error("删除分组失败");
  }
};

const handleSaveGroup = async (groupData: any) => {
  try {
    if (currentGroup.value) {
      await groupStore.updateGroup(currentGroup.value.id, groupData);
      ElMessage.success("分组更新成功");
    } else {
      await groupStore.createGroup(groupData);
      ElMessage.success("分组创建成功");
    }
    groupFormVisible.value = false;
  } catch (error) {
    ElMessage.error("保存分组失败");
  }
};

const handleAddKey = () => {
  // KeyTable组件会处理添加密钥的逻辑
};

const handleEditKey = (key: APIKey) => {
  // KeyTable组件会处理编辑密钥的逻辑
  console.log("Edit key:", key.id);
};

const handleDeleteKey = async (keyId: number) => {
  try {
    await keyStore.deleteKey(keyId.toString());
    ElMessage.success("密钥删除成功");
    // 重新加载当前分组的密钥
    if (selectedGroupId.value) {
      loadGroupKeys(selectedGroupId.value);
    }
  } catch (error) {
    ElMessage.error("删除密钥失败");
  }
};

const handleToggleKeyStatus = async (key: APIKey) => {
  try {
    const newStatus = key.status === "active" ? "inactive" : "active";
    await keyStore.updateKeyStatus(key.id, newStatus);
    ElMessage.success(`密钥已${newStatus === "active" ? "启用" : "禁用"}`);
    // 重新加载当前分组的密钥
    if (selectedGroupId.value) {
      loadGroupKeys(selectedGroupId.value);
    }
  } catch (error) {
    ElMessage.error("操作失败");
  }
};

const handleBatchOperation = async (operation: string, keys: APIKey[]) => {
  try {
    switch (operation) {
      case "enable":
        await keyStore.batchUpdateStatus(
          keys.map((k) => k.id),
          "active"
        );
        ElMessage.success(`批量启用 ${keys.length} 个密钥成功`);
        break;
      case "disable":
        await keyStore.batchUpdateStatus(
          keys.map((k) => k.id),
          "inactive"
        );
        ElMessage.success(`批量禁用 ${keys.length} 个密钥成功`);
        break;
      case "delete":
        await keyStore.batchDelete(keys.map((k) => k.id));
        ElMessage.success(`批量删除 ${keys.length} 个密钥成功`);
        break;
    }
    // 重新加载当前分组的密钥
    if (selectedGroupId.value) {
      loadGroupKeys(selectedGroupId.value);
    }
  } catch (error) {
    ElMessage.error("批量操作失败");
  }
};

const loadGroupKeys = async (groupId: number) => {
  try {
    loading.value = true;
    await groupStore.fetchGroupKeys(groupId);
  } catch (error) {
    console.error("加载分组密钥失败:", error);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  // 加载分组列表
  groupStore.fetchGroups();
});
</script>

<style scoped>
.groups-view {
  height: 100%;
  padding: 20px;
  box-sizing: border-box;
}

.main-layout {
  height: 100%;
}

.left-panel {
  height: 100%;
}

.left-content {
  background-color: white;
  border-radius: 8px;
  height: 100%;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.right-panel {
  height: 100%;
}

.right-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: 100%;
}

.group-info-card {
  background-color: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  overflow: hidden;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  border-bottom: 1px solid var(--el-border-color-light);
}

.group-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.group-title h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.card-actions {
  display: flex;
  gap: 8px;
}

.card-content {
  padding: 20px;
}

.group-description {
  margin: 0 0 20px 0;
  color: var(--el-text-color-regular);
  line-height: 1.5;
}

.group-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.stat-item {
  text-align: center;
  padding: 12px;
  background-color: var(--el-bg-color-page);
  border-radius: 6px;
}

.stat-label {
  display: block;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}

.stat-value {
  display: block;
  font-size: 20px;
  font-weight: 600;
  color: var(--el-color-primary);
}

.group-config {
  border-top: 1px solid var(--el-border-color-lighter);
  padding-top: 16px;
}

.group-config h4 {
  margin: 0 0 12px 0;
  font-size: 14px;
  color: var(--el-text-color-primary);
}

.config-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}

.config-item:last-child {
  border-bottom: none;
}

.config-label {
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.config-value {
  font-size: 13px;
  color: var(--el-text-color-primary);
  font-family: monospace;
}

.keys-section {
  flex: 1;
  background-color: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  padding: 20px;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.section-header {
  margin-bottom: 16px;
  border-bottom: 1px solid var(--el-border-color-light);
  padding-bottom: 12px;
}

.section-header h4 {
  margin: 0;
  font-size: 16px;
  color: var(--el-text-color-primary);
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  background-color: white;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

/* 响应式设计 */
@media (max-width: 768px) {
  .groups-view {
    padding: 10px;
  }

  .main-layout {
    flex-direction: column;
  }

  .left-panel {
    height: auto;
    margin-bottom: 20px;
  }

  .left-content {
    height: 300px;
  }

  .card-header {
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
  }

  .group-stats {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .group-stats {
    grid-template-columns: 1fr;
  }

  .config-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
}
</style>
