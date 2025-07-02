import { authService } from "@/services/auth";
import axios from "axios";
import { useRouter } from "vue-router";

const http = axios.create({
  baseURL: "/api",
  timeout: 10000,
  headers: { "Content-Type": "application/json" },
});

// 请求拦截器
http.interceptors.request.use(config => {
  const authKey = authService.getAuthKey();
  if (authKey) {
    config.headers.Authorization = `Bearer ${authKey}`;
  }
  return config;
});

// 响应拦截器
http.interceptors.response.use(
  response => response.data,
  error => {
    if (error.response && error.response.status === 401) {
      authService.logout();
      useRouter().push("/login");
    }
    console.error("API Error:", error);
    return Promise.reject(error);
  }
);

export default http;
