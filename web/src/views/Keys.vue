<script setup lang="ts">
import { keysApi } from "@/api/keys";
import GroupInfoCard from "@/components/keys/GroupInfoCard.vue";
import GroupList from "@/components/keys/GroupList.vue";
import KeyTable from "@/components/keys/KeyTable.vue";
import type { Group } from "@/types/models";
import { onMounted, ref } from "vue";

const groups = ref<Group[]>([]);
const loading = ref(false);
const selectedGroup = ref<Group | null>(null);

onMounted(async () => {
  await loadGroups();
});

async function loadGroups() {
  try {
    loading.value = true;
    groups.value = await keysApi.getGroups();
    // 默认选择第一个分组
    if (groups.value.length > 0 && !selectedGroup.value) {
      selectedGroup.value = groups.value[0];
    }
  } catch (_error) {
    // 错误已记录
    window.$message.error("加载分组失败");
  } finally {
    loading.value = false;
  }
}

function handleGroupSelect(group: Group) {
  selectedGroup.value = group;
}

function handleGroupRefresh() {
  loadGroups();
}
</script>

<template>
  <div class="keys-container">
    <div class="keys-content">
      <div class="sidebar">
        <group-list
          :groups="groups"
          :selected-group="selectedGroup"
          :loading="loading"
          @group-select="handleGroupSelect"
          @refresh="handleGroupRefresh"
        />
      </div>

      <!-- 右侧主内容区域，占80% -->
      <div class="main-content">
        <!-- 分组信息卡片，更紧凑 -->
        <div v-if="selectedGroup" class="group-info">
          <group-info-card :group="selectedGroup" @refresh="handleGroupRefresh" />
        </div>

        <!-- 密钥表格区域，占主要空间 -->
        <div class="key-table-section">
          <key-table :selected-group="selectedGroup" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.keys-container {
  /* padding: 12px 0; */
  /* max-width: 1600px; */
  /* margin: 0 auto; */
  height: 100%;
  display: flex;
  flex-direction: column;
}

.page-header {
  margin-bottom: 12px;
  padding-bottom: 6px;
  border-bottom: 1px solid #e9ecef;
}

.page-title {
  font-size: 20px;
  font-weight: 600;
  color: #333;
  margin: 0;
}

.keys-content {
  display: flex;
  gap: 12px;
  flex: 1;
  min-height: 0;
}

.sidebar {
  width: 240px;
  flex-shrink: 0;
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.group-info {
  flex-shrink: 0;
}

.key-table-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

@media (max-width: 1024px) {
  .keys-content {
    flex-direction: column;
  }

  .sidebar {
    width: 100%;
  }

  .main-content {
    width: 100%;
  }
}
</style>
