import apiClient from "./index";
import type { Group } from "../types/models";

/**
 * 获取所有分组列表
 */
export const fetchGroups = (): Promise<Group[]> => {
  return apiClient.get("/groups").then((res) => {
    const groups = res.data.data;
    // 将后端返回的 config 字符串解析为对象
    return groups.map((group: any) => ({
      ...group,
      config:
        typeof group.config === "string"
          ? JSON.parse(group.config)
          : group.config,
    }));
  });
};

/**
 * 获取单个分组的详细信息
 * @param id 分组ID
 */
export const fetchGroup = (id: string): Promise<Group> => {
  return apiClient.get(`/groups/${id}`).then((res) => {
    const group = res.data.data;
    // 将后端返回的 config 字符串解析为对象
    return {
      ...group,
      config:
        typeof group.config === "string"
          ? JSON.parse(group.config)
          : group.config,
    };
  });
};

/**
 * 创建一个新的分组
 * @param groupData 新分组的数据
 */
export const createGroup = (
  groupData: Omit<Group, "id" | "created_at" | "updated_at" | "api_keys">
): Promise<Group> => {
  // 将 config 对象转换为 JSON 字符串，匹配后端期望的格式
  const requestData = {
    ...groupData,
    config:
      typeof groupData.config === "object"
        ? JSON.stringify(groupData.config)
        : groupData.config,
  };

  console.log("createGroup - Original data:", groupData);
  console.log("createGroup - Request data:", requestData);
  console.log("createGroup - Config type:", typeof requestData.config);

  return apiClient.post("/groups", requestData).then((res) => res.data.data);
};

/**
 * 更新一个已存在的分组
 * @param id 分组ID
 * @param groupData 要更新的数据
 */
export const updateGroup = (
  id: string,
  groupData: Partial<
    Omit<Group, "id" | "created_at" | "updated_at" | "api_keys">
  >
): Promise<Group> => {
  // 将 config 对象转换为 JSON 字符串，匹配后端期望的格式
  const requestData = {
    ...groupData,
    config:
      groupData.config && typeof groupData.config === "object"
        ? JSON.stringify(groupData.config)
        : groupData.config,
  };
  return apiClient
    .put(`/groups/${id}`, requestData)
    .then((res) => res.data.data);
};

/**
 * 删除一个分组
 * @param id 分组ID
 */
export const deleteGroup = (id: string): Promise<void> => {
  return apiClient.delete(`/groups/${id}`).then((res) => res.data);
};
