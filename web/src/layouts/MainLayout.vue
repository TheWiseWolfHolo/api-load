<template>
  <el-container style="height: 100vh;">
    <el-aside width="200px">
      <el-menu
        :default-active="activeIndex"
        class="el-menu-vertical-demo"
        @select="handleSelect"
        router
      >
        <el-menu-item index="/dashboard">
          <template #title>
            <span>Dashboard</span>
          </template>
        </el-menu-item>
        <el-menu-item index="/groups">
          <template #title>
            <span>Groups</span>
          </template>
        </el-menu-item>
        <el-menu-item index="/logs">
          <template #title>
            <span>Logs</span>
          </template>
        </el-menu-item>
        <el-menu-item index="/settings">
          <template #title>
            <span>Settings</span>
          </template>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-main>
      <router-view></router-view>
    </el-main>
  </el-container>
</template>

<script lang="ts" setup>
import { ref } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const activeIndex = ref(route.path)

const handleSelect = (key: string, keyPath: string[]) => {
  console.log(key, keyPath)
}
</script>

<style scoped>
.el-menu-vertical-demo {
  height: 100%;
}
</style>