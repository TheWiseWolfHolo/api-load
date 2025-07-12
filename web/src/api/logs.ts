import type { ApiResponse, Group, LogFilter, LogsResponse } from "@/types/models";
import http from "@/utils/http";

export const logApi = {
  // 获取日志列表
  getLogs: (params: LogFilter): Promise<ApiResponse<LogsResponse>> => {
    return http.get("/logs", { params });
  },

  // 获取分组列表（用于筛选）
  getGroups: (): Promise<ApiResponse<Group[]>> => {
    return http.get("/groups");
  },
};
