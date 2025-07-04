import type { APIKey, Group, GroupStats, TaskInfo } from "@/types/models";

// Mock数据 - 实际开发时应该从后端获取
const mockGroups: Group[] = [
  {
    id: 1,
    name: "openai-main",
    display_name: "OpenAI主组",
    description: "OpenAI主要API组",
    sort: 1,
    channel_type: "openai",
    upstreams: [
      { url: "https://api.openai.com", weight: 1 },
      { url: "https://api.openai.com/v1", weight: 2 },
    ],
    config: {
      test_model: "gpt-3.5-turbo",
      param_overrides: { temperature: 0.7 },
      request_timeout: 30000,
    },
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
  },
  {
    id: 2,
    name: "gemini-backup",
    display_name: "Gemini备用组",
    description: "Gemini备用API组",
    sort: 2,
    channel_type: "gemini",
    upstreams: [{ url: "https://generativelanguage.googleapis.com", weight: 1 }],
    config: {
      test_model: "gemini-pro",
      param_overrides: {},
      request_timeout: 25000,
    },
    created_at: "2024-01-02T00:00:00Z",
    updated_at: "2024-01-02T00:00:00Z",
  },
  {
    id: 3,
    name: "silicon-test",
    display_name: "Silicon测试组",
    description: "Silicon Flow测试API组",
    sort: 3,
    channel_type: "silicon",
    upstreams: [{ url: "https://api.siliconflow.cn", weight: 1 }],
    config: {
      test_model: "qwen-turbo",
      param_overrides: {},
      request_timeout: 20000,
    },
    created_at: "2024-01-03T00:00:00Z",
    updated_at: "2024-01-03T00:00:00Z",
  },
];

const mockAPIKeys: APIKey[] = [
  {
    id: 1,
    group_id: 1,
    key_value: "sk-1234567890abcdef1234567890abcdef",
    status: "active",
    request_count: 1250,
    failure_count: 3,
    last_used_at: "2024-01-01T12:00:00Z",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
  },
  {
    id: 2,
    group_id: 1,
    key_value: "sk-abcdef1234567890abcdef1234567890",
    status: "inactive",
    request_count: 890,
    failure_count: 15,
    last_used_at: "2024-01-01T10:00:00Z",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
  },
  {
    id: 3,
    group_id: 1,
    key_value: "sk-fedcba0987654321fedcba0987654321",
    status: "active",
    request_count: 2100,
    failure_count: 1,
    last_used_at: "2024-01-01T14:00:00Z",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
  },
  {
    id: 4,
    group_id: 1,
    key_value: "gk-1234567890abcdef1234567890abcdef",
    status: "active",
    request_count: 450,
    failure_count: 2,
    last_used_at: "2024-01-02T11:00:00Z",
    created_at: "2024-01-02T00:00:00Z",
    updated_at: "2024-01-02T00:00:00Z",
  },
  {
    id: 5,
    group_id: 1,
    key_value: "sf-1234567890abcdef1234567890abcdef",
    status: "active",
    request_count: 320,
    failure_count: 0,
    last_used_at: "2024-01-03T09:00:00Z",
    created_at: "2024-01-03T00:00:00Z",
    updated_at: "2024-01-03T00:00:00Z",
  },
  {
    id: 6,
    group_id: 1,
    key_value: "sf-1234567890abcdef1234567890abcdef",
    status: "active",
    request_count: 320,
    failure_count: 0,
    last_used_at: "2024-01-03T09:00:00Z",
    created_at: "2024-01-03T00:00:00Z",
    updated_at: "2024-01-03T00:00:00Z",
  },
  {
    id: 7,
    group_id: 1,
    key_value: "sf-1234567890abcdef1234567890abcdef",
    status: "active",
    request_count: 320,
    failure_count: 0,
    last_used_at: "2024-01-03T09:00:00Z",
    created_at: "2024-01-03T00:00:00Z",
    updated_at: "2024-01-03T00:00:00Z",
  },
  {
    id: 5,
    group_id: 1,
    key_value: "sf-1234567890abcdef1234567890abcdef",
    status: "active",
    request_count: 320,
    failure_count: 0,
    last_used_at: "2024-01-03T09:00:00Z",
    created_at: "2024-01-03T00:00:00Z",
    updated_at: "2024-01-03T00:00:00Z",
  },
  {
    id: 8,
    group_id: 1,
    key_value: "sf-1234567890abcdef1234567890abcdef",
    status: "active",
    request_count: 320,
    failure_count: 0,
    last_used_at: "2024-01-03T09:00:00Z",
    created_at: "2024-01-03T00:00:00Z",
    updated_at: "2024-01-03T00:00:00Z",
  },
];

