<template>
  <n-menu mode="horizontal" :options="menuOptions" :value="activeMenu" responsive />
</template>

<script setup lang="ts">
import type { MenuOption } from "naive-ui";
import { h, computed } from "vue";
import { RouterLink, useRoute } from "vue-router";

const menuOptions: MenuOption[] = [
  renderMenuItem("dashboard", "仪表盘"),
  renderMenuItem("keys", "密钥管理"),
  renderMenuItem("logs", "日志"),
  renderMenuItem("settings", "系统设置"),
];

const route = useRoute();
const activeMenu = computed(() => route.name);

function renderMenuItem(key: string, label: string): MenuOption {
  return {
    label: () =>
      h(
        RouterLink,
        {
          to: {
            name: key,
          },
        },
        { default: () => label }
      ),
    key,
  };
}
</script>
