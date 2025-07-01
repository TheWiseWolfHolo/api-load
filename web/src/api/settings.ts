import request from './index';
import type { SettingCategory, SystemSettings } from '@/types/models';

// A generic function to get settings for a specific category
export function getSettings<T>(category: SettingCategory) {
  // The backend API would need to support this, e.g., /api/settings/system
  return request.get<T>(`/settings/${category}`);
}

// A generic function to update settings for a specific category
export function updateSettings<T>(category: SettingCategory, settings: T) {
  return request.put(`/settings/${category}`, settings);
}

// Specific functions for system settings as an example
export function getSystemSettings() {
  return getSettings<SystemSettings>('system');
}

export function updateSystemSettings(settings: SystemSettings) {
  return updateSettings('system', settings);
}