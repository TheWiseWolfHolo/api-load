<template>
  <el-container style="height: 100vh;">
    <el-header class="header">
      <div class="header-content">
        <div class="header-title">
          <h3>GPT Load 管理面板</h3>
        </div>
        <div class="header-actions" v-if="authStore.isAuthenticated">
          <el-dropdown @command="handleCommand">
            <span class="el-dropdown-link">
              <el-icon><User /></el-icon>
              <span style="margin-left: 5px;">管理员</span>
              <el-icon class="el-icon--right"><arrow-down /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </el-header>
    
    <el-container>
      <el-aside width="200px">
        <el-menu
          :default-active="activeIndex"
          class="el-menu-vertical-demo"
          @select="handleSelect"
          router
        >
          <el-menu-item index="/dashboard">
            <el-icon><Odometer /></el-icon>
            <template #title>
              <span>Dashboard</span>
            </template>
          </el-menu-item>
          <el-menu-item index="/groups">
            <el-icon><Files /></el-icon>
            <template #title>
              <span>Groups</span>
            </template>
          </el-menu-item>
          <el-menu-item index="/logs">
            <el-icon><Document /></el-icon>
            <template #title>
              <span>Logs</span>
            </template>
          </el-menu-item>
          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
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
  </el-container>
</template>

<script lang="ts" setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  User,
  ArrowDown,
  SwitchButton,
  Odometer,
  Files,
  Document,
  Setting
} from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/authStore'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const activeIndex = ref(route.path)

const handleSelect = (key: string, keyPath: string[]) => {
  console.log(key, keyPath)
}

const handleCommand = async (command: string) => {
  if (command === 'logout') {
    try {
      await ElMessageBox.confirm(
        '确定要退出登录吗？',
        '提示',
        {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning',
        }
      )
      
      authStore.logout()
      ElMessage.success('已退出登录')
      await router.push('/login')
    } catch {
      // 用户取消退出
    }
  }
}
</script>

<style scoped>
.header {
  background-color: #fff;
  border-bottom: 1px solid #e4e7ed;
  padding: 0;
  line-height: 60px;
  box-shadow: 0 1px 4px rgba(0,21,41,.08);
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 100%;
  padding: 0 20px;
}

.header-title h3 {
  margin: 0;
  color: #303133;
  font-weight: 500;
}

.header-actions {
  display: flex;
  align-items: center;
}

.el-dropdown-link {
  cursor: pointer;
  color: #606266;
  display: flex;
  align-items: center;
  padding: 0 12px;
  height: 40px;
  border-radius: 4px;
  transition: background-color 0.3s;
}

.el-dropdown-link:hover {
  background-color: #f5f7fa;
  color: #409eff;
}

.el-menu-vertical-demo {
  height: calc(100vh - 61px);
  border-right: 1px solid #e4e7ed;
}

:deep(.el-menu-item) {
  height: 50px;
  line-height: 50px;
}

:deep(.el-menu-item.is-active) {
  background-color: #ecf5ff;
  color: #409eff;
}

:deep(.el-menu-item:hover) {
  background-color: #f5f7fa;
}
</style>