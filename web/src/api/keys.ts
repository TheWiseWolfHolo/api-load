import type { APIKey, Group, GroupConfigOption, TaskInfo } from "@/types/models";
import http from "@/utils/http";

export const keysApi = {
  // 获取所有分组
  async getGroups(): Promise<Group[]> {
    const res = await http.get("/groups");
    return res.data || [];
  },

  // 创建分组
  async createGroup(group: Partial<Group>): Promise<Group> {
    const res = await http.post("/groups", group);
    return res.data;
  },

  // 更新分组
  async updateGroup(groupId: number, group: Partial<Group>): Promise<Group> {
    const res = await http.put(`/groups/${groupId}`, group);
    return res.data;
  },

  // 删除分组
  deleteGroup(groupId: number): Promise<void> {
    return http.delete(`/groups/${groupId}`);
  },

  // 获取分组统计信息
  async getGroupStats(groupId: number): Promise<GroupStats> {
    await new Promise(resolve => setTimeout(resolve, 200));
    return {};
  },

  // 获取分组可配置参数
  async getGroupConfigOptions(): Promise<GroupConfigOption[]> {
    const res = await http.get("/groups/config-options");
    return res.data || [];
  },

  // 获取分组的密钥列表
  async getGroupKeys(params: {
    group_id: number;
    page: number;
    page_size: number;
    key?: string;
    status?: "active" | "inactive";
  }): Promise<{
    items: APIKey[];
    pagination: {
      total_items: number;
    };
  }> {
    const res = await http.get("/keys", { params });
    return res.data;
  },

  // 批量添加密钥
  async addMultipleKeys(
    group_id: number,
    keys_text: string
  ): Promise<{
    added_count: number;
    ignored_count: number;
    total_in_group: number;
  }> {
    const res = await http.post("/keys/add-multiple", {
      group_id,
      keys_text,
    });
    return res.data;
  },

  // 测试单个密钥
  async testKeys(
    group_id: number,
    keys_text: string
  ): Promise<
    {
      key_value: string;
      is_valid: boolean;
      error: string;
    }[]
  > {
    const res = await http.post("/keys/test-multiple", {
      group_id,
      keys_text,
    });
    return res.data;
  },

  // 删除密钥
  deleteKeys(group_id: number, keys_text: string): Promise<void> {
    return http.post("/keys/delete-multiple", {
      group_id,
      keys_text,
    });
  },

  // 恢复所有无效密钥
  restoreAllInvalidKeys(group_id: number): Promise<void> {
    return http.post("/keys/restore-all-invalid", { group_id });
  },

  // 清空所有无效密钥
  clearAllInvalidKeys(group_id: number): Promise<void> {
    return http.post("/keys/clear-all-invalid", { group_id });
  },

  // 导出密钥
  async exportKeys(
    groupId: number,
    filter: "all" | "valid" | "invalid" = "all"
  ): Promise<{ keys: string[] }> {
    const params: any = { filter };
    const res = await http.get(`/groups/${groupId}/keys/export`, { params });
    return res.data;
  },

  // 验证分组密钥
  async validateGroupKeys(groupId: number): Promise<{
    is_running: boolean;
    group_name: string;
    processed: number;
    total: number;
    started_at: string;
  }> {
    const res = await http.post("/keys/validate-group", { group_id: groupId });
    return res.data;
  },

  // 获取任务状态
  async getTaskStatus(): Promise<TaskInfo> {
    const res = await http.get("/tasks/status");
    return res.data;
  },
};
