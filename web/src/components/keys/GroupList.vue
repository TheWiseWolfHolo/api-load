<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group } from "@/types/models";
import { getGroupDisplayName } from "@/utils/display";
import { NButton, NCard, NEmpty, NInput, NSpin, NTag, useMessage } from "naive-ui";
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

// 获取渠道类型的标签颜色
function getChannelTagType(channelType: string) {
  switch (channelType) {
    case "openai":
      return "success";
    case "gemini":
      return "info";
    default:
      return "default";
  }
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
  } catch (_error) {
    // 错误已记录
    message.error("创建分组失败");
  }
}
</script>

<template>
  <div class="group-list-container">
    <n-card class="group-list-card modern-card" :bordered="false" size="small">
      <!-- 搜索框 -->
      <div class="search-section">
        <n-input v-model:value="searchText" placeholder="搜索分组名称..." size="small" clearable>
          <template #prefix>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <path
                d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"
              />
            </svg>
          </template>
        </n-input>
      </div>

      <!-- 分组列表 -->
      <div class="groups-section">
        <n-spin :show="loading" size="small">
          <div v-if="filteredGroups.length === 0 && !loading" class="empty-container">
            <n-empty size="small" :description="searchText ? '未找到匹配的分组' : '暂无分组'" />
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
                <span v-else>🔧</span>
              </div>
              <div class="group-content">
                <div class="group-name">{{ getGroupDisplayName(group) }}</div>
                <div class="group-meta">
                  <n-tag size="tiny" :type="getChannelTagType(group.channel_type)">
                    {{ group.channel_type }}
                  </n-tag>
                  <span class="group-id">#{{ group.name }}</span>
                </div>
              </div>
            </div>
          </div>
        </n-spin>
      </div>

      <!-- 添加分组按钮 -->
      <div class="add-section">
        <n-button type="primary" size="small" block @click="createDemoGroup">
          <template #icon>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
            </svg>
          </template>
          创建分组
        </n-button>
      </div>
    </n-card>
  </div>
</template>

<style scoped>
:deep(.n-card__content) {
  height: 100%;
}

.groups-section::-webkit-scrollbar {
  width: 1px;
  height: 1px;
}

.group-list-container {
  height: 100%;
}

.group-list-card {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.group-list-card:hover {
  transform: none;
  box-shadow: var(--shadow-lg);
}

.search-section {
  height: 41px;
}

.groups-section {
  flex: 1;
  height: calc(100% - 82px);
  overflow: auto;
}

.empty-container {
  padding: 20px 0;
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
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
  font-size: 12px;
}

.group-item:hover {
  background: rgba(102, 126, 234, 0.1);
  border-color: rgba(102, 126, 234, 0.2);
}

.group-item.active {
  background: var(--primary-gradient);
  color: white;
  border-color: transparent;
  box-shadow: var(--shadow-md);
}

.group-icon {
  font-size: 16px;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(102, 126, 234, 0.1);
  border-radius: 6px;
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
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
}

.group-id {
  opacity: 0.7;
  color: #64748b;
}

.group-item.active .group-id {
  opacity: 0.8;
  color: rgba(255, 255, 255, 0.8);
}

.add-section {
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  padding-top: 12px;
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

  .group-meta {
    font-size: 9px;
  }
}
</style>
