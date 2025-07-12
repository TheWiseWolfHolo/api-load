<script setup lang="ts">
import { getDashboardChart, getGroupList } from "@/api/dashboard";
import type { ChartData, ChartDataset } from "@/types/models";
import { NSelect, NSpin } from "naive-ui";
import { computed, onMounted, ref, watch } from "vue";

// 图表数据
const chartData = ref<ChartData | null>(null);
const selectedGroup = ref<number | null>(null);
const loading = ref(true);
const animationProgress = ref(0);
const hoveredPoint = ref<{
  datasetIndex: number;
  pointIndex: number;
  x: number;
  y: number;
} | null>(null);
const tooltipData = ref<{
  time: string;
  label: string;
  value: number;
  color: string;
} | null>(null);
const tooltipPosition = ref({ x: 0, y: 0 });
const chartSvg = ref<SVGElement>();

// 图表尺寸和边距
const chartWidth = 800;
const chartHeight = 400;
const padding = { top: 40, right: 40, bottom: 60, left: 80 };

// 格式化分组选项
const groupOptions = ref<Array<{ label: string; value: number | null }>>([]);

// 计算有效的绘图区域
const plotWidth = chartWidth - padding.left - padding.right;
const plotHeight = chartHeight - padding.top - padding.bottom;

// 计算数据的最大值和最小值
const dataRange = computed(() => {
  if (!chartData.value) {
    return { min: 0, max: 100 };
  }

  const allValues = chartData.value.datasets.flatMap(d => d.data);
  const max = Math.max(...allValues, 0);
  const min = Math.min(...allValues, 0);

  // 添加一些padding让图表更好看
  const paddingValue = (max - min) * 0.1;
  return {
    min: Math.max(0, min - paddingValue),
    max: max + paddingValue,
  };
});

// 生成Y轴刻度
const yTicks = computed(() => {
  const { min, max } = dataRange.value;
  const range = max - min;
  const tickCount = 5;
  const step = range / (tickCount - 1);

  return Array.from({ length: tickCount }, (_, i) => min + i * step);
});

// 生成可见的X轴标签（避免重叠）
const visibleLabels = computed(() => {
  if (!chartData.value) {
    return [];
  }

  const labels = chartData.value.labels;
  const maxLabels = 8; // 最多显示8个标签
  const step = Math.ceil(labels.length / maxLabels);

  return labels.map((label, index) => ({ text: label, index })).filter((_, i) => i % step === 0);
});

// 位置计算函数
const getXPosition = (index: number) => {
  if (!chartData.value) {
    return 0;
  }
  const totalPoints = chartData.value.labels.length;
  return padding.left + (index / (totalPoints - 1)) * plotWidth;
};

const getYPosition = (value: number) => {
  const { min, max } = dataRange.value;
  const ratio = (value - min) / (max - min);
  return padding.top + (1 - ratio) * plotHeight;
};

// 生成线条路径
const generateLinePath = (data: number[]) => {
  if (!data.length) {
    return "";
  }

  const points = data.map((value, index) => {
    const x = getXPosition(index);
    const y = getYPosition(value);
    return `${x},${y}`;
  });

  return `M ${points.join(" L ")}`;
};

// 生成填充区域路径
const generateAreaPath = (data: number[]) => {
  if (!data.length) {
    return "";
  }

  const points = data.map((value, index) => {
    const x = getXPosition(index);
    const y = getYPosition(value);
    return `${x},${y}`;
  });

  const baseY = getYPosition(dataRange.value.min);
  const firstX = getXPosition(0);
  const lastX = getXPosition(data.length - 1);

  return `M ${firstX},${baseY} L ${points.join(" L ")} L ${lastX},${baseY} Z`;
};

// 数字格式化
const formatNumber = (value: number) => {
  if (value >= 1000000) {
    return `${(value / 1000000).toFixed(1)}M`;
  } else if (value >= 1000) {
    return `${(value / 1000).toFixed(1)}K`;
  }
  return Math.round(value).toString();
};

