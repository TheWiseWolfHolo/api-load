<template>
  <div ref="chart" style="width: 100%; height: 400px;"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import * as echarts from 'echarts';
import type { GroupRequestStat } from '@/types/models';

const props = defineProps<{
  data: GroupRequestStat[];
}>();

const chart = ref<HTMLElement | null>(null);
let myChart: echarts.ECharts | null = null;

const initChart = () => {
  if (chart.value) {
    myChart = echarts.init(chart.value);
    updateChart();
  }
};

const updateChart = () => {
  if (!myChart) return;
  myChart.setOption({
    title: {
      text: '各分组请求量',
    },
    tooltip: {},
    xAxis: {
      data: props.data.map(item => item.group_name),
    },
    yAxis: {},
    series: [
      {
        name: '请求量',
        type: 'bar',
        data: props.data.map(item => item.request_count),
      },
    ],
  });
};

onMounted(() => {
  initChart();
});

watch(() => props.data, () => {
  updateChart();
}, { deep: true });
</script>