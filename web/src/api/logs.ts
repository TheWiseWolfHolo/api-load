import type { Group, LogFilter, LogsResponse } from "@/types/models";
import http from "@/utils/http";

export const logApi = {
  // 获取日志列表
  getLogs: (params: LogFilter): Promise<LogsResponse> => {
    return http.get("/logs", { params });
  },

  // 获取分组列表（用于筛选）
  getGroups: (): Promise<Group[]> => {
    return http.get("/groups");
  },
};