// 动画相关
const animatedStroke = ref("0");
const animatedOffset = ref("0");

const startAnimation = () => {
  if (!chartData.value) {
    return;
  }

  // 计算总路径长度（近似）
  const totalLength = plotWidth + plotHeight;
  animatedStroke.value = `${totalLength}`;
  animatedOffset.value = `${totalLength}`;

  let start = 0;
  const animate = (timestamp: number) => {
    if (!start) {
      start = timestamp;
    }
    const progress = Math.min((timestamp - start) / 1500, 1);

    animatedOffset.value = `${totalLength * (1 - progress)}`;
    animationProgress.value = progress;

    if (progress < 1) {
      requestAnimationFrame(animate);
    }
  };
  requestAnimationFrame(animate);
};

// 鼠标交互
const handleMouseMove = (event: MouseEvent) => {
  if (!chartData.value || !chartSvg.value) {
    return;
  }

  const rect = chartSvg.value.getBoundingClientRect();
  const mouseX = event.clientX - rect.left;
  const mouseY = event.clientY - rect.top;

  // 找到最近的数据点
  let closestDistance = Infinity;
  let closestDatasetIndex = -1;
  let closestPointIndex = -1;

  chartData.value.datasets.forEach((dataset, datasetIndex) => {
    dataset.data.forEach((value, pointIndex) => {
      const x = getXPosition(pointIndex);
      const y = getYPosition(value);
      const distance = Math.sqrt((mouseX - x) ** 2 + (mouseY - y) ** 2);

      if (distance < 30 && distance < closestDistance) {
        closestDistance = distance;
        closestDatasetIndex = datasetIndex;
        closestPointIndex = pointIndex;
      }
    });
  });

  if (closestDatasetIndex >= 0 && closestPointIndex >= 0) {
    hoveredPoint.value = {
      datasetIndex: closestDatasetIndex,
      pointIndex: closestPointIndex,
      x: mouseX,
      y: mouseY,
    };
  } else {
    hoveredPoint.value = null;
    tooltipData.value = null;
  }
};

const showTooltip = (
  event: MouseEvent,
  dataset: ChartDataset,
  pointIndex: number,
  value: number
) => {
  if (!chartData.value) {
    return;
  }

  const rect = (event.target as SVGElement).getBoundingClientRect();
  const containerRect = chartSvg.value?.getBoundingClientRect();

  if (containerRect) {
    tooltipPosition.value = {
      x: rect.left - containerRect.left + rect.width / 2,
      y: rect.top - containerRect.top - 10,
    };
  }

  tooltipData.value = {
    time: chartData.value.labels[pointIndex],
    label: dataset.label,
    value,
    color: dataset.color,
  };
};

const hideTooltip = () => {
  hoveredPoint.value = null;
  tooltipData.value = null;
};

// 获取分组列表
const fetchGroups = async () => {
  try {
    const response = await getGroupList();
    groupOptions.value = [
      { label: "全部分组", value: null },
      ...response.data.map(group => ({
        label: group.display_name || group.name,
        value: group.id || 0,
      })),
    ];
  } catch (error) {
    console.error("获取分组列表失败:", error);
  }
};

// 获取图表数据
const fetchChartData = async () => {
  try {
    loading.value = true;
    const response = await getDashboardChart(selectedGroup.value || undefined);
    chartData.value = response.data;

    // 延迟启动动画，确保DOM更新完成
    setTimeout(() => {
      startAnimation();
    }, 100);
  } catch (error) {
    console.error("获取图表数据失败:", error);
  } finally {
    loading.value = false;
  }
};

// 监听分组选择变化
watch(selectedGroup, () => {
  fetchChartData();
});

onMounted(() => {
  fetchGroups();
  fetchChartData();
});
</script>

