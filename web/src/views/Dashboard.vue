<template>
  <div class="dashboard-page">
    <h1>仪表盘</h1>
    <div v-if="loading">加载中...</div>
    <div v-if="stats" class="stats-grid">
      <el-card>
        <el-statistic title="总请求数" :value="stats.total_requests" />
      </el-card>
      <el-card>
        <el-statistic title="成功请求数" :value="stats.success_requests" />
      </el-card>
      <el-card>
        <el-statistic title="成功率" :value="stats.success_rate" :formatter="rateFormatter" />
      </el-card>
    </div>
    <el-card v-if="stats && stats.group_stats.length > 0" class="chart-card">
      <StatsChart :data="stats.group_stats" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';
import { storeToRefs } from 'pinia';
import { useDashboardStore } from '@/stores/dashboardStore';
import StatsChart from '@/components/StatsChart.vue';

const dashboardStore = useDashboardStore();
const { stats, loading } = storeToRefs(dashboardStore);

onMounted(() => {
  dashboardStore.fetchStats();
});

const rateFormatter = (rate: number) => {
  return `${(rate * 100).toFixed(2)}%`;
};
</script>

<style scoped>
.dashboard-page {
  padding: 20px;
}
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 20px;
  margin-bottom: 20px;
}
.chart-card {
  margin-top: 20px;
}
</style>