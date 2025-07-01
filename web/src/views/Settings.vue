<template>
  <div class="flex h-full bg-gray-100">
    <!-- Left Navigation -->
    <aside class="w-64 bg-white p-4 shadow-md">
      <h2 class="text-xl font-bold mb-6">设置</h2>
      <nav class="space-y-2">
        <a
          v-for="item in navigation"
          :key="item.name"
          @click="activeTab = item.component"
          :class="[
            'block px-4 py-2 rounded-md cursor-pointer',
            activeTab === item.component
              ? 'bg-indigo-500 text-white'
              : 'text-gray-700 hover:bg-gray-200',
          ]"
        >
          {{ item.name }}
        </a>
      </nav>
    </aside>

    <!-- Right Content -->
    <main class="flex-1 p-8 overflow-y-auto">
      <div class="max-w-4xl mx-auto">
        <transition name="fade" mode="out-in">
          <component :is="activeComponent" />
        </transition>

        <!-- Action Buttons -->
        <div class="mt-8 flex justify-end space-x-4">
          <button
            @click="handleReset"
            class="px-6 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
          >
            重置
          </button>
          <button
            @click="handleSave"
            :disabled="loading"
            class="px-6 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
          >
            {{ loading ? '保存中...' : '保存配置' }}
          </button>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue';
import { useSettingStore } from '@/stores/settingStore';
import { storeToRefs } from 'pinia';
import SystemSettings from '@/components/business/settings/SystemSettings.vue';
import GroupSettings from '@/components/business/settings/GroupSettings.vue';

const settingStore = useSettingStore();
const { loading } = storeToRefs(settingStore);

const navigation = [
  { name: '系统设置', component: 'SystemSettings' },
  { name: '认证设置', component: 'AuthSettings' },
  { name: '性能设置', component: 'PerformanceSettings' },
  { name: '日志设置', component: 'LogSettings' },
  { name: '分组设置', component: 'GroupSettings' },
];

const components: Record<string, any> = {
  SystemSettings,
  GroupSettings,
  // Placeholder for other setting components
  AuthSettings: { template: '<div class="p-6 bg-white shadow-md rounded-lg"><h3 class="text-lg font-semibold">认证设置</h3><p class="text-gray-500 mt-4">此部分功能待开发。</p></div>' },
  PerformanceSettings: { template: '<div class="p-6 bg-white shadow-md rounded-lg"><h3 class="text-lg font-semibold">性能设置</h3><p class="text-gray-500 mt-4">此部分功能待开发。</p></div>' },
  LogSettings: { template: '<div class="p-6 bg-white shadow-md rounded-lg"><h3 class="text-lg font-semibold">日志设置</h3><p class="text-gray-500 mt-4">此部分功能待开发。</p></div>' },
};

const activeTab = shallowRef('SystemSettings');

const activeComponent = computed(() => components[activeTab.value]);

onMounted(() => {
  // Fetch initial data for the default tab
  settingStore.fetchSystemSettings();
});

const handleSave = () => {
  // This logic would need to be more sophisticated if handling multiple setting types
  if (activeTab.value === 'SystemSettings') {
    settingStore.saveSystemSettings();
  }
  // Add logic for other setting types here
};

const handleReset = () => {
  if (activeTab.value === 'SystemSettings') {
    settingStore.resetSystemSettings();
  }
  // Add logic for other setting types here
};
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>