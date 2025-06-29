<template>
  <div v-loading="loading">
    <h1>Settings</h1>
    <el-form :model="form" label-width="200px">
      <el-form-item v-for="setting in settings" :key="setting.key" :label="setting.key">
        <el-input v-model="form[setting.key]"></el-input>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="saveSettings">Save</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { useSettingStore } from '@/stores/settingStore';
import { storeToRefs } from 'pinia';
import type { Setting } from '@/types/models';

const settingStore = useSettingStore();
const { settings, loading } = storeToRefs(settingStore);

const form = ref<Record<string, string>>({});

onMounted(() => {
  settingStore.fetchSettings();
});

watch(settings, (newSettings) => {
  form.value = newSettings.reduce((acc, setting) => {
    acc[setting.key] = setting.value;
    return acc;
  }, {} as Record<string, string>);
}, { immediate: true, deep: true });

const saveSettings = () => {
  const settingsToUpdate: Setting[] = Object.entries(form.value).map(([key, value]) => ({
    key,
    value,
  }));
  settingStore.updateSettings(settingsToUpdate);
};
</script>