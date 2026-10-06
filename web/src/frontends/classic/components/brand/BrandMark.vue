<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    size?: number | string
    label?: string
  }>(),
  { size: 24, label: undefined },
)
const dimension = computed(() => (typeof props.size === 'number' ? String(props.size) : props.size))
</script>

<template>
  <picture class="brand-mark">
    <source srcset="/logo-dark.svg" media="(prefers-color-scheme: dark)" />
    <img src="/logo-light.svg" :width="dimension" :height="dimension" :alt="label ?? ''" />
  </picture>
</template>

<style scoped>
.brand-mark {
  display: block;
  flex: none;
}
</style>
