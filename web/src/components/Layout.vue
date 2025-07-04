<script setup lang="ts">
import Logout from "@/components/Logout.vue";
import NavBar from "@/components/NavBar.vue";
</script>

<template>
  <n-layout class="main-layout">
    <n-layout-header class="layout-header">
      <div class="header-content">
        <div class="header-brand">
          <div class="brand-icon">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none">
              <path
                d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
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
</template>

<style scoped>
.main-layout {
  /* height: 100vh; */
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
  max-width: 1400px;
  margin: 0 auto;
}

.header-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 100%;
  padding: 0 8px;
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
  width: 40px;
  height: 40px;
  background: var(--primary-gradient);
  border-radius: var(--border-radius-md);
  color: white;
  box-shadow: var(--shadow-md);
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

.header-nav {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  width: 100%;
  max-width: 600px;
  display: flex;
  justify-content: center;
}

.header-actions {
  flex-shrink: 0;
  margin-left: auto;
}

.layout-content {
  flex: 1;
  overflow: auto;
  background: transparent;
}

.content-wrapper {
  max-width: 1400px;
  margin: 0 auto;
  padding: 24px 12px;
  height: calc(100vh - 64px);
  overflow: auto;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .header-content {
    padding: 0 4px;
  }

  .brand-title {
    font-size: 1.2rem;
  }

  .content-wrapper {
    padding: 16px;
  }

  .header-nav {
    display: none;
  }
}

@media (max-width: 480px) {
  .brand-title {
    display: none;
  }

  .content-wrapper {
    padding: 12px;
  }
}

:deep(.n-layout-header) {
  height: 64px;
  padding: 0;
}

:deep(.n-layout-content) {
  padding: 0;
}
</style>
