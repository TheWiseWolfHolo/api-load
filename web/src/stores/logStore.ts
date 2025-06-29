import { defineStore } from 'pinia';
import { ref, reactive } from 'vue';
import { getLogs } from '@/api/logs';
import type { RequestLog, LogQuery, PaginatedLogs } from '@/api/logs';

export const useLogStore = defineStore('logs', () => {
  const logs = ref<RequestLog[]>([]);
  const loading = ref(false);
  const pagination = reactive({
    page: 1,
    size: 10,
    total: 0,
  });
  const filters = reactive<LogQuery>({});

  const fetchLogs = async () => {
    loading.value = true;
    try {
      const query: LogQuery = {
        ...filters,
        page: pagination.page,
        size: pagination.size,
      };
      const response: PaginatedLogs = await getLogs(query);
      logs.value = response.data;
      pagination.total = response.total;
    } catch (error) {
      console.error('Failed to fetch logs:', error);
    } finally {
      loading.value = false;
    }
  };

  const setFilters = (newFilters: LogQuery) => {
    Object.assign(filters, newFilters);
    pagination.page = 1;
    fetchLogs();
  };

  const setPage = (page: number) => {
    pagination.page = page;
    fetchLogs();
  };

  const setSize = (size: number) => {
    pagination.size = size;
    pagination.page = 1;
    fetchLogs();
  };

  return {
    logs,
    loading,
    pagination,
    filters,
    fetchLogs,
    setFilters,
    setPage,
    setSize,
  };
});