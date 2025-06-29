import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import * as groupApi from '@/api/groups';
import type { Group } from '@/types/models';

export const useGroupStore = defineStore('group', () => {
  // State
  const groups = ref<Group[]>([]);
  const selectedGroupId = ref<string | null>(null);
  const isLoading = ref(false);

  // Getters
  const selectedGroupDetails = computed(() => {
    if (!selectedGroupId.value) {
      return null;
    }
    return groups.value.find(g => g.id === selectedGroupId.value) || null;
  });

  // Actions
  async function fetchGroups() {
    isLoading.value = true;
    try {
      groups.value = await groupApi.fetchGroups();
      // 默认选中第一个分组
      if (groups.value.length > 0 && !selectedGroupId.value) {
        selectedGroupId.value = groups.value[0].id;
      }
    } catch (error) {
      console.error('Failed to fetch groups:', error);
      // 这里可以添加更复杂的错误处理逻辑，例如用户通知
    } finally {
      isLoading.value = false;
    }
  }

  function selectGroup(id: string | null) {
    selectedGroupId.value = id;
  }

  return {
    // State
    groups,
    selectedGroupId,
    isLoading,
    // Getters
    selectedGroupDetails,
    // Actions
    fetchGroups,
    selectGroup,
  };
});