<script setup lang="ts">
import { NCard, NGrid, NGridItem, NSpace, NTag } from "naive-ui";
import { onMounted, ref } from "vue";

// 模拟数据
const stats = ref([
  {
    title: "总请求数",
    value: "125,842",
    icon: "📈",
    color: "var(--primary-gradient)",
    trend: "+12.5%",
    trendUp: true,
  },
  {
    title: "活跃连接",
    value: "1,234",
    icon: "🔗",
    color: "var(--success-gradient)",
    trend: "+5.2%",
    trendUp: true,
  },
  {
    title: "响应时间",
    value: "245ms",
    icon: "⚡",
    color: "var(--warning-gradient)",
    trend: "-8.1%",
    trendUp: false,
  },
  {
    title: "错误率",
    value: "0.12%",
    icon: "🛡️",
    color: "var(--secondary-gradient)",
    trend: "-2.3%",
    trendUp: false,
  },
]);

const animatedValues = ref<Record<string, number>>({});

onMounted(() => {
  // 动画效果
  stats.value.forEach((stat, index) => {
    setTimeout(() => {
      animatedValues.value[stat.title] = 1;
    }, index * 150);
  });
});
</script>

<template>
  <div class="stats-container">
    <n-space vertical size="medium">
      <n-grid :cols="4" :x-gap="20" :y-gap="20" responsive="screen">
        <n-grid-item v-for="(stat, index) in stats" :key="stat.title" span="1">
          <n-card
            :bordered="false"
            class="stat-card"
            :style="{ animationDelay: `${index * 0.07}s` }"
          >
            <div class="stat-header">
              <div class="stat-icon" :style="{ background: stat.color }">
                {{ stat.icon }}
              </div>
              <n-tag :type="stat.trendUp ? 'success' : 'error'" size="small" class="stat-trend">
                {{ stat.trend }}
              </n-tag>
            </div>

            <div class="stat-content">
              <div class="stat-value">{{ stat.value }}</div>
              <div class="stat-title">{{ stat.title }}</div>
            </div>

            <div class="stat-bar">
              <div
                class="stat-bar-fill"
                :style="{
                  background: stat.color,
                  width: `${animatedValues[stat.title] * 100}%`,
                }"
              />
            </div>
          </n-card>
        </n-grid-item>
      </n-grid>
    </n-space>
  </div>
</template>

<style scoped>
.stats-container {
  width: 100%;
  animation: fadeInUp 0.2s ease-out;
}

.stat-card {
  background: rgba(255, 255, 255, 0.98);
  border-radius: var(--border-radius-lg);
  border: 1px solid rgba(255, 255, 255, 0.3);
  position: relative;
  overflow: hidden;
  animation: slideInUp 0.2s ease-out both;
  transition: all 0.2s ease;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg);
}

.stat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: var(--border-radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.5rem;
  color: white;
  box-shadow: var(--shadow-md);
}

.stat-trend {
  font-weight: 600;
}

.stat-trend:before {
  content: "";
  display: inline-block;
  width: 0;
  height: 0;
  margin-right: 4px;
  vertical-align: middle;
}

.stat-content {
  margin-bottom: 16px;
}

.stat-value {
  font-size: 2.5rem;
  font-weight: 700;
  line-height: 1.2;
  color: #1e293b;
  margin-bottom: 4px;
}

.stat-title {
  font-size: 0.95rem;
  color: #64748b;
  font-weight: 500;
}

.stat-bar {
  width: 100%;
  height: 4px;
  background: rgba(0, 0, 0, 0.05);
  border-radius: 2px;
  overflow: hidden;
  position: relative;
}

.stat-bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 1s ease-out;
  transition-delay: 0.2s;
}

@keyframes slideInUp {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
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

/* 响应式网格 */
:deep(.n-grid-item) {
  min-width: 0;
}

@media (max-width: 1200px) {
  :deep(.n-grid) {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  :deep(.n-grid) {
    grid-template-columns: 1fr;
  }

  .stat-value {
    font-size: 2rem;
  }
}
</style>
