<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group, GroupStats } from "@/types/models";
import { onMounted, ref } from "vue";

interface Props {
  group: Group;
}

const props = defineProps<Props>();

const stats = ref<GroupStats | null>(null);
const loading = ref(false);
const showDetails = ref(false);

onMounted(() => {
  loadStats();
});

async function loadStats() {
  try {
    loading.value = true;
    stats.value = await keysApi.getGroupStats(props.group.id);
  } catch (error) {
    console.error("加载统计信息失败:", error);
  } finally {
    loading.value = false;
  }
}

function handleEdit() {
  window.$message.info("编辑分组功能开发中...");
}

function handleDelete() {
  window.$message.info("删除分组功能开发中...");
}

function toggleDetails() {
  showDetails.value = !showDetails.value;
}

function formatNumber(num: number): string {
  if (num >= 1000000) {
    return `${(num / 1000000).toFixed(1)}M`;
  }
  if (num >= 1000) {
    return `${(num / 1000).toFixed(1)}K`;
  }
  return num.toString();
}

function formatPercentage(num: number): string {
  return `${num.toFixed(1)}%`;
}
</script>

<template>
  <div class="group-info-container">
    <div class="group-info-card">
      <!-- 头部信息 -->
      <div class="card-header">
        <div class="header-left">
          <h3 class="group-title">{{ group.display_name || group.name }}</h3>
          <div class="group-meta">
            <span class="channel-badge" :class="`channel-${group.channel_type}`">
              {{ group.channel_type.toUpperCase() }}
            </span>
            <span class="group-id">#{{ group.id }}</span>
          </div>
        </div>
        <div class="header-actions">
          <button class="action-btn" @click="handleEdit" title="编辑分组">
            <span class="icon">✏️</span>
          </button>
          <button class="action-btn delete-btn" @click="handleDelete" title="删除分组">
            <span class="icon">🗑️</span>
          </button>
        </div>
      </div>

      <!-- 统计摘要区 -->
      <div class="stats-summary">
        <div v-if="loading" class="loading-stats">
          <div class="loading-placeholder" />
          <div class="loading-placeholder" />
          <div class="loading-placeholder" />
          <div class="loading-placeholder" />
        </div>
        <div v-else-if="stats" class="stats-grid">
          <div class="stat-item">
            <div class="stat-value">{{ stats.active_keys }}/{{ stats.total_keys }}</div>
            <div class="stat-label">密钥数量</div>
          </div>
          <div class="stat-item">
            <div class="stat-value failure-rate">
              {{ formatPercentage(stats.failure_rate_24h) }}
            </div>
            <div class="stat-label">失败率</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ formatNumber(stats.requests_1h) }}</div>
            <div class="stat-label">近1小时</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ formatNumber(stats.requests_24h) }}</div>
            <div class="stat-label">近24小时</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ formatNumber(stats.requests_7d) }}</div>
            <div class="stat-label">近7天</div>
          </div>
        </div>
      </div>

      <!-- 详细信息区（可折叠） -->
      <div class="details-section">
        <button class="toggle-btn" @click="toggleDetails">
          <span class="toggle-text">{{ showDetails ? "收起" : "展开" }}详细信息</span>
          <span class="toggle-icon" :class="{ expanded: showDetails }">▼</span>
        </button>

        <div v-if="showDetails" class="details-content">
          <div class="detail-section">
            <h4 class="section-title">基础信息</h4>
            <div class="detail-grid">
              <div class="detail-item">
                <span class="detail-label">分组名称:</span>
                <span class="detail-value">{{ group.name }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">渠道类型:</span>
                <span class="detail-value">{{ group.channel_type }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">排序:</span>
                <span class="detail-value">{{ group.sort }}</span>
              </div>
              <div v-if="group.description" class="detail-item full-width">
                <span class="detail-label">描述:</span>
                <span class="detail-value">{{ group.description }}</span>
              </div>
            </div>
          </div>

          <div class="detail-section">
            <h4 class="section-title">上游地址</h4>
            <div class="upstream-list">
              <div v-for="(upstream, index) in group.upstreams" :key="index" class="upstream-item">
                <span class="upstream-url">{{ upstream.url }}</span>
                <span class="upstream-weight">权重: {{ upstream.weight }}</span>
              </div>
            </div>
          </div>

          <div class="detail-section">
            <h4 class="section-title">配置信息</h4>
            <div class="config-content">
              <div v-if="group.config.test_model" class="config-item">
                <span class="config-label">测试模型:</span>
                <span class="config-value">{{ group.config.test_model }}</span>
              </div>
              <div v-if="group.config.request_timeout" class="config-item">
                <span class="config-label">请求超时:</span>
                <span class="config-value">{{ group.config.request_timeout }}ms</span>
              </div>
              <div
                v-if="Object.keys(group.config.param_overrides || {}).length > 0"
                class="config-item"
              >
                <span class="config-label">参数覆盖:</span>
                <pre class="config-json">{{
                  JSON.stringify(group.config.param_overrides, null, 2)
                }}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.group-info-container {
  width: 100%;
}

.group-info-card {
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  overflow: hidden;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #e9ecef;
}

.header-left {
  flex: 1;
}

.group-title {
  font-size: 18px;
  font-weight: 600;
  color: #333;
  margin: 0 0 4px 0;
}

.group-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.channel-badge {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 3px;
  text-transform: uppercase;
}

.channel-openai {
  background: rgba(16, 163, 127, 0.1);
  color: #10a37f;
}

.channel-gemini {
  background: rgba(66, 133, 244, 0.1);
  color: #4285f4;
}

.channel-silicon {
  background: rgba(147, 51, 234, 0.1);
  color: #9333ea;
}

.channel-chutes {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.group-id {
  font-size: 12px;
  color: #6c757d;
}

.header-actions {
  display: flex;
  gap: 4px;
}

.action-btn {
  padding: 4px 6px;
  border: none;
  background: transparent;
  cursor: pointer;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.action-btn:hover {
  background: rgba(0, 123, 255, 0.1);
}

.action-btn.delete-btn:hover {
  background: rgba(239, 68, 68, 0.1);
}

.action-btn .icon {
  font-size: 14px;
}

.stats-summary {
  padding: 12px 16px;
  background: #f8f9fa;
  border-bottom: 1px solid #e9ecef;
}

.loading-stats {
  display: flex;
  gap: 12px;
}

.loading-placeholder {
  height: 40px;
  flex: 1;
  background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
  background-size: 200% 100%;
  animation: loading 1.5s infinite;
  border-radius: 4px;
}

@keyframes loading {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 12px;
}

.stat-item {
  text-align: center;
  padding: 8px;
  background: white;
  border-radius: 4px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}

.stat-value {
  font-size: 16px;
  font-weight: 700;
  color: #333;
  margin-bottom: 2px;
}

.stat-value.failure-rate {
  color: #dc3545;
}

.stat-label {
  font-size: 10px;
  color: #6c757d;
  font-weight: 500;
}

.details-section {
  border-top: 1px solid #e9ecef;
}

.toggle-btn {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 8px 16px;
  border: none;
  background: #f8f9fa;
  cursor: pointer;
  transition: background-color 0.2s;
}

.toggle-btn:hover {
  background: #e9ecef;
}

.toggle-text {
  font-size: 12px;
  font-weight: 500;
  color: #495057;
}

.toggle-icon {
  font-size: 10px;
  color: #6c757d;
  transition: transform 0.2s;
}

.toggle-icon.expanded {
  transform: rotate(180deg);
}

.details-content {
  padding: 12px 16px;
  background: white;
}

.detail-section {
  margin-bottom: 16px;
}

.detail-section:last-child {
  margin-bottom: 0;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin: 0 0 8px 0;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 8px;
}

.detail-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.detail-item.full-width {
  grid-column: 1 / -1;
  flex-direction: column;
  align-items: flex-start;
}

.detail-label {
  font-weight: 500;
  color: #6c757d;
}

.detail-value {
  color: #333;
}

.upstream-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.upstream-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 8px;
  background: #f8f9fa;
  border-radius: 4px;
  font-size: 12px;
}

.upstream-url {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  color: #333;
}

.upstream-weight {
  color: #6c757d;
  font-weight: 500;
}

.config-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.config-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
}

.config-label {
  font-weight: 500;
  color: #6c757d;
}

.config-value {
  color: #333;
}

.config-json {
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 4px;
  padding: 8px;
  font-size: 10px;
  color: #495057;
  overflow-x: auto;
  margin: 0;
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  .detail-grid {
    grid-template-columns: 1fr;
  }

  .upstream-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
}
</style>
