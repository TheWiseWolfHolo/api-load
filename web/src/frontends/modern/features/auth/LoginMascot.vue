<script setup lang="ts">
defineProps<{ quiet?: boolean }>()
</script>

<template>
  <picture class="modern-login-brand" aria-hidden="true">
    <source srcset="/logo-dark.svg" media="(prefers-color-scheme: dark)" />
    <img src="/logo-light.svg" alt="" width="64" height="64" />
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
