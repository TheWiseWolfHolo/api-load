import { defineStore } from "pinia";
import { ref } from "vue";
import * as keyApi from "@/api/keys";
import type { APIKey } from "@/types/models";
import { useGroupStore } from "./groupStore";

export const useKeyStore = defineStore("key", () => {
  // State
  const keys = ref<APIKey[]>([]);
  const selectedKeyIds = ref<number[]>([]);
  const isLoading = ref(false);

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
      keys.value = [];
    } finally {
      isLoading.value = false;
    }
  }

  function setSelectedKeys(ids: number[]) {
    selectedKeyIds.value = ids;
  }

  function clearKeys() {
    keys.value = [];
    selectedKeyIds.value = [];
  }

  async function createKey(
    groupId: string,
    keyData: Omit<
      APIKey,
      | "id"
      | "group_id"
      | "created_at"
      | "updated_at"
      | "request_count"
      | "failure_count"
    >
  ) {
    try {
      await keyApi.createKey(groupId, keyData);
      await fetchKeys(groupId);
    } catch (error) {
      console.error("Failed to create key:", error);
      throw error;
    }
  }

  async function updateKey(id: string, keyData: Partial<APIKey>) {
    const groupStore = useGroupStore();
    try {
      await keyApi.updateKey(id, keyData);
      if (groupStore.selectedGroupId) {
        await fetchKeys(groupStore.selectedGroupId.toString());
      }
    } catch (error) {
      console.error(`Failed to update key ${id}:`, error);
      throw error;
    }
  }

  async function deleteKey(id: string) {
    const groupStore = useGroupStore();
    try {
      await keyApi.deleteKey(id);
      if (groupStore.selectedGroupId) {
        await fetchKeys(groupStore.selectedGroupId.toString());
      }
    } catch (error) {
      console.error(`Failed to delete key ${id}:`, error);
      throw error;
    }
  }

  // 新增方法：更新密钥状态
  async function updateKeyStatus(
    id: number,
    status: "active" | "inactive" | "error"
  ) {
    try {
      await keyApi.updateKey(id.toString(), { status });
      const groupStore = useGroupStore();
      if (groupStore.selectedGroupId) {
        await fetchKeys(groupStore.selectedGroupId.toString());
      }
    } catch (error) {
      console.error(`Failed to update key status ${id}:`, error);
      throw error;
    }
  }

  async function batchUpdateStatus(
    ids: number[],
    status: "active" | "inactive" | "error"
  ) {
    const groupStore = useGroupStore();
    try {
      await keyApi.batchUpdateKeys(
        ids.map((id) => id.toString()),
        { status }
      );
      if (groupStore.selectedGroupId) {
        await fetchKeys(groupStore.selectedGroupId.toString());
        selectedKeyIds.value = []; // Clear selection after batch operation
      }
    } catch (error) {
      console.error("Failed to batch update key status:", error);
      throw error;
    }
  }

  // 新增方法：批量删除
  async function batchDelete(ids: number[]) {
    const groupStore = useGroupStore();
    try {
      await keyApi.batchDeleteKeys(ids.map((id) => id.toString()));
      if (groupStore.selectedGroupId) {
        await fetchKeys(groupStore.selectedGroupId.toString());
        selectedKeyIds.value = []; // Clear selection after batch operation
      }
    } catch (error) {
      console.error("Failed to batch delete keys:", error);
      throw error;
    }
  }

  async function batchDeleteKeys(ids: string[]) {
    const groupStore = useGroupStore();
    try {
      await keyApi.batchDeleteKeys(ids);
      if (groupStore.selectedGroupId) {
        await fetchKeys(groupStore.selectedGroupId.toString());
        selectedKeyIds.value = []; // Clear selection after batch operation
      }
    } catch (error) {
      console.error("Failed to batch delete keys:", error);
      throw error;
    }
  }

  return {
    // State
    keys,
    selectedKeyIds,
    isLoading,
    // Actions
    fetchKeys,
    setSelectedKeys,
    clearKeys,
    createKey,
    updateKey,
    deleteKey,
    updateKeyStatus,
    batchUpdateStatus,
    batchDelete,
    batchDeleteKeys,
  };
});
