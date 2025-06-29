<template>
  <div class="logs-page">
    <h1>日志查询</h1>
    <LogFilter />
    <el-table :data="logs" v-loading="loading" style="width: 100%">
      <el-table-column prop="timestamp" label="时间" width="180" :formatter="formatDate" />
      <el-table-column prop="group_id" label="分组ID" width="100" />
      <el-table-column prop="key_id" label="密钥ID" width="100" />
      <el-table-column prop="source_ip" label="源IP" width="150" />
      <el-table-column prop="status_code" label="状态码" width="100" />
      <el-table-column prop="request_path" label="请求路径" />
      <el-table-column prop="request_body_snippet" label="请求体片段" />
    </el-table>
    <el-pagination
      background
      layout="prev, pager, next, sizes"
      :total="pagination.total"
      :page-size="pagination.size"
      :current-page="pagination.page"
      @current-change="handlePageChange"
      @size-change="handleSizeChange"
      class="pagination-container"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { storeToRefs } from 'pinia';
import { useLogStore } from '@/stores/logStore';
import LogFilter from '@/components/LogFilter.vue';
import type { RequestLog } from '@/types/models';

const logStore = useLogStore();
const { logs, loading, pagination } = storeToRefs(logStore);

onMounted(() => {
  logStore.fetchLogs();
});

const handlePageChange = (page: number) => {
  logStore.setPage(page);
};

const handleSizeChange = (size: number) => {
  logStore.setSize(size);
};

const formatDate = (_row: RequestLog, _column: any, cellValue: string) => {
  return new Date(cellValue).toLocaleString();
};
</script>

<style scoped>
.logs-page {
  padding: 20px;
}
.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>