<template>
  <div class="main-layout">
    <HeaderNav />
    <main class="main-content">
      <router-view />
    </main>
    <Footer />
  </div>
</template>

<script setup lang="ts">
import HeaderNav from "./HeaderNav.vue";
import Footer from "./Footer.vue";
</script>

<style scoped>
.main-layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  background-color: var(--el-bg-color-page);
}

.main-content {
  flex: 1;
  padding: 20px;
  overflow: auto;
}

@media (max-width: 768px) {
  .main-content {
    padding: 10px;
  }
}
</style>
