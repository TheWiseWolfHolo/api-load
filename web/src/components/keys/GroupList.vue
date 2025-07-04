<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group } from "@/types/models";
import { useMessage } from "naive-ui";
import { computed, ref } from "vue";

interface Props {
  groups: Group[];
  selectedGroup: Group | null;
  loading?: boolean;
}

interface Emits {
  (e: "group-select", group: Group): void;
  (e: "refresh"): void;
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
});

const emit = defineEmits<Emits>();

const searchText = ref("");
const message = useMessage();

// 过滤后的分组列表
const filteredGroups = computed(() => {
  if (!searchText.value) {
    return props.groups;
  }
  const search = searchText.value.toLowerCase();
  return props.groups.filter(
    group =>
      group.name.toLowerCase().includes(search) ||
      (group.display_name && group.display_name.toLowerCase().includes(search))
  );
});

function handleGroupClick(group: Group) {
  emit("group-select", group);
}

// 简单的创建分组功能（演示用）
async function createDemoGroup() {
  try {
    const newGroup = await keysApi.createGroup({
      name: `demo-group-${Date.now()}`,
      display_name: `演示分组 ${props.groups.length + 1}`,
      description: "这是一个演示分组",
      sort: props.groups.length + 1,
      channel_type: "openai",
      upstreams: [{ url: "https://api.openai.com", weight: 1 }],
      config: {
        test_model: "gpt-3.5-turbo",
        param_overrides: {},
        request_timeout: 30000,
      },
    });

    message.success(`创建分组成功: ${newGroup.display_name}`);
    emit("refresh");
  } catch (error) {
    console.error("创建分组失败:", error);
    message.error("创建分组失败");
  }
}
</script>

<template>
  <div class="group-list-container">
    <div class="group-list-card">
      <!-- 搜索框 -->
      <div class="search-section">
        <input v-model="searchText" placeholder="搜索分组名称..." class="search-input" />
      </div>

      <!-- 分组列表 -->
      <div class="groups-section">
        <div v-if="loading" class="loading-state">加载中...</div>
        <div v-else-if="filteredGroups.length === 0" class="empty-state">
          <div class="empty-text">
            {{ searchText ? "未找到匹配的分组" : "暂无分组" }}
          </div>
        </div>
        <div v-else class="groups-list">
          <div
            v-for="group in filteredGroups"
            :key="group.id"
            class="group-item"
            :class="{ active: selectedGroup?.id === group.id }"
            @click="handleGroupClick(group)"
          >
            <div class="group-icon">
              <span v-if="group.channel_type === 'openai'">🤖</span>
              <span v-else-if="group.channel_type === 'gemini'">💎</span>
              <span v-else-if="group.channel_type === 'silicon'">⚡</span>
              <span v-else>🔧</span>
            </div>
            <div class="group-content">
              <div class="group-name">
                {{ group.display_name || group.name }}
              </div>
              <div class="group-info">
                <span class="channel-type">{{ group.channel_type }}</span>
                <span class="group-id">#{{ group.id }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 添加分组按钮 -->
      <div class="add-section">
        <button class="add-button" @click="createDemoGroup">
          <span class="add-icon">+</span>
          添加分组
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.group-list-container {
  height: 100%;
}

.group-list-card {
  height: 100%;
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  padding: 12px;
  display: flex;
  flex-direction: column;
}

.search-section {
  margin-bottom: 12px;
}

.search-input {
  width: 100%;
  padding: 6px 8px;
  border: 1px solid #e9ecef;
  border-radius: 4px;
  font-size: 12px;
  background: #f8f9fa;
}

.search-input:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.groups-section {
  flex: 1;
  min-height: 0;
  margin-bottom: 12px;
}

.loading-state {
  text-align: center;
  padding: 20px;
  color: #6c757d;
  font-size: 14px;
}

.empty-state {
  text-align: center;
  padding: 20px;
  color: #6c757d;
}

.empty-text {
  font-size: 12px;
}

.groups-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 100%;
  overflow-y: auto;
}

.group-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid transparent;
  font-size: 12px;
}

.group-item:hover {
  background: #f8f9fa;
  border-color: #e9ecef;
}

.group-item.active {
  background: #007bff;
  color: white;
  border-color: #007bff;
}

.group-icon {
  font-size: 14px;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
  flex-shrink: 0;
}

.group-item.active .group-icon {
  background: rgba(255, 255, 255, 0.2);
}

.group-content {
  flex: 1;
  min-width: 0;
}

.group-name {
  font-weight: 600;
  font-size: 14px;
  line-height: 1.2;
  margin-bottom: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-info {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  opacity: 0.7;
}

.channel-type {
  text-transform: uppercase;
  font-weight: 500;
}

.group-id {
  opacity: 0.6;
}

.add-section {
  border-top: 1px solid #e9ecef;
  padding-top: 12px;
}

.add-button {
  width: 100%;
  padding: 8px;
  border: 1px solid #007bff;
  background: #007bff;
  color: white;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: background-color 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

.add-button:hover {
  background: #0056b3;
}

.add-icon {
  font-size: 14px;
}

/* 滚动条样式 */
.groups-list::-webkit-scrollbar {
  width: 4px;
}

.groups-list::-webkit-scrollbar-track {
  background: transparent;
}

.groups-list::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.2);
  border-radius: 2px;
}

.groups-list::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 0, 0, 0.3);
}

@media (max-width: 768px) {
  .group-item {
    padding: 6px;
  }

  .group-name {
    font-size: 11px;
  }

  .group-info {
    font-size: 9px;
  }
}
</style>
