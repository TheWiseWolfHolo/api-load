import type { ChartData, DashboardStatsResponse, Group, TokenStatsResponse } from "@/types/models";
import http from "@/utils/http";

/**
 * 获取仪表盘基础统计数据
 */
export const getDashboardStats = () => {
  return http.get<DashboardStatsResponse>("/dashboard/stats");
};

/**
 * 获取仪表盘图表数据
 * @param groupId 可选的分组ID
 */
export const getDashboardChart = (groupId?: number) => {
  return http.get<ChartData>("/dashboard/chart", {
    params: groupId ? { groupId } : {},
  });
};

export const getTokenStats = (params?: {
  group_by?: "model" | "group" | "hour";
  start_time?: string;
  end_time?: string;
}) => {
  return http.get<TokenStatsResponse>("/dashboard/token-stats", { params });
};

/**
 * 获取用于筛选的分组列表
 */
export const getGroupList = () => {
  return http.get<Group[]>("/groups/list");
};
