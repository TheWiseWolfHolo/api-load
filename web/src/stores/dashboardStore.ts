import { defineStore } from 'pinia';
import { ref } from 'vue';
import { getDashboardStats } from '@/api/dashboard';
import type { DashboardStats } from '@/types/models';

export const useDashboardStore = defineStore('dashboard', () => {
  const stats = ref<DashboardStats | null>(null);
  const loading = ref(false);

  const fetchStats = async () => {
    loading.value = true;
    try {
      const response = await getDashboardStats();
      stats.value = response;
    } catch (error) {
      console.error('Failed to fetch dashboard stats:', error);
    } finally {
      loading.value = false;
    }
  };

  return {
    stats,
    loading,
    fetchStats,
  };
});