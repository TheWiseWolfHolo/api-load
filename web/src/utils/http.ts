import { useAuthService } from "@/services/auth";
import axios from "axios";
import { appState } from "./app-state";

const http = axios.create({
  baseURL: "/api",
  timeout: 10000,
  headers: { "Content-Type": "application/json" },
});

// 请求拦截器
http.interceptors.request.use(config => {
  appState.loading = true;
  const authKey = localStorage.getItem("authKey");
  if (authKey) {
    config.headers.Authorization = `Bearer ${authKey}`;
  }
  return config;
});

// 响应拦截器
http.interceptors.response.use(
  response => {
    appState.loading = false;
    if (response.config.method !== "get") {
      window.$message.success("操作成功");
    }
    return response.data;
  },
  error => {
    appState.loading = false;
    if (error.response) {
      // The request was made and the server responded with a status code
      // that falls out of the range of 2xx
      if (error.response.status === 401) {
        const { logout } = useAuthService();
        logout();
        window.location.href = "/login";
      }
      window.$message.error(error.response.data?.message || `请求失败: ${error.response.status}`);
    } else if (error.request) {
      // The request was made but no response was received
      window.$message.error("网络错误，请检查您的连接");
    } else {
      // Something happened in setting up the request that triggered an Error
      window.$message.error("请求设置错误");
    }
    console.error("API Error:", error);
    return Promise.reject(error);
  }
);

export default http;
