<script setup lang="ts">
import BaseInfoCard from "@/components/BaseInfoCard.vue";
import LineChart from "@/components/LineChart.vue";
</script>

<template>
  <div class="dashboard-container">
    <div class="dashboard-header">
      <h2 class="dashboard-title">仪表盘</h2>
      <p class="dashboard-subtitle">系统概览与实时监控</p>
    </div>

    <div class="dashboard-content">
      <base-info-card />
      <line-chart class="dashboard-chart" />
    </div>
  </div>
</template>

<style scoped>
.dashboard-container {
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
}

.dashboard-header {
  margin-bottom: 32px;
  text-align: center;
}

.dashboard-title {
  font-size: 2.25rem;
  font-weight: 700;
  background: var(--primary-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0 0 8px 0;
  letter-spacing: -0.5px;
}

.dashboard-subtitle {
  font-size: 1.1rem;
  color: #64748b;
  margin: 0;
  font-weight: 500;
}

.dashboard-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.dashboard-chart {
  animation: fadeInUp 0.6s ease-out 0.2s both;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 768px) {
  .dashboard-title {
    font-size: 1.75rem;
  }

  .dashboard-subtitle {
    font-size: 1rem;
  }

  .dashboard-content {
    gap: 16px;
  }
}
</style>
