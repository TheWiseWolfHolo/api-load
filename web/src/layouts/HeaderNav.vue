<template>
  <header class="header">
    <div class="header-container">
      <div class="header-content">
        <div class="header-brand">
          <router-link to="/" class="brand-link">GPT-Load</router-link>
        </div>

        <!-- Mobile Menu Button -->
        <div class="mobile-menu-button">
          <el-button text @click="isMenuOpen = !isMenuOpen">
            <el-icon size="20"><Menu /></el-icon>
          </el-button>
        </div>

        <!-- Desktop Menu -->
        <nav class="desktop-nav">
          <router-link
            v-for="item in menuItems"
            :key="item.path"
            :to="item.path"
            class="nav-link"
            active-class="nav-link-active"
          >
            {{ item.name }}
          </router-link>
        </nav>

        <!-- User Menu -->
        <div class="user-menu">
          <div v-if="authStore.isAuthenticated">
            <el-dropdown @command="handleUserMenuCommand">
              <el-button text>
                <span>{{ authStore.user?.username || "User" }}</span>
                <el-icon class="el-icon--right"><arrow-down /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="logout">退出登录</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
          <router-link v-else to="/login" class="login-link">登录</router-link>
        </div>
      </div>

      <!-- Mobile Menu -->
      <div v-if="isMenuOpen" class="mobile-menu">
        <nav class="mobile-nav">
          <router-link
            v-for="item in menuItems"
            :key="item.path"
            :to="item.path"
            @click="isMenuOpen = false"
            class="mobile-nav-link"
            active-class="mobile-nav-link-active"
          >
            {{ item.name }}
          </router-link>
          <hr class="mobile-menu-divider" />
          <div v-if="authStore.isAuthenticated" class="mobile-user-section">
            <div class="mobile-username">
              {{ authStore.user?.username || "User" }}
            </div>
            <el-button text type="danger" @click="logout" class="mobile-logout">
              退出登录
            </el-button>
          </div>
          <router-link
            v-else
            to="/login"
            @click="isMenuOpen = false"
            class="mobile-nav-link"
          >
            登录
          </router-link>
        </nav>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/authStore";
import { Menu, ArrowDown } from "@element-plus/icons-vue";

const router = useRouter();
const authStore = useAuthStore();

const isMenuOpen = ref(false);

const menuItems = [
  { name: "仪表盘", path: "/dashboard" },
  { name: "分组管理", path: "/groups" },
  { name: "日志", path: "/logs" },
  { name: "系统设置", path: "/settings" },
];

const handleUserMenuCommand = (command: string) => {
  if (command === "logout") {
    logout();
  }
};

const logout = () => {
  authStore.logout();
  isMenuOpen.value = false;
  router.push("/login");
};
</script>

<style scoped>
.header {
  background-color: #ffffff;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06);
  border-bottom: 1px solid #e5e7eb;
}

.header-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 16px;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 64px;
}

.header-brand {
  display: flex;
  align-items: center;
}

.brand-link {
  font-size: 20px;
  font-weight: 700;
  color: #1f2937;
  text-decoration: none;
}

.brand-link:hover {
  color: #3b82f6;
}

.mobile-menu-button {
  display: none;
}

.desktop-nav {
  display: flex;
  align-items: center;
  gap: 24px;
}

.nav-link {
  color: #6b7280;
  text-decoration: none;
  font-weight: 500;
  padding: 8px 12px;
  border-radius: 6px;
  transition: color 0.2s;
}

.nav-link:hover {
  color: #3b82f6;
}

.nav-link-active {
  color: #3b82f6;
  font-weight: 600;
}

.user-menu {
  display: flex;
  align-items: center;
}

.login-link {
  color: #6b7280;
  text-decoration: none;
  font-weight: 500;
}

.login-link:hover {
  color: #3b82f6;
}

.mobile-menu {
  display: none;
  border-top: 1px solid #e5e7eb;
  padding: 16px 0;
}

.mobile-nav {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.mobile-nav-link {
  color: #6b7280;
  text-decoration: none;
  padding: 12px 16px;
  border-radius: 6px;
  font-weight: 500;
}

.mobile-nav-link:hover {
  color: #3b82f6;
  background-color: #f3f4f6;
}

.mobile-nav-link-active {
  color: #3b82f6;
  background-color: #dbeafe;
  font-weight: 600;
}

.mobile-menu-divider {
  margin: 8px 0;
  border: none;
  border-top: 1px solid #e5e7eb;
}

.mobile-user-section {
  padding: 12px 16px;
}

.mobile-username {
  font-weight: 500;
  color: #1f2937;
  margin-bottom: 8px;
}

.mobile-logout {
  width: 100%;
  justify-content: flex-start;
}

@media (max-width: 768px) {
  .mobile-menu-button {
    display: block;
  }

  .desktop-nav,
  .user-menu {
    display: none;
  }

  .mobile-menu {
    display: block;
  }
}
</style>
