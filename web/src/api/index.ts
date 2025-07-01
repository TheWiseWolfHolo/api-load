import axios from "axios";
import { useAuthStore } from "@/stores/authStore";
import router from "@/router";

const apiClient = axios.create({
  baseURL: "/api",
  headers: {
    "Content-Type": "application/json",
  },
});

// 请求拦截器：自动添加认证头
apiClient.interceptors.request.use(
  (config) => {
    const authStore = useAuthStore();
    const authKey = authStore.getAuthKey();
    
    if (authKey) {
      config.headers.Authorization = `Bearer ${authKey}`;
    }
    
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 响应拦截器：处理401认证失败
apiClient.interceptors.response.use(
  (response) => {
    return response;
  },
  (error) => {
    if (error.response?.status === 401) {
      // 认证失败，清除登录状态并跳转到登录页
      const authStore = useAuthStore();
      authStore.logout();
      
      // 跳转到登录页（如果不在登录页的话）
      if (router.currentRoute.value.path !== '/login') {
        router.push('/login');
      }
    }
    
    return Promise.reject(error);
  }
);

export default apiClient;
