<template>
  <div class="dashboard-container">
    <div class="dashboard-header">
      <h1>仪表盘</h1>
      <p>查看您账户的总体使用情况和统计数据。</p>
    </div>

    <LoadingSpinner v-if="loading && !stats.total_keys" />

    <div v-else class="dashboard-content">
      <!-- 统计卡片 -->
      <StatsCards />

      <!-- 快捷操作和筛选 -->
      <div class="dashboard-grid">
        <div class="quick-actions-section">
          <QuickActions />
        </div>
        <div class="filters-section">
          <el-card shadow="never">
            <template #header>
              <h3>筛选图表</h3>
            </template>
            <div class="filter-controls">
              <!-- 时间范围筛选 -->
              <el-select
                v-model="filters.timeRange"
                @change="onFilterChange"
                placeholder="选择时间范围"
                style="width: 200px"
              >
                <el-option label="过去 24 小时" value="24h" />
                <el-option label="过去 7 天" value="7d" />
                <el-option label="过去 30 天" value="30d" />
              </el-select>
              <!-- 分组筛选 -->
              <el-select
                v-model="filters.groupId"
                @change="onFilterChange"
                placeholder="选择分组"
                style="width: 200px"
                clearable
              >
                <el-option
                  v-for="group in groupStore.groups"
                  :key="group.id"
                  :label="group.name"
                  :value="group.id"
                />
              </el-select>
            </div>
          </el-card>
        </div>
      </div>

      <!-- 请求统计图表 -->
      <RequestChart />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { storeToRefs } from "pinia";
import { useDashboardStore } from "@/stores/dashboardStore";
import { useGroupStore } from "@/stores/groupStore";
import StatsCards from "@/components/business/dashboard/StatsCards.vue";
import RequestChart from "@/components/business/dashboard/RequestChart.vue";
import QuickActions from "@/components/business/dashboard/QuickActions.vue";
import LoadingSpinner from "@/components/common/LoadingSpinner.vue";

const dashboardStore = useDashboardStore();
const groupStore = useGroupStore();

const { stats, loading, filters } = storeToRefs(dashboardStore);

const onFilterChange = () => {
  dashboardStore.fetchDashboardData();
};

onMounted(() => {
  dashboardStore.startPolling();
  groupStore.fetchGroups(); // 获取分组列表用于筛选
});

onUnmounted(() => {
  dashboardStore.stopPolling();
});
</script>

<style scoped>
.dashboard-container {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.dashboard-header {
  margin-bottom: 24px;
}

.dashboard-header h1 {
  font-size: 28px;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 8px;
}

.dashboard-header p {
  color: #6b7280;
  font-size: 14px;
}

.dashboard-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 24px;
  align-items: start;
}

.quick-actions-section {
  min-height: 200px;
}

.filters-section h3 {
  font-size: 18px;
  font-weight: 500;
  color: #1f2937;
  margin: 0;
}

.filter-controls {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

@media (max-width: 1024px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .dashboard-container {
    padding: 16px;
  }

  .filter-controls {
    flex-direction: column;
  }

  .filter-controls .el-select {
    width: 100% !important;
  }
}
</style>
