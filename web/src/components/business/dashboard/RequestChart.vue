<template>
  <div class="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-md">
    <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">
      分组请求统计
    </h3>
    <div ref="chartRef" style="width: 100%; height: 400px"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from "vue";
import { storeToRefs } from "pinia";
import * as echarts from "echarts";
import { useDashboardStore } from "@/stores/dashboardStore";

const chartRef = ref<HTMLElement | null>(null);
const dashboardStore = useDashboardStore();
const { chartData } = storeToRefs(dashboardStore);
let chartInstance: echarts.ECharts | null = null;

const initChart = () => {
  if (chartRef.value) {
    chartInstance = echarts.init(chartRef.value);
    setChartOptions();
  }
};

const setChartOptions = () => {
  if (!chartInstance) return;

  const options: echarts.EChartsOption = {
    tooltip: {
      trigger: "axis",
      axisPointer: {
        type: "shadow",
      },
    },
    xAxis: {
      type: "category",
      data: chartData.value.labels,
      axisLabel: {
        rotate: 45,
        interval: 0,
      },
    },
    yAxis: {
      type: "value",
      name: "请求数",
    },
    series: [
      {
        name: "请求数",
        data: chartData.value.data,
        type: "bar",
        itemStyle: {
          color: "#409EFF",
        },
      },
    ],
    grid: {
      left: "3%",
      right: "4%",
      bottom: "15%",
      containLabel: true,
    },
  };

  chartInstance.setOption(options);
};

const resizeChart = () => {
  chartInstance?.resize();
};

onMounted(() => {
  initChart();
  window.addEventListener("resize", resizeChart);
});

onUnmounted(() => {
  chartInstance?.dispose();
  window.removeEventListener("resize", resizeChart);
});

watch(
  chartData,
  () => {
    setChartOptions();
  },
  { deep: true }
);
</script>
