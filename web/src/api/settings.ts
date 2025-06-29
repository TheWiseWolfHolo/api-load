import request from './index';
import type { Setting } from '@/types/models';

export function getSettings() {
  return request.get<Setting[]>('/api/settings');
}

export function updateSettings(settings: Setting[]) {
  return request.put('/api/settings', settings);
}