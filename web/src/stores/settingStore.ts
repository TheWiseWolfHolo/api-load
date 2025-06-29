import { defineStore } from 'pinia';
import { getSettings, updateSettings as apiUpdateSettings } from '@/api/settings';
import type { Setting } from '@/types/models';
import { ElMessage } from 'element-plus';

interface SettingState {
  settings: Setting[];
  loading: boolean;
  error: any;
}

export const useSettingStore = defineStore('setting', {
  state: (): SettingState => ({
    settings: [],
    loading: false,
    error: null,
  }),
  actions: {
    async fetchSettings() {
      this.loading = true;
      this.error = null;
      try {
        const response = await getSettings();
        this.settings = response.data;
      } catch (error) {
        this.error = error;
        ElMessage.error('Failed to fetch settings.');
      } finally {
        this.loading = false;
      }
    },
    async updateSettings(settingsToUpdate: Setting[]) {
      this.loading = true;
      this.error = null;
      try {
        await apiUpdateSettings(settingsToUpdate);
        await this.fetchSettings(); // Refresh the settings after update
        ElMessage.success('Settings updated successfully.');
      } catch (error) {
        this.error = error;
        ElMessage.error('Failed to update settings.');
      } finally {
        this.loading = false;
      }
    },
  },
});