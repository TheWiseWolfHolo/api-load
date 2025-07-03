<script setup lang="ts">
import type { MenuOption } from "naive-ui";
import { computed, h } from "vue";
import { RouterLink, useRoute } from "vue-router";

const menuOptions: MenuOption[] = [
  renderMenuItem("dashboard", "仪表盘", "📊"),
  renderMenuItem("keys", "密钥管理", "🔑"),
  renderMenuItem("logs", "日志", "📋"),
  renderMenuItem("settings", "系统设置", "⚙️"),
];

const route = useRoute();
const activeMenu = computed(() => route.name);

function renderMenuItem(key: string, label: string, icon: string): MenuOption {
  return {
    label: () =>
      h(
        RouterLink,
        {
          to: {
            name: key,
          },
          class: "nav-menu-item",
        },
        {
          default: () => [
            h("span", { class: "nav-item-icon" }, icon),
            h("span", { class: "nav-item-text" }, label),
          ],
        }
      ),
    key,
  };
}
</script>

<template>
  <div class="navbar-container">
    <n-menu mode="horizontal" :options="menuOptions" :value="activeMenu" class="modern-menu" />
  </div>
</template>

<style scoped>
.navbar-container {
  flex: 1;
  display: flex;
  justify-content: center;
  max-width: 600px;
}

.nav-menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  color: inherit;
  padding: 8px 16px;
  border-radius: var(--border-radius-md);
  transition: all 0.3s ease;
  font-weight: 500;
}

.nav-menu-item:hover {
  background: rgba(102, 126, 234, 0.1);
  transform: translateY(-1px);
}

.nav-item-icon {
  font-size: 1.1rem;
  display: flex;
  align-items: center;
}

.nav-item-text {
  font-size: 0.95rem;
  letter-spacing: 0.2px;
}

:deep(.n-menu) {
  background: transparent;
  border-bottom: none;
}

:deep(.n-menu-item) {
  border-radius: var(--border-radius-md);
  margin: 0 4px;
  transition: all 0.3s ease;
}

:deep(.n-menu-item:hover) {
  background: rgba(102, 126, 234, 0.1);
  transform: translateY(-1px);
}

:deep(.n-menu-item--selected) {
  background: var(--primary-gradient);
  color: white;
  font-weight: 600;
  box-shadow: var(--shadow-md);
}

:deep(.n-menu-item--selected:hover) {
  background: linear-gradient(135deg, #5a6fd8 0%, #6a4190 100%);
  transform: translateY(-1px);
}

:deep(.n-menu-item-content) {
  border-radius: var(--border-radius-md);
}

:deep(.n-menu-item-content-header) {
  overflow: visible;
}

@media (max-width: 768px) {
  .navbar-container {
    display: none;
  }
}
</style>
