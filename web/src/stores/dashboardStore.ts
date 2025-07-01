import { ref, computed } from "vue";
import { defineStore } from "pinia";
import { getDashboardData } from "@/api/dashboard";
import type { DashboardStats } from "@/types/models";

export const useDashboardStore = defineStore("dashboard", () => {
  const stats = ref<DashboardStats>({
    total_requests: 0,
    success_requests: 0,
    success_rate: 0,
    group_stats: [],
    // 前端扩展字段
    total_keys: 0,
    active_keys: 0,
    inactive_keys: 0,
    error_keys: 0,
  });
  const loading = ref(false);
  const filters = ref({
    timeRange: "7d",
    groupId: null as number | null,
  });

  let pollingInterval: number | undefined;

  const chartData = computed(() => {
    // 基于group_stats生成图表数据
    return {
      labels: stats.value.group_stats.map((g) => g.group_name),
      data: stats.value.group_stats.map((g) => g.request_count),
    };
  });

  const fetchDashboardData = async () => {
    loading.value = true;
    try {
      const response = await getDashboardData(
        filters.value.timeRange,
        filters.value.groupId
      );
      stats.value = response;
    } catch (error) {
      console.error("Failed to fetch dashboard data:", error);
    } finally {
      loading.value = false;
    }
  };

  const startPolling = () => {
    fetchDashboardData();
    pollingInterval = window.setInterval(fetchDashboardData, 30000); // 30 seconds
  };

  const stopPolling = () => {
    if (pollingInterval) {
      clearInterval(pollingInterval);
      pollingInterval = undefined;
    }
  };

  return {
    stats,
    loading,
    filters,
    chartData,
    fetchDashboardData,
    startPolling,
    stopPolling,
  };
});
