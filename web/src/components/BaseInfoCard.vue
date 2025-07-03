<script setup lang="ts">
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
    <div class="stats-grid">
      <div
        v-for="(stat, index) in stats"
        :key="stat.title"
        class="stat-card modern-card"
        :style="{ animationDelay: `${index * 0.1}s` }"
      >
        <div class="stat-header">
          <div class="stat-icon" :style="{ background: stat.color }">
            {{ stat.icon }}
          </div>
          <div
            class="stat-trend"
            :class="{ 'trend-up': stat.trendUp, 'trend-down': !stat.trendUp }"
          >
            {{ stat.trend }}
          </div>
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
      </div>
    </div>
  </div>
</template>

<style scoped>
.stats-container {
  width: 100%;
  animation: fadeInUp 0.6s ease-out;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
}

.stat-card {
  padding: 24px;
  background: rgba(255, 255, 255, 0.98);
  border-radius: var(--border-radius-lg);
  border: 1px solid rgba(255, 255, 255, 0.3);
  position: relative;
  overflow: hidden;
  animation: slideInUp 0.6s ease-out both;
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
  font-size: 0.875rem;
  font-weight: 600;
  padding: 4px 8px;
  border-radius: 6px;
  display: flex;
  align-items: center;
}

.trend-up {
  background: rgba(34, 197, 94, 0.1);
  color: #16a34a;
}

.trend-down {
  background: rgba(239, 68, 68, 0.1);
  color: #dc2626;
}

.trend-up::before {
  content: "↗";
  margin-right: 4px;
}

.trend-down::before {
  content: "↘";
  margin-right: 4px;
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
  transition-delay: 0.3s;
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

@media (max-width: 640px) {
  .stats-grid {
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .stat-card {
    padding: 20px;
  }

  .stat-value {
    font-size: 2rem;
  }
}
</style>
