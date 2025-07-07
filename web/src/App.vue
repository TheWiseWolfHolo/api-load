<script setup lang="ts">
import GlobalProviders from "@/components/GlobalProviders.vue";
import Layout from "@/components/Layout.vue";
import { useAuthKey } from "@/services/auth";
import { computed } from "vue";

const authKey = useAuthKey();
const isLoggedIn = computed(() => !!authKey.value);
</script>

<template>
  <global-providers>
    <div id="app-root">
      <layout v-if="isLoggedIn" key="layout" />
      <router-view v-else key="auth" />
    </div>
  </global-providers>
</template>

<style>
#app-root {
  width: 100%;
  overflow: hidden;
}
</style>
