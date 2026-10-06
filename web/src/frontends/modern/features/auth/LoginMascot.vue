<script setup lang="ts">
import logoLight from '@shared/brand/logo-light.svg'
import logoDark from '@shared/brand/logo-dark.svg'
import { usePreferences } from '@modern/app/preferences'

const { resolvedTheme } = usePreferences()
</script>

<template>
  <img
    class="modern-login-brand"
    :src="resolvedTheme === 'dark' ? logoDark : logoLight"
    alt=""
    width="64"
    height="64"
  />
</template>

<style scoped>
.modern-login-brand {
  display: block;
  pointer-events: none;
}
</style>