<template>
  <div class="chart-container">
    <div class="chart-header">
      <h3 class="chart-title">24小时请求趋势</h3>
      <n-select
        v-model:value="selectedGroup"
        :options="groupOptions as any"
        placeholder="选择分组"
        size="small"
        style="width: 120px"
        clearable
        @update:value="fetchChartData"
      />
    </div>

    <div v-if="chartData" class="chart-content">
      <div class="chart-legend">
        <div v-for="dataset in chartData.datasets" :key="dataset.label" class="legend-item">
          <div class="legend-color" :style="{ backgroundColor: dataset.color }" />
          <span class="legend-label">{{ dataset.label }}</span>
        </div>
      </div>

      <div class="chart-wrapper">
        <svg
          ref="chartSvg"
          :width="chartWidth"
          :height="chartHeight"
          class="chart-svg"
          @mousemove="handleMouseMove"
          @mouseleave="hideTooltip"
        >
          <!-- 背景网格 -->
          <defs>
            <pattern id="grid" width="40" height="30" patternUnits="userSpaceOnUse">
              <path
                d="M 40 0 L 0 0 0 30"
                fill="none"
                stroke="#f0f0f0"
                stroke-width="1"
                opacity="0.3"
              />
            </pattern>
          </defs>
          <rect width="100%" height="100%" fill="url(#grid)" />

          <!-- Y轴刻度线和标签 -->
          <g class="y-axis">
            <line
              :x1="padding.left"
              :y1="padding.top"
              :x2="padding.left"
              :y2="chartHeight - padding.bottom"
              stroke="#e0e0e0"
              stroke-width="2"
            />
            <g v-for="(tick, index) in yTicks" :key="index">
              <line
                :x1="padding.left - 5"
                :y1="getYPosition(tick)"
                :x2="padding.left"
                :y2="getYPosition(tick)"
                stroke="#666"
                stroke-width="1"
              />
              <text
                :x="padding.left - 10"
                :y="getYPosition(tick) + 4"
                text-anchor="end"
                class="axis-label"
              >
                {{ formatNumber(tick) }}
              </text>
            </g>
          </g>

          <!-- X轴刻度线和标签 -->
          <g class="x-axis">
            <line
              :x1="padding.left"
              :y1="chartHeight - padding.bottom"
              :x2="chartWidth - padding.right"
              :y2="chartHeight - padding.bottom"
              stroke="#e0e0e0"
              stroke-width="2"
            />
            <g v-for="(label, index) in visibleLabels" :key="index">
              <line
                :x1="getXPosition(label.index)"
                :y1="chartHeight - padding.bottom"
                :x2="getXPosition(label.index)"
                :y2="chartHeight - padding.bottom + 5"
                stroke="#666"
                stroke-width="1"
              />
              <text
                :x="getXPosition(label.index)"
                :y="chartHeight - padding.bottom + 18"
                text-anchor="middle"
                class="axis-label"
              >
                {{ label.text }}
              </text>
            </g>
          </g>

          <!-- 数据线条 -->
          <g v-for="(dataset, datasetIndex) in chartData.datasets" :key="dataset.label">
            <!-- 渐变定义 -->
            <defs>
              <linearGradient :id="`gradient-${datasetIndex}`" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" :stop-color="dataset.color" stop-opacity="0.3" />
                <stop offset="100%" :stop-color="dataset.color" stop-opacity="0.05" />
              </linearGradient>
            </defs>

            <!-- 填充区域 -->
            <path
              :d="generateAreaPath(dataset.data)"
              :fill="`url(#gradient-${datasetIndex})`"
              class="area-path"
            />

            <!-- 主线条 -->
            <path
              :d="generateLinePath(dataset.data)"
              :stroke="dataset.color"
              stroke-width="3"
              fill="none"
              class="line-path"
              :style="{
                strokeDasharray: animatedStroke,
                strokeDashoffset: animatedOffset,
                filter: 'drop-shadow(0 2px 4px rgba(0,0,0,0.1))',
              }"
            />

            <!-- 数据点 -->
            <g v-for="(value, pointIndex) in dataset.data" :key="pointIndex">
              <circle
                :cx="getXPosition(pointIndex)"
                :cy="getYPosition(value)"
                r="4"
                :fill="dataset.color"
                :stroke="dataset.color"
                stroke-width="2"
                class="data-point"
                :class="{
                  'point-hover':
                    hoveredPoint?.datasetIndex === datasetIndex &&
                    hoveredPoint?.pointIndex === pointIndex,
                }"
                @mouseenter="showTooltip($event, dataset, pointIndex, value)"
              />
            </g>
          </g>

          <!-- 悬停指示线 -->
          <line
            v-if="hoveredPoint"
            :x1="getXPosition(hoveredPoint.pointIndex)"
            :y1="padding.top"
            :x2="getXPosition(hoveredPoint.pointIndex)"
            :y2="chartHeight - padding.bottom"
            stroke="#999"
            stroke-width="1"
            stroke-dasharray="5,5"
            opacity="0.7"
          />
        </svg>

        <!-- 提示框 -->
        <div
          v-if="tooltipData"
          class="chart-tooltip"
          :style="{
            left: tooltipPosition.x + 'px',
            top: tooltipPosition.y + 'px',
          }"
        >
          <div class="tooltip-time">{{ tooltipData.time }}</div>
          <div class="tooltip-value">
            <span class="tooltip-color" :style="{ backgroundColor: tooltipData.color }" />
            {{ tooltipData.label }}: {{ formatNumber(tooltipData.value) }}
          </div>
        </div>
      </div>
    </div>

    <div v-else class="chart-loading">
      <n-spin size="large" />
      <p>加载中...</p>
    </div>
  </div>
