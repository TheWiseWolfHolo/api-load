import vue from "@vitejs/plugin-vue";
import path from "path";
import { defineConfig } from "vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  // 解析配置
  resolve: {
    // 配置路径别名
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  // 开发服务器配置
  server: {
    // 代理配置示例
    proxy: {
      "/api": {
        target: "http://localhost:3000",
        changeOrigin: true,
      },
    },
  },
  // 构建配置
  build: {
    outDir: "../cmd/gpt-load/dist",
    assetsDir: "assets",
  },
});
