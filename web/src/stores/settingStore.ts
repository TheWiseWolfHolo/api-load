import { defineStore } from 'pinia';
import { getSystemSettings, updateSystemSettings } from '@/api/settings';
import type { SystemSettings } from '@/types/models';
import { ElMessage } from 'element-plus';

interface SettingsState {
  systemSettings: SystemSettings | null;
  loading: boolean;
  error: any;
  errors: Record<string, string>; // For field-specific validation errors
}

export const useSettingStore = defineStore('setting', {
  state: (): SettingsState => ({
    systemSettings: null,
    loading: false,
    error: null,
    errors: {},
  }),
  actions: {
    async fetchSystemSettings() {
      this.loading = true;
      this.error = null;
      try {
        const response = await getSystemSettings();
        this.systemSettings = response.data;
      } catch (error) {
        this.error = error;
        ElMessage.error('Failed to fetch system settings.');
      } finally {
        this.loading = false;
      }
    },
    async saveSystemSettings() {
      if (!this.systemSettings) return;

      this.loading = true;
      this.error = null;
      this.errors = {};

      // Basic validation example
      if (this.systemSettings.port < 1 || this.systemSettings.port > 65535) {
        this.errors['port'] = 'Port must be between 1 and 65535.';
      }
      if (Object.keys(this.errors).length > 0) {
        this.loading = false;
        ElMessage.error('Please correct the errors before saving.');
        return;
      }

      try {
        await updateSystemSettings(this.systemSettings);
        await this.fetchSystemSettings(); // Refresh state
        ElMessage.success('System settings updated successfully.');
      } catch (error) {
        this.error = error;
        ElMessage.error('Failed to update system settings.');
      } finally {
        this.loading = false;
      }
    },
    // Action to reset settings to their original state (fetched from server)
    async resetSystemSettings() {
      await this.fetchSystemSettings();
    },
  },
});