import i18n from "@/locales";
import type { ApiResponse, Group, LogFilter, LogsResponse } from "@/types/models";
import { downloadAuthenticatedFile } from "@/utils/download";
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

  // 导出日志
  exportLogs: (params: Omit<LogFilter, "page" | "page_size">) => {
    const authKey = localStorage.getItem("authKey");
    if (!authKey) {
      window.$message.error(i18n.global.t("auth.noAuthKeyFound"));
      return;
    }

    const queryParams = Object.entries(params).reduce(
      (acc, [key, value]) => {
        if (value !== undefined && value !== null && value !== "") {
          acc[key] = String(value);
        }
        return acc;
      },
      {} as Record<string, string>
    );

    downloadAuthenticatedFile("/logs/export", `logs-${Date.now()}.csv`, queryParams);
  },
};
