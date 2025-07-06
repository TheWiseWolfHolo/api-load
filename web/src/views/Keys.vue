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
  } finally {
    loading.value = false;
  }
}

function handleGroupSelect(group: Group) {
  selectedGroup.value = group;
}

async function handleGroupRefresh() {
  await loadGroups();
  if (selectedGroup.value) {
    // 重新加载当前选中的分组信息
    selectedGroup.value = groups.value.find(g => g.id === selectedGroup.value?.id) || null;
  }
}

function handleGroupDelete(deletedGroup: Group) {
  // 从分组列表中移除已删除的分组
  groups.value = groups.value.filter(g => g.id !== deletedGroup.id);

  // 如果删除的是当前选中的分组，则切换到第一个分组
  if (selectedGroup.value?.id === deletedGroup.id) {
    selectedGroup.value = groups.value.length > 0 ? groups.value[0] : null;
  }
}
</script>

<template>
  <div class="keys-container">
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
      <div class="group-info">
        <group-info-card
          :group="selectedGroup"
          @refresh="handleGroupRefresh"
          @delete="handleGroupDelete"
        />
      </div>

      <!-- 密钥表格区域，占主要空间 -->
      <div class="key-table-section">
        <key-table :selected-group="selectedGroup" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.keys-container {
  display: flex;
  gap: 12px;
  width: 100%;
}

.sidebar {
  width: 240px;
  flex-shrink: 0;
  height: calc(100vh - 106px);
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
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
</style>
