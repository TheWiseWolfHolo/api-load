import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import type { User } from '@/types/models';

const AUTH_KEY_STORAGE = 'gpt-load-auth-key';

export const useAuthStore = defineStore('auth', () => {
  // State
  const authKey = ref<string>('');
  const user = ref<User | null>(null);
  
  // Computed
  const isAuthenticated = computed(() => !!authKey.value);
  
  // Actions
  function login(key: string) {
    authKey.value = key;
    // For now, we'll just mock a user object.
    // In a real app, you'd fetch this from an API.
    user.value = { id: '1', username: 'admin' };
    localStorage.setItem(AUTH_KEY_STORAGE, key);
  }
  
  function logout() {
    authKey.value = '';
    user.value = null;
    localStorage.removeItem(AUTH_KEY_STORAGE);
  }
  
  function getAuthKey(): string {
    return authKey.value;
  }
  
  function initializeAuth() {
    const storedKey = localStorage.getItem(AUTH_KEY_STORAGE);
    if (storedKey) {
      authKey.value = storedKey;
      // If auth key exists, we can assume the user is logged in.
      // You might want to verify the key with the server here.
      user.value = { id: '1', username: 'admin' };
    }
  }
  
  // 在store初始化时自动恢复认证状态
  initializeAuth();
  
  return {
    // State
    authKey,
    user,
    // Computed
    isAuthenticated,
    // Actions
    login,
    logout,
    getAuthKey,
    initializeAuth,
  };
});