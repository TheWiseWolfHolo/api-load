<script setup lang="ts">
defineProps<{
  compact?: boolean
  resolvedTheme: 'light' | 'dark'
}>()
</script>

<template>
  <span class="modern-brand-logo" aria-label="API-Load">
    <img
      :src="resolvedTheme === 'dark' ? '/logo-dark.svg' : '/logo-light.svg'"
      alt=""
      :width="compact ? 32 : 40"
      :height="compact ? 32 : 40"
    />
    <span v-if="!compact">API-Load</span>
  </span>
</template>

<style scoped>
.modern-brand-logo {
  display: flex;
  align-items: center;
  gap: var(--modern-space-3);
  color: var(--modern-text);
  font-weight: var(--modern-weight-semibold);
}
</style>