</template>

<style scoped>
.chart-container {
  padding: 20px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 16px;
  box-shadow: 0 8px 32px rgba(31, 38, 135, 0.37);
  backdrop-filter: blur(4px);
  border: 1px solid rgba(255, 255, 255, 0.18);
  color: white;
}

.chart-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.chart-title {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  background: linear-gradient(45deg, #fff, #f0f0f0);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.chart-content {
  background: rgba(255, 255, 255, 0.95);
  border-radius: 12px;
  padding: 20px;
  color: #333;
}

.chart-legend {
  display: flex;
  justify-content: center;
  gap: 24px;
  margin-bottom: 20px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.legend-color {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.legend-label {
  font-size: 14px;
  color: #666;
}

.chart-wrapper {
  position: relative;
  display: flex;
  justify-content: center;
}

.chart-svg {
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.axis-label {
  fill: #666;
  font-size: 12px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
}

.line-path {
  transition: all 0.3s ease;
}

.area-path {
  opacity: 0.6;
  transition: opacity 0.3s ease;
}

.data-point {
  cursor: pointer;
  transition: all 0.2s ease;
}

.data-point:hover,
.point-hover {
  r: 6;
  filter: drop-shadow(0 0 8px rgba(0, 0, 0, 0.3));
}

.chart-tooltip {
  position: absolute;
  background: rgba(0, 0, 0, 0.8);
  color: white;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 12px;
  pointer-events: none;
  transform: translateX(-50%) translateY(-100%);
  z-index: 1000;
  backdrop-filter: blur(4px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.tooltip-time {
  font-weight: 600;
  margin-bottom: 4px;
}

.tooltip-value {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tooltip-color {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.chart-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 300px;
  color: white;
}

.chart-loading p {
  margin-top: 16px;
  font-size: 16px;
  opacity: 0.8;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .chart-container {
    padding: 16px;
  }

  .chart-title {
    font-size: 20px;
  }

  .chart-header {
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
  }

  .chart-legend {
    flex-wrap: wrap;
    gap: 16px;
  }

  .chart-svg {
    width: 100%;
    height: auto;
  }
}

/* 动画效果 */
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

.chart-container {
  animation: fadeInUp 0.6s ease-out;
}

.legend-item {
  animation: fadeInUp 0.6s ease-out;
}

.legend-item:nth-child(2) {
  animation-delay: 0.1s;
}

.legend-item:nth-child(3) {
  animation-delay: 0.2s;
}
</style>
