<template>
  <div class="loading-spinner" :style="{ width: size, height: size }">
    <svg class="spinner" viewBox="0 0 50 50">
      <circle
        class="path"
        cx="25"
        cy="25"
        r="20"
        fill="none"
        :stroke="color"
        stroke-width="5"
      ></circle>
    </svg>
  </div>
</template>

<script setup lang="ts">
import { defineProps, withDefaults } from 'vue';

withDefaults(defineProps<{
  size?: string;
  color?: string;
}>(), {
  size: '48px',
  color: '#409EFF', // Element Plus 主色
});
</script>

<style scoped>
.loading-spinner {
  display: inline-block;
  position: relative;
}

.spinner {
  animation: rotate 2s linear infinite;
}

.path {
  stroke-linecap: round;
  animation: dash 1.5s ease-in-out infinite;
}

@keyframes rotate {
  100% {
    transform: rotate(360deg);
  }
}

@keyframes dash {
  0% {
    stroke-dasharray: 1, 150;
    stroke-dashoffset: 0;
  }
  50% {
    stroke-dasharray: 90, 150;
    stroke-dashoffset: -35;
  }
  100% {
    stroke-dasharray: 90, 150;
    stroke-dashoffset: -124;
  }
}
</style>