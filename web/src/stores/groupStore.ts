import { defineStore } from "pinia";
import { ref, computed } from "vue";
import * as groupApi from "@/api/groups";
import type { Group } from "@/types/models";
import { useKeyStore } from "./keyStore";

export const useGroupStore = defineStore("group", () => {
  // State
  const groups = ref<Group[]>([]);
  const selectedGroupId = ref<number | null>(null);
  const isLoading = ref(false);

  // Getters
  const selectedGroupDetails = computed(() => {
    if (!selectedGroupId.value) {
      return null;
    }
    return groups.value.find((g) => g.id === selectedGroupId.value) || null;
  });

  // Actions
  async function fetchGroups() {
    isLoading.value = true;
    try {
      groups.value = await groupApi.fetchGroups();
      // 如果没有选中的分组，或者选中的分组已不存在，则默认选中第一个
      const selectedExists = groups.value.some(
        (g) => g.id === selectedGroupId.value
      );
      if (groups.value.length > 0 && !selectedExists) {
        selectGroup(groups.value[0].id);
      }
    } catch (error) {
      console.error("Failed to fetch groups:", error);
    } finally {
      isLoading.value = false;
    }
  }

  function selectGroup(id: number | null) {
    selectedGroupId.value = id;
    const keyStore = useKeyStore();
    if (id) {
      keyStore.fetchKeys(id.toString()); // 暂时转换为string以兼容现有API
    } else {
      keyStore.clearKeys();
    }
  }

  async function fetchGroupKeys(groupId: number) {
    // TODO: 实现获取特定分组的密钥
    console.log("fetchGroupKeys not implemented yet, groupId:", groupId);
    /*
    const group = groups.value.find(g => g.id === groupId);
    if (group) {
      try {
        const keys = await groupApi.fetchGroupKeys(groupId);
        group.api_keys = keys;
      } catch (error) {
        console.error('Failed to fetch group keys:', error);
        throw error;
      }
    }
    */
  }

  async function createGroup(
    groupData: Omit<Group, "id" | "created_at" | "updated_at" | "api_keys">
  ) {
    try {
      const newGroup = await groupApi.createGroup(groupData);
      await fetchGroups(); // Re-fetch to get the full list
      selectGroup(newGroup.id);
    } catch (error) {
      console.error("Failed to create group:", error);
      throw error;
    }
  }

  async function updateGroup(id: number, groupData: Partial<Group>) {
    try {
      await groupApi.updateGroup(id.toString(), groupData); // 暂时转换为string
      await fetchGroups(); // Re-fetch to update the list
    } catch (error) {
      console.error("Failed to update group:", error);
      throw error;
    }
  }

  async function deleteGroup(id: number) {
    try {
      await groupApi.deleteGroup(id.toString()); // 暂时转换为string
      await fetchGroups(); // Re-fetch to update the list
      if (selectedGroupId.value === id) {
        selectedGroupId.value = null;
      }
    } catch (error) {
      console.error("Failed to delete group:", error);
      throw error;
    }
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
    fetchGroupKeys,
    createGroup,
    updateGroup,
    deleteGroup,
  };
});
