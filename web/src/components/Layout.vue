<template>
  <n-layout v-if="authService.isLoggedIn()">
    <n-layout-header class="flex items-center">
      <h1 class="layout-header-title">T.COM</h1>
      <nav-bar />
      <logout />
    </n-layout-header>
    <n-layout-content class="layout-content" content-style="padding: 24px;">
      <router-view />
    </n-layout-content>
  </n-layout>
  <router-view v-else />
</template>

<script setup lang="ts">
import NavBar from "@/components/NavBar.vue";
import Logout from "@/components/Logout.vue";
import { authService } from "@/services/auth";
</script>

<style scoped>
.layout-header-title {
  margin-top: 0;
  margin-bottom: 8px;
  margin-right: 20px;
}
.layout-content {
  width: 100%;
}
</style>
