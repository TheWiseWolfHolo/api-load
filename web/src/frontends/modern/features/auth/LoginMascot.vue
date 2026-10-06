<script setup lang="ts">
import logoLight from '@shared/brand/logo-light.svg'
import logoDark from '@shared/brand/logo-dark.svg'
defineProps<{ quiet?: boolean }>()
</script>

<template>
  <picture class="modern-login-brand" aria-hidden="true">
    <source :srcset="logoDark" media="(prefers-color-scheme: dark)" />
    <img :src="logoLight" alt="" width="64" height="64" />
  </picture>
</template>

<style scoped>
.modern-login-brand {
  position: absolute;
  top: var(--modern-space-4);
  left: var(--modern-space-8);
  pointer-events: none;
}
</style>
