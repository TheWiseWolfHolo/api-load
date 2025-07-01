<template>
  <div id="app">
    <router-view />
  </div>
</template>

<script setup lang="ts">
// App.vue 现在只需要渲染路由视图
// 路由器会决定是渲染 Login 组件还是 MainLayout 组件
console.log("App.vue loaded");
</script>

<style>
#app {
  width: 100%;
  min-height: 100vh;
  margin: 0;
  padding: 0;
}
</style>
