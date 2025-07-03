<script setup lang="ts">
// 这里可以添加密钥管理相关的逻辑
</script>

<template>
  <div class="keys-container">
    <div class="page-header">
      <h2 class="page-title">密钥管理</h2>
      <p class="page-subtitle">管理API密钥和访问凭证</p>
    </div>

    <div class="content-placeholder">
      <div class="placeholder-card modern-card">
        <div class="placeholder-icon">🔑</div>
        <h3 class="placeholder-title">密钥管理功能</h3>
        <p class="placeholder-description">
          此功能正在开发中，将提供完整的API密钥管理功能，包括添加、删除、编辑和监控密钥使用情况。
        </p>
        <div class="placeholder-features">
          <div class="feature-item">✨ 密钥添加与删除</div>
          <div class="feature-item">📊 使用情况统计</div>
          <div class="feature-item">🔒 安全性验证</div>
          <div class="feature-item">🔄 自动轮换</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.keys-container {
  max-width: 1000px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 32px;
  text-align: center;
}

.page-title {
  font-size: 2.25rem;
  font-weight: 700;
  background: var(--primary-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0 0 8px 0;
  letter-spacing: -0.5px;
}

.page-subtitle {
  font-size: 1.1rem;
  color: #64748b;
  margin: 0;
  font-weight: 500;
}

.content-placeholder {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
}

.placeholder-card {
  text-align: center;
  max-width: 500px;
  padding: 48px 32px;
  background: rgba(255, 255, 255, 0.98);
}

.placeholder-icon {
  font-size: 4rem;
  margin-bottom: 24px;
  display: block;
}

.placeholder-title {
  font-size: 1.5rem;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 16px 0;
}

.placeholder-description {
  font-size: 1rem;
  color: #64748b;
  line-height: 1.6;
  margin: 0 0 32px 0;
}

.placeholder-features {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
}

.feature-item {
  padding: 12px 16px;
  background: rgba(102, 126, 234, 0.1);
  border-radius: var(--border-radius-md);
  color: #667eea;
  font-weight: 500;
  font-size: 0.9rem;
}

@media (max-width: 768px) {
  .placeholder-card {
    margin: 0 16px;
    padding: 32px 24px;
  }

  .placeholder-features {
    grid-template-columns: 1fr;
  }
}
</style>
