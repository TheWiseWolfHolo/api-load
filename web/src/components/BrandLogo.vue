<script setup lang="ts">
import darkLogoUrl from "@/assets/logo-dark.svg";
import lightLogoUrl from "@/assets/logo-light.svg";
</script>

<template>
  <span class="brand-logo" aria-hidden="true">
    <img class="brand-logo__image brand-logo__image--light" :src="lightLogoUrl" alt="" />
    <img class="brand-logo__image brand-logo__image--dark" :src="darkLogoUrl" alt="" />
  </span>
</template>

<style scoped>
.brand-logo {
  display: inline-grid;
  width: 100%;
  height: 100%;
  flex: none;
}

.brand-logo__image {
  display: block;
  grid-area: 1 / 1;
  width: 100%;
  height: 100%;
  object-fit: contain;
  pointer-events: none;
}

.brand-logo__image--dark {
  display: none;
}

:global(:root.dark) .brand-logo__image--light {
  display: none;
}

:global(:root.dark) .brand-logo__image--dark {
  display: block;
}
</style>
