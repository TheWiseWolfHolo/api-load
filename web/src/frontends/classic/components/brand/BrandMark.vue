<script setup lang="ts">
import { computed } from 'vue'
import logoLight from '@shared/brand/logo-light.svg'
import logoDark from '@shared/brand/logo-dark.svg'

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
    <source :srcset="logoDark" media="(prefers-color-scheme: dark)" />
    <img :src="logoLight" :width="dimension" :height="dimension" :alt="label ?? ''" />
  </picture>
</template>

<style scoped>
.brand-mark {
  display: block;
  flex: none;
}
</style>
