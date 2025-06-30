import request from './index';
import type { Setting } from '@/types/models';

export function getSettings() {
  return request.get<Setting[]>('/settings');
}

export function updateSettings(settings: Setting[]) {
  return request.put('/settings', settings);
}