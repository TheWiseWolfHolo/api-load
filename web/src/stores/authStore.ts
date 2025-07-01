import { defineStore } from 'pinia';
import { ref, computed } from 'vue';

const AUTH_KEY_STORAGE = 'gpt-load-auth-key';

export const useAuthStore = defineStore('auth', () => {
  // State
  const authKey = ref<string>('');
  
  // Computed
  const isAuthenticated = computed(() => !!authKey.value);
  
  // Actions
  function login(key: string) {
    authKey.value = key;
    localStorage.setItem(AUTH_KEY_STORAGE, key);
  }
  
  function logout() {
    authKey.value = '';
    localStorage.removeItem(AUTH_KEY_STORAGE);
  }
  
  function getAuthKey(): string {
    return authKey.value;
  }
  
  function initializeAuth() {
    const storedKey = localStorage.getItem(AUTH_KEY_STORAGE);
    if (storedKey) {
      authKey.value = storedKey;
    }
  }
  
  // 在store初始化时自动恢复认证状态
  initializeAuth();
  
  return {
    // State
    authKey,
    // Computed
    isAuthenticated,
    // Actions
    login,
    logout,
    getAuthKey,
    initializeAuth,
  };
});