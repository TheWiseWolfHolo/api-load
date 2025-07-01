import apiClient from "./index";
import type { Key } from "../types/models";

/**
 * 获取指定分组下的所有密钥列表
 * @param groupId 分组ID
 */
export const fetchKeysInGroup = (groupId: string): Promise<Key[]> => {
  return apiClient.get(`/groups/${groupId}/keys`).then((res) => res.data.data);
};

/**
 * 在指定分组下创建一个新的密钥
 * @param groupId 分组ID
 * @param keyData 新密钥的数据
 */
export const createKey = (
  groupId: string,
  keyData: Omit<
    Key,
    | "id"
    | "group_id"
    | "created_at"
    | "updated_at"
    | "request_count"
    | "failure_count"
  >
): Promise<Key> => {
  return apiClient
    .post(`/groups/${groupId}/keys`, keyData)
    .then((res) => res.data.data);
};

/**
 * 更新一个已存在的密钥
 * @param id 密钥ID
 * @param keyData 要更新的数据
 */
export const updateKey = (id: string, keyData: Partial<Key>): Promise<Key> => {
  return apiClient.put(`/keys/${id}`, keyData).then((res) => res.data.data);
};

/**
 * 删除一个密钥
 * @param id 密钥ID
 */
export const deleteKey = (id: string): Promise<void> => {
  return apiClient.delete(`/keys/${id}`).then((res) => res.data);
};

/**
 * 批量更新密钥
 * @param ids 密钥ID列表
 * @param data 要更新的数据
 */
export const batchUpdateKeys = (
  ids: string[],
  data: Partial<Key>
): Promise<void> => {
  return apiClient
    .post("/keys/batch-update", { ids, data })
    .then((res) => res.data);
};

/**
 * 批量删除密钥
 * @param ids 密钥ID列表
 */
export const batchDeleteKeys = (ids: string[]): Promise<void> => {
  return apiClient.post("/keys/batch-delete", { ids }).then((res) => res.data);
};
