import axios from "axios";

const apiClient = axios.create({
  baseURL: "/api",
  headers: {
    "Content-Type": "application/json",
  },
});

// 可以添加请求和响应拦截器
// apiClient.interceptors.request.use(...)
// apiClient.interceptors.response.use(...)

export default apiClient;
