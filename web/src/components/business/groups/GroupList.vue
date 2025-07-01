<template>
  <div class="group-list-container">
    <div class="header">
      <search-input v-model="searchQuery" placeholder="搜索分组..." />
      <el-button type="primary" :icon="Plus" @click="handleAddGroup"
        >添加分组</el-button
      >
    </div>
    <el-scrollbar class="group-list-scrollbar">
      <loading-spinner v-if="groupStore.isLoading" />
      <empty-state
        v-else-if="filteredGroups.length === 0"
        message="未找到分组"
      />
      <ul v-else class="group-list">
        <li
          v-for="group in filteredGroups"
          :key="group.id"
          :class="{ active: group.id === selectedGroupId }"
          @click="handleSelectGroup(group.id)"
        >
          <div class="group-item">
            <span class="group-name">{{ group.name }}</span>
            <div class="group-meta">
              <el-tag
                size="small"
                :type="getChannelTypeColor(group.channel_type)"
              >
                {{ getChannelTypeName(group.channel_type) }}
              </el-tag>
              <span class="key-count"
                >{{ (group.api_keys || []).length }} 密钥</span
              >
            </div>
          </div>
          <div class="group-actions">
            <el-button size="small" text @click.stop="handleEditGroup(group)">
              编辑
            </el-button>
            <el-button
              size="small"
              text
              type="danger"
              @click.stop="handleDeleteGroup(group)"
            >
              删除
            </el-button>
          </div>
        </li>
      </ul>
    </el-scrollbar>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useGroupStore } from "@/stores/groupStore";
import SearchInput from "@/components/common/SearchInput.vue";
import LoadingSpinner from "@/components/common/LoadingSpinner.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import { ElButton, ElScrollbar, ElTag, ElMessageBox } from "element-plus";
import { Plus } from "@element-plus/icons-vue";
import type { Group } from "@/types/models";

interface Props {
  selectedGroupId?: number;
}

const props = defineProps<Props>();
const selectedGroupId = computed(() => props.selectedGroupId);

const emit = defineEmits<{
  (e: "select-group", groupId: number): void;
  (e: "add-group"): void;
  (e: "edit-group", group: Group): void;
  (e: "delete-group", groupId: number): void;
}>();

const groupStore = useGroupStore();
const searchQuery = ref("");

const filteredGroups = computed(() => {
  if (!searchQuery.value) {
    return groupStore.groups;
  }
  return groupStore.groups.filter(
    (group) =>
      group.name.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      group.description.toLowerCase().includes(searchQuery.value.toLowerCase())
  );
});

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

const handleSelectGroup = (groupId: number) => {
  emit("select-group", groupId);
};

const handleAddGroup = () => {
  emit("add-group");
};

const handleEditGroup = (group: Group) => {
  emit("edit-group", group);
};

const handleDeleteGroup = async (group: Group) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除分组 "${group.name}" 吗？这将同时删除该分组下的所有密钥。`,
      "确认删除",
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
      }
    );
    emit("delete-group", group.id);
  } catch {
    // 用户取消删除
  }
};

onMounted(() => {
  if (groupStore.groups.length === 0) {
    groupStore.fetchGroups();
  }
});
</script>

<style scoped>
.group-list-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 16px;
  box-sizing: border-box;
}

.header {
  display: flex;
  gap: 10px;
  margin-bottom: 16px;
}

.group-list-scrollbar {
  flex-grow: 1;
}

.group-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.group-list li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  cursor: pointer;
  border-radius: 8px;
  transition: all 0.2s;
  margin-bottom: 8px;
  border: 1px solid transparent;
}

.group-list li:hover {
  background-color: var(--el-fill-color-light);
  border-color: var(--el-border-color);
}

.group-list li.active {
  background-color: var(--el-color-primary-light-9);
  border-color: var(--el-color-primary);
  color: var(--el-color-primary);
}

.group-item {
  flex: 1;
  min-width: 0;
}

.group-name {
  font-weight: 500;
  font-size: 14px;
  display: block;
  margin-bottom: 4px;
}

.group-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.key-count {
  color: var(--el-text-color-secondary);
}

.group-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s;
}

.group-list li:hover .group-actions {
  opacity: 1;
}
</style>
