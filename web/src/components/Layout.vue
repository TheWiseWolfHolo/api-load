<script setup lang="ts">
import GlobalTaskProgressBar from "@/components/GlobalTaskProgressBar.vue";
import Logout from "@/components/Logout.vue";
import NavBar from "@/components/NavBar.vue";
</script>

<template>
  <n-layout class="main-layout">
    <n-layout-header class="layout-header">
      <div class="header-content">
        <div class="header-brand">
          <div class="brand-icon">
            <img src="@/assets/logo.png" alt="" />
          </div>
          <h1 class="brand-title">GPT Load</h1>
        </div>

        <nav-bar class="header-nav" />

        <div class="header-actions">
          <logout />
        </div>
      </div>
    </n-layout-header>

    <n-layout-content class="layout-content">
      <div class="content-wrapper">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </div>
    </n-layout-content>
  </n-layout>

  <!-- 全局任务进度条 -->
  <global-task-progress-bar />
</template>

<style scoped>
.main-layout {
  background: transparent;
}

.layout-header {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(0, 0, 0, 0.08);
  box-shadow: var(--shadow-sm);
  position: sticky;
  top: 0;
  z-index: 100;
  padding: 0 24px;
}

.header-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px;
  overflow-x: auto;
  max-width: 1200px;
  margin: 0 auto;
}

.header-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.brand-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 35px;
  height: 35px;
  img {
    height: 100%;
  }
}

.brand-title {
  font-size: 1.4rem;
  font-weight: 700;
  background: var(--primary-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0;
  letter-spacing: -0.3px;
}

.header-actions {
  flex-shrink: 0;
}

.layout-content {
  flex: 1;
  overflow: auto;
  background: transparent;
  max-width: 1200px;
  margin: 0 auto;
}

.content-wrapper {
  padding: 24px 12px;
}
</style>