let mockTaskInfo: TaskInfo = {
  is_running: false,
};

export const keysApi = {
  // 获取所有分组
  async getGroups(): Promise<Group[]> {
    // 模拟网络延迟
    await new Promise(resolve => setTimeout(resolve, 300));
    return mockGroups;
  },

  // 获取分组信息
  async getGroup(groupId: number): Promise<Group | null> {
    await new Promise(resolve => setTimeout(resolve, 200));
    return mockGroups.find(g => g.id === groupId) || null;
  },

  // 创建分组
  async createGroup(group: Partial<Group>): Promise<Group> {
    await new Promise(resolve => setTimeout(resolve, 500));
    const newGroup: Group = {
      id: Math.max(...mockGroups.map(g => g.id)) + 1,
      name: group.name || "",
      display_name: group.display_name || "",
      description: group.description || "",
      sort: group.sort || 0,
      channel_type: group.channel_type || "openai",
      upstreams: group.upstreams || [],
      config: group.config || {},
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    mockGroups.push(newGroup);
    return newGroup;
  },

  // 更新分组
  async updateGroup(groupId: number, group: Partial<Group>): Promise<Group> {
    await new Promise(resolve => setTimeout(resolve, 500));
    const index = mockGroups.findIndex(g => g.id === groupId);
    if (index === -1) {
      throw new Error("分组不存在");
    }

    mockGroups[index] = {
      ...mockGroups[index],
      ...group,
      updated_at: new Date().toISOString(),
    };
    return mockGroups[index];
  },

  // 删除分组
  async deleteGroup(groupId: number): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 300));
    const index = mockGroups.findIndex(g => g.id === groupId);
    if (index === -1) {
      throw new Error("分组不存在");
    }

    mockGroups.splice(index, 1);
    // 同时删除该分组的所有密钥
    const keyIndexes = mockAPIKeys
      .map((key, i) => (key.group_id === groupId ? i : -1))
      .filter(i => i !== -1);
    keyIndexes.reverse().forEach(i => mockAPIKeys.splice(i, 1));
  },

  // 获取分组统计信息
  async getGroupStats(groupId: number): Promise<GroupStats> {
    await new Promise(resolve => setTimeout(resolve, 200));
    const keys = mockAPIKeys.filter(k => k.group_id === groupId);
    const activeKeys = keys.filter(k => k.status === "active");

    return {
      total_keys: keys.length,
      active_keys: activeKeys.length,
      requests_1h: Math.floor(Math.random() * 100),
      requests_24h: keys.reduce((sum, key) => sum + key.request_count, 0),
      requests_7d: Math.floor(keys.reduce((sum, key) => sum + key.request_count, 0) * 7.2),
      failure_rate_24h:
        keys.length > 0
          ? (keys.reduce((sum, key) => sum + key.failure_count, 0) /
              keys.reduce((sum, key) => sum + key.request_count, 1)) *
            100
          : 0,
    };
  },

  // 获取分组的密钥列表
  async getGroupKeys(
    groupId: number,
    page = 1,
    size = 10,
    filter?: string
  ): Promise<{
    data: APIKey[];
    total: number;
    page: number;
    size: number;
  }> {
    await new Promise(resolve => setTimeout(resolve, 300));
    let keys = mockAPIKeys.filter(k => k.group_id === groupId);

    if (filter === "valid") {
      keys = keys.filter(k => k.status === "active");
    } else if (filter === "invalid") {
      keys = keys.filter(k => k.status !== "active");
    }

    const start = (page - 1) * size;
    const end = start + size;

    return {
      data: keys.slice(start, end),
      total: keys.length,
      page,
      size,
    };
  },

  // 获取密钥列表（简化方法）
  async getKeys(groupId: number): Promise<APIKey[]> {
    await new Promise(resolve => setTimeout(resolve, 200));
    return mockAPIKeys.filter(k => k.group_id === groupId);
  },

  // 批量添加密钥
  async addMultipleKeys(
    groupId: number,
    keysText: string
  ): Promise<{
    added_count: number;
    ignored_count: number;
    total_in_group: number;
  }> {
    await new Promise(resolve => setTimeout(resolve, 800));

    // 解析密钥文本
    const keys = this.parseKeysText(keysText);
    const existingKeys = mockAPIKeys.filter(k => k.group_id === groupId).map(k => k.key_value);

    let addedCount = 0;
    let ignoredCount = 0;

    keys.forEach(key => {
      if (existingKeys.includes(key)) {
        ignoredCount++;
      } else {
        const newKey: APIKey = {
          id: Math.max(...mockAPIKeys.map(k => k.id)) + 1,
          group_id: groupId,
          key_value: key,
          status: "active",
          request_count: 0,
          failure_count: 0,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        mockAPIKeys.push(newKey);
        addedCount++;
      }
    });

    return {
      added_count: addedCount,
      ignored_count: ignoredCount,
      total_in_group: mockAPIKeys.filter(k => k.group_id === groupId).length,
    };
  },

  // 批量添加密钥（别名方法）
  async batchAddKeys(_keysText: string): Promise<void> {
    // 模拟批量导入，触发全局任务
    mockTaskInfo = {
      is_running: true,
      task_name: "批量导入密钥",
      group_id: 1,
      group_name: "当前分组",
      processed: 0,
      total: 100,
      started_at: new Date().toISOString(),
      message: "正在导入密钥...",
    };

    // 10秒后完成任务
    setTimeout(() => {
      mockTaskInfo = { is_running: false };
    }, 10000);
  },

  // 解析密钥文本
  parseKeysText(text: string): string[] {
    const keys: string[] = [];

    // 尝试解析JSON数组
    try {
      const parsed = JSON.parse(text);
      if (Array.isArray(parsed)) {
        return parsed.filter(key => typeof key === "string" && key.trim().length > 0);
      }
    } catch {
      // 不是JSON，继续其他解析方式
    }

    // 按行分割，然后按常见分隔符分割
    const lines = text.split(/\r?\n/);
    lines.forEach(line => {
      // 按逗号、分号、空格分割
      const parts = line.split(/[,;\s]+/).filter(part => part.trim().length > 0);
      keys.push(...parts);
    });

    return keys.filter(key => key.trim().length > 0);
  },

  // 测试单个密钥
  async testKey(_keyId: number): Promise<{ success: boolean; message: string }> {
    await new Promise(resolve => setTimeout(resolve, 1000));
    // 模拟测试结果
    const success = Math.random() > 0.2; // 80% 成功率
    return {
      success,
      message: success ? "密钥测试成功" : "密钥测试失败：权限不足或密钥无效",
    };
  },

  // 恢复密钥
  async restoreKey(keyId: number): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 300));
    const key = mockAPIKeys.find(k => k.id === keyId);
    if (key) {
      key.status = "active";
      key.updated_at = new Date().toISOString();
    }
  },

  // 删除密钥
  async deleteKey(keyId: number): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 300));
    const index = mockAPIKeys.findIndex(k => k.id === keyId);
    if (index !== -1) {
      mockAPIKeys.splice(index, 1);
    }
  },

  // 恢复所有无效密钥
  async restoreAllInvalidKeys(groupId: number): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 500));
    mockAPIKeys.forEach(key => {
      if (key.group_id === groupId && key.status !== "active") {
        key.status = "active";
        key.updated_at = new Date().toISOString();
      }
    });
  },

  // 清空所有无效密钥
  async clearAllInvalidKeys(groupId: number): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 500));
    for (let i = mockAPIKeys.length - 1; i >= 0; i--) {
      if (mockAPIKeys[i].group_id === groupId && mockAPIKeys[i].status !== "active") {
        mockAPIKeys.splice(i, 1);
      }
    }
  },

  // 导出密钥
  async exportKeys(
    groupId: number,
    filter: "all" | "valid" | "invalid" = "all"
  ): Promise<{ keys: string[] }> {
    await new Promise(resolve => setTimeout(resolve, 300));
    let keys = mockAPIKeys.filter(k => k.group_id === groupId);

    if (filter === "valid") {
      keys = keys.filter(k => k.status === "active");
    } else if (filter === "invalid") {
      keys = keys.filter(k => k.status !== "active");
    }

    return {
      keys: keys.map(k => k.key_value),
    };
  },

  // 验证分组密钥
  async validateGroupKeys(groupId: number): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 500));

    if (mockTaskInfo.is_running) {
      throw new Error("已有验证任务正在运行");
    }

    const group = mockGroups.find(g => g.id === groupId);
    const keys = mockAPIKeys.filter(k => k.group_id === groupId);

    mockTaskInfo = {
      is_running: true,
      task_name: "key_validation",
      group_id: groupId,
      group_name: group?.display_name || group?.name || "",
      processed: 0,
      total: keys.length,
      started_at: new Date().toISOString(),
    };

    // 模拟异步验证过程
    setTimeout(() => {
      mockTaskInfo = { is_running: false };
    }, 10000); // 10秒后完成
  },

  // 获取任务状态
  async getTaskStatus(): Promise<TaskInfo> {
    await new Promise(resolve => setTimeout(resolve, 100));

    if (
      mockTaskInfo.is_running &&
      mockTaskInfo.processed !== undefined &&
      mockTaskInfo.total !== undefined
    ) {
      // 模拟进度更新
      if (mockTaskInfo.processed < mockTaskInfo.total) {
        mockTaskInfo.processed = Math.min(mockTaskInfo.processed + 1, mockTaskInfo.total);
      }
    }

    return mockTaskInfo;
  },

  // 批量切换密钥状态
  async batchToggleKeys(keyIds: string[], status: 0 | 1): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 500));

    // 模拟批量操作
    const numKeys = keyIds.length;
    console.warn(`Mock: 批量${status === 1 ? "启用" : "禁用"}${numKeys}个密钥`, keyIds);
  },

  // 批量删除密钥
  async batchDeleteKeys(keyIds: string[]): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 800));

    // 模拟批量删除
    const numKeys = keyIds.length;
    console.warn(`Mock: 批量删除${numKeys}个密钥`, keyIds);
  },

  // 切换单个密钥状态
  async toggleKeyStatus(keyId: string, status: 0 | 1): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 300));

    console.warn(`Mock: ${status === 1 ? "启用" : "禁用"}密钥 ${keyId}`);
  },

  // 删除单个密钥
  async deleteKeyById(keyId: string): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, 400));

    console.warn(`Mock: 删除密钥 ${keyId}`);
  },

  // 验证密钥
  async validateKeys(groupId: number): Promise<{ valid_count: number; invalid_count: number }> {
    await new Promise(resolve => setTimeout(resolve, 2000));

    // 模拟验证结果
    const validCount = Math.floor(Math.random() * 10) + 5;
    const invalidCount = Math.floor(Math.random() * 3);

    console.warn(`Mock: 验证分组${groupId}的密钥，有效：${validCount}，无效：${invalidCount}`);

    return {
      valid_count: validCount,
      invalid_count: invalidCount,
    };
  },
};
