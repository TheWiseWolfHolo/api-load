<script setup lang="ts">
import GlobalProviders from "@/components/GlobalProviders.vue";
import GlobalTaskProgressBar from "@/components/GlobalTaskProgressBar.vue";
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

      <!-- 全局任务进度条 -->
      <global-task-progress-bar />
    </div>
  </global-providers>
</template>

<style>
#app-root {
  width: 100%;
  /* height: 100vh; */
  overflow: hidden;
}

.app-transition-enter-active,
.app-transition-leave-active {
  transition: all 0.4s ease;
}

.app-transition-enter-from {
  opacity: 0;
  transform: translateY(20px);
}

.app-transition-leave-to {
  opacity: 0;
  transform: translateY(-20px);
}
</style>
