<script setup lang="ts">
import { onMounted, ref } from "vue";

// 模拟图表数据
const chartData = ref({
  labels: ["00:00", "04:00", "08:00", "12:00", "16:00", "20:00", "24:00"],
  datasets: [
    {
      label: "请求数量",
      data: [120, 150, 300, 450, 380, 280, 200],
      color: "#667eea",
    },
    {
      label: "响应时间",
      data: [200, 180, 250, 300, 220, 190, 160],
      color: "#f093fb",
    },
  ],
});

const chartContainer = ref<HTMLElement>();
const animationProgress = ref(0);

// 生成SVG路径
const generatePath = (data: number[]) => {
  const points = data.map((value, index) => {
    const x = (index / (data.length - 1)) * 380 + 10;
    const y = 200 - (value / 500) * 180 - 10;
    return `${x},${y}`;
  });
  return `M ${points.join(" L ")}`;
};

onMounted(() => {
  // 简单的动画效果
  let start = 0;
  const animate = (timestamp: number) => {
    if (!start) {
      start = timestamp;
    }
    const progress = Math.min((timestamp - start) / 2000, 1);
    animationProgress.value = progress;

    if (progress < 1) {
      requestAnimationFrame(animate);
    }
  };
  requestAnimationFrame(animate);
});
</script>

<template>
  <div class="chart-container">
    <n-card class="chart-card modern-card" :bordered="false">
      <template #header>
        <div class="chart-header">
          <h3 class="chart-title">性能监控</h3>
          <p class="chart-subtitle">实时系统性能指标</p>
        </div>
      </template>

      <div ref="chartContainer" class="chart-content">
        <div class="chart-legend">
          <div v-for="dataset in chartData.datasets" :key="dataset.label" class="legend-item">
            <div class="legend-color" :style="{ backgroundColor: dataset.color }" />
            <span class="legend-label">{{ dataset.label }}</span>
          </div>
        </div>

        <div class="chart-area">
          <div class="chart-grid">
            <div
              v-for="(label, index) in chartData.labels"
              :key="label"
              class="grid-line"
              :style="{ left: `${(index / (chartData.labels.length - 1)) * 100}%` }"
            />
          </div>

          <svg class="chart-svg" viewBox="0 0 400 200">
            <!-- 数据线条 -->
            <g v-for="dataset in chartData.datasets" :key="dataset.label">
              <path
                :d="generatePath(dataset.data)"
                :stroke="dataset.color"
                stroke-width="3"
                fill="none"
                stroke-linecap="round"
                stroke-linejoin="round"
                class="chart-line"
                :style="{
                  strokeDasharray: '1000',
                  strokeDashoffset: `${1000 * (1 - animationProgress)}`,
                }"
              />

              <!-- 数据点 -->
              <g v-for="(value, index) in dataset.data" :key="index">
                <circle
                  :cx="(index / (dataset.data.length - 1)) * 380 + 10"
                  :cy="200 - (value / 500) * 180 - 10"
                  :r="animationProgress > index / dataset.data.length ? 4 : 0"
                  :fill="dataset.color"
                  class="chart-point"
                />
              </g>
            </g>
          </svg>

          <div class="chart-labels">
            <div
              v-for="(label, index) in chartData.labels"
              :key="label"
              class="chart-label"
              :style="{ left: `${(index / (chartData.labels.length - 1)) * 100}%` }"
            >
              {{ label }}
            </div>
          </div>
        </div>
      </div>
    </n-card>
  </div>
</template>

<style scoped>
.chart-container {
  width: 100%;
}

.chart-card {
  background: rgba(255, 255, 255, 0.98);
}

.chart-header {
  text-align: center;
  margin-bottom: 8px;
}

.chart-title {
  font-size: 1.3rem;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 4px 0;
}

.chart-subtitle {
  font-size: 0.9rem;
  color: #64748b;
  margin: 0;
}

.chart-content {
  position: relative;
}

.chart-legend {
  display: flex;
  justify-content: center;
  gap: 24px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.legend-color {
  width: 16px;
  height: 16px;
  border-radius: 4px;
  box-shadow: var(--shadow-sm);
}

.legend-label {
  font-size: 0.9rem;
  color: #374151;
  font-weight: 500;
}

.chart-area {
  position: relative;
  height: 240px;
  background: linear-gradient(180deg, rgba(102, 126, 234, 0.02) 0%, rgba(102, 126, 234, 0.08) 100%);
  border-radius: var(--border-radius-md);
  border: 1px solid rgba(102, 126, 234, 0.1);
  overflow: hidden;
}

.chart-grid {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
}

.grid-line {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: rgba(102, 126, 234, 0.1);
}

.chart-svg {
  width: 100%;
  height: 200px;
  position: relative;
  z-index: 1;
}

.chart-line {
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.1));
  transition: stroke-dashoffset 0.2s ease-out;
}

.chart-point {
  transition: r 0.2s ease;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.2));
}

.chart-point:hover {
  r: 6;
}

.chart-labels {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 40px;
  display: flex;
  align-items: center;
}

.chart-label {
  position: absolute;
  font-size: 0.8rem;
  color: #64748b;
  font-weight: 500;
  transform: translateX(-50%);
  background: rgba(255, 255, 255, 0.9);
  padding: 4px 8px;
  border-radius: 4px;
  backdrop-filter: blur(4px);
}
</style>
