<script setup lang="ts">
import {
  BarChartOutline,
  DocumentTextOutline,
  KeyOutline,
  ServerOutline,
  SettingsOutline,
} from "@vicons/ionicons5";
import { NIcon, type MenuOption } from "naive-ui";
import { computed, h, type Component, watch } from "vue";
import { RouterLink, useRoute } from "vue-router";
import { useI18n } from "vue-i18n";

const { t } = useI18n();

const props = defineProps({
  mode: {
    type: String,
    default: "horizontal",
  },
});

const emit = defineEmits(["close"]);

const menuOptions = computed<MenuOption[]>(() => {
  const options: MenuOption[] = [
    renderMenuItem("dashboard", t("nav.dashboard"), BarChartOutline),
    renderMenuItem("keys", t("nav.keys"), KeyOutline),
    renderMenuItem("resource-pools", t("nav.resourcePools"), ServerOutline),
    renderMenuItem("logs", t("nav.logs"), DocumentTextOutline),
    renderMenuItem("settings", t("nav.settings"), SettingsOutline),
  ];

  return options;
});

const route = useRoute();
const activeMenu = computed(() => route.name);

watch(activeMenu, () => {
  if (props.mode === "vertical") {
    emit("close");
  }
});

function renderMenuItem(key: string, label: string, icon: Component): MenuOption {
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
            h(NIcon, { class: "nav-item-icon", component: icon }),
            h("span", { class: "nav-item-text" }, label),
          ],
        }
      ),
    key,
  };
}
</script>

<template>
  <div class="nav-shell">
    <n-menu
      :mode="mode"
      :options="menuOptions"
      :value="activeMenu"
      :responsive="mode === 'horizontal'"
      class="modern-menu"
    />
  </div>
</template>

<style scoped>
.nav-shell {
  width: 100%;
  min-width: 0;
}

:deep(.nav-menu-item) {
  display: flex;
  align-items: center;
  gap: 8px;
  width: max-content;
  text-decoration: none;
  color: inherit;
  padding: 8px 6px;
  border-radius: var(--border-radius-md);
  transition: all 0.2s ease;
  font-weight: 500;
  white-space: nowrap;
}

:deep(.nav-item-icon) {
  width: 17px;
  height: 17px;
  color: var(--text-tertiary);
  transition: color 0.2s ease;
}

:deep(.n-menu-item) {
  border-radius: var(--border-radius-md);
}

:deep(.n-menu--horizontal) {
  justify-content: center;
}

:deep(.n-menu--horizontal .n-menu-item-content-header) {
  min-width: max-content;
}

:deep(.nav-item-text) {
  white-space: nowrap;
}

:deep(.n-menu--vertical .n-menu-item-content) {
  justify-content: center;
}

:deep(.n-menu--vertical .n-menu-item) {
  margin: 4px 8px;
}

:deep(.n-menu-item:hover) {
  background: var(--primary-color-suppl);
  border-radius: var(--border-radius-md);
}

:deep(.n-menu-item--selected) {
  background: var(--primary-color-suppl);
  color: var(--primary-color);
  font-weight: 600;
  box-shadow: none;
  border: 1px solid var(--primary-border);
  border-radius: var(--border-radius-md);
}

:deep(.n-menu-item--selected .nav-item-icon) {
  color: var(--primary-color);
}

:deep(.n-menu-item--selected:hover) {
  background: var(--primary-color-suppl-hover);
}
</style>
