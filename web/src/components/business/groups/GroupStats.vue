<template>
  <div class="group-stats-container">
    <empty-state
      v-if="!selectedGroup"
      message="请从左侧选择一个分组以查看详情"
    />
    <div v-else class="stats-content">
      <div class="header">
        <h2 class="group-name">{{ selectedGroup.name }}</h2>
        <div class="actions">
          <el-button :icon="Edit" @click="handleEdit">编辑</el-button>
          <el-button type="danger" :icon="Delete" @click="handleDelete">删除</el-button>
        </div>
      </div>
      <p class="group-description">{{ selectedGroup.description || '暂无描述' }}</p>
      <el-row :gutter="20" class="stats-cards">
        <el-col :span="8">
          <el-card shadow="never">
            <div class="stat-item">
              <div class="stat-value">{{ keyStore.keys.length }}</div>
              <div class="stat-label">密钥总数</div>
            </div>
          </el-card>
        </el-col>
        <el-col :span="8">
          <el-card shadow="never">
            <div class="stat-item">
              <div class="stat-value">{{ activeKeysCount }}</div>
              <div class="stat-label">已启用</div>
            </div>
          </el-card>
        </el-col>
        <el-col :span="8">
          <el-card shadow="never">
            <div class="stat-item">
              <div class="stat-value">{{ disabledKeysCount }}</div>
              <div class="stat-label">已禁用</div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useGroupStore } from '@/stores/groupStore';
import { useKeyStore } from '@/stores/keyStore';
import EmptyState from '@/components/common/EmptyState.vue';
import { ElButton, ElRow, ElCol, ElCard, ElMessage } from 'element-plus';
import { Edit, Delete } from '@element-plus/icons-vue';

const groupStore = useGroupStore();
const keyStore = useKeyStore();

const selectedGroup = computed(() => groupStore.selectedGroupDetails);

const activeKeysCount = computed(() => {
  return keyStore.keys.filter(key => key.status === 'active').length;
});

const disabledKeysCount = computed(() => {
  return keyStore.keys.filter(key => key.status !== 'active').length;
});

const handleEdit = () => {
  // TODO: Implement edit group logic (e.g., open a dialog)
  console.log('Edit group:', selectedGroup.value?.id);
  ElMessage.info('编辑功能待实现');
};

const handleDelete = () => {
  // TODO: Implement delete group logic (with confirmation)
  console.log('Delete group:', selectedGroup.value?.id);
  ElMessage.warning('删除功能待实现');
};
</script>

<style scoped>
.group-stats-container {
  width: 100%;
}

.stats-content {
  display: flex;
  flex-direction: column;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.group-name {
  font-size: 24px;
  font-weight: bold;
  margin: 0;
}

.group-description {
  color: #606266;
  margin-bottom: 20px;
  min-height: 22px;
}

.stats-cards .stat-item {
  text-align: center;
}

.stat-value {
  font-size: 28px;
  font-weight: bold;
  color: var(--el-color-primary);
}

.stat-label {
  font-size: 14px;
  color: #909399;
}
</style>