import { defineStore } from 'pinia';
import { ref, watch } from 'vue';
import * as keyApi from '@/api/keys';
import type { Key } from '@/types/models';
import { useGroupStore } from './groupStore';

export const useKeyStore = defineStore('key', () => {
  // State
  const keys = ref<Key[]>([]);
  const isLoading = ref(false);
  const groupStore = useGroupStore();

  // Actions
  async function fetchKeys(groupId: string) {
    if (!groupId) {
      keys.value = [];
      return;
    }
    isLoading.value = true;
    try {
      keys.value = await keyApi.fetchKeysInGroup(groupId);
    } catch (error) {
      console.error(`Failed to fetch keys for group ${groupId}:`, error);
      keys.value = []; // 出错时清空列表
    } finally {
      isLoading.value = false;
    }
  }

  // Watch for changes in the selected group and fetch keys accordingly
  watch(() => groupStore.selectedGroupId, (newGroupId) => {
    if (newGroupId) {
      fetchKeys(newGroupId);
    } else {
      keys.value = [];
    }
  }, { immediate: true }); // immediate: true ensures it runs on initialization

  return {
    // State
    keys,
    isLoading,
    // Actions
    fetchKeys,
  };
});