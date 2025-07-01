<template>
  <div class="p-6 bg-white shadow-md rounded-lg">
    <h3 class="text-lg font-semibold leading-6 text-gray-900 mb-6">系统设置</h3>
    <div v-if="settings" class="space-y-6">
      <SettingItem
        v-model.number="settings.port"
        label="服务端口"
        type="number"
        description="Web 服务和 API 监听的端口。"
        :error="errors['port']"
      />
      <SettingItem
        v-model="settings.cors.allowed_origins"
        label="允许的跨域来源 (CORS)"
        description="允许访问 API 的来源列表，用逗号分隔。使用 '*' 表示允许所有来源。"
        :error="errors['cors.allowed_origins']"
      />
      <SettingItem
        v-model.number="settings.timeout.read"
        label="读取超时 (秒)"
        type="number"
        description="服务器读取请求的超时时间。"
        :error="errors['timeout.read']"
      />
      <SettingItem
        v-model.number="settings.timeout.write"
        label="写入超时 (秒)"
        type="number"
        description="服务器写入响应的超时时间。"
        :error="errors['timeout.write']"
      />
    </div>
    <div v-else>
      <p>正在加载设置...</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { storeToRefs } from 'pinia';
import { useSettingStore } from '@/stores/settingStore';
import SettingItem from './SettingItem.vue';

const settingStore = useSettingStore();
// 我们将在 store 中定义 systemSettings 和 errors
const { systemSettings: settings, errors } = storeToRefs(settingStore);
</script>