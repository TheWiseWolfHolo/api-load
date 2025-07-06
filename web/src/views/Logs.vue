<script setup lang="ts">
import { NCard, NH3, NSpace, NTag, NText } from "naive-ui";
// 这里可以添加日志管理相关的逻辑
</script>

<template>
  <div class="logs-container">
    <n-space vertical size="large">
      <!-- 占位符内容 -->
      <div class="content-placeholder">
        <n-card :bordered="false" class="placeholder-card">
          <n-space vertical align="center" size="large">
            <div class="placeholder-icon">📋</div>
            <n-h3 class="placeholder-title">日志管理功能</n-h3>
            <n-text depth="2" class="placeholder-description">
              此功能正在开发中，将提供完整的系统日志查看和管理功能，包括实时日志、历史记录和日志分析。
            </n-text>

            <n-space wrap size="medium" class="placeholder-features">
              <n-tag type="info" size="medium">📝 实时日志流</n-tag>
              <n-tag type="info" size="medium">🔍 日志搜索过滤</n-tag>
              <n-tag type="info" size="medium">📈 错误统计分析</n-tag>
              <n-tag type="info" size="medium">💾 日志导出功能</n-tag>
            </n-space>
          </n-space>
        </n-card>
      </div>
    </n-space>
  </div>
</template>

<style scoped>
.page-header-card {
  background: rgba(255, 255, 255, 0.98);
  border-radius: var(--border-radius-lg);
  border: 1px solid rgba(255, 255, 255, 0.3);
  animation: fadeInUp 0.2s ease-out;
}

.page-title {
  font-size: 2.25rem;
  font-weight: 700;
  margin: 0;
  letter-spacing: -0.5px;
}

.page-subtitle {
  font-size: 1.1rem;
  font-weight: 500;
}

.content-placeholder {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
  animation: fadeInUp 0.2s ease-out 0.1s both;
}

.placeholder-card {
  text-align: center;
  max-width: 500px;
  padding: 48px 32px;
  background: rgba(255, 255, 255, 0.98);
  border-radius: var(--border-radius-lg);
  border: 1px solid rgba(255, 255, 255, 0.3);
}

.placeholder-icon {
  font-size: 4rem;
  display: block;
}

.placeholder-title {
  font-size: 1.5rem;
  font-weight: 600;
  margin: 0;
}

.placeholder-description {
  font-size: 1rem;
  line-height: 1.6;
  text-align: center;
  max-width: 400px;
}

.placeholder-features {
  justify-content: center;
  margin-top: 8px;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
