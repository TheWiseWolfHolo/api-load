<template>
  <base-info-card />
  <line-chart class="chart" />
</template>

<script setup lang="ts">
import BaseInfoCard from "@/components/BaseInfoCard.vue";
import LineChart from "@/components/LineChart.vue";
</script>

<style scoped>
.chart {
  margin-top: 20px;
}
</style>
