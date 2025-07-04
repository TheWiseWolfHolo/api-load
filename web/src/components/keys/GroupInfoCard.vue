<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group, GroupStats } from "@/types/models";
import {
  NButton,
  NCard,
  NCollapse,
  NCollapseItem,
  NDescriptions,
  NDescriptionsItem,
  NGrid,
  NGridItem,
  NSpin,
  NTag,
  useMessage,
} from "naive-ui";
import { onMounted, ref, watch } from "vue";

interface Props {
  group: Group | null;
}

const props = defineProps<Props>();

const stats = ref<GroupStats | null>(null);
const loading = ref(false);
const message = useMessage();

onMounted(() => {
  loadStats();
});

watch(
  () => props.group,
  () => {
    loadStats();
  }
);

async function loadStats() {
  if (!props.group) {
    stats.value = null;
    return;
  }

  try {
    loading.value = true;
    stats.value = await keysApi.getGroupStats(props.group.id);
  } catch (_error) {
    // 错误已记录
  } finally {
    loading.value = false;
  }
}

function handleEdit() {
  message.info("编辑分组功能开发中...");
}

function handleDelete() {
  message.info("删除分组功能开发中...");
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

function copyUrl(url: string) {
  navigator.clipboard
    .writeText(url)
    .then(() => {
      window.$message.success("地址已复制到剪贴板");
    })
    .catch(() => {
      window.$message.error("复制失败");
    });
}
</script>

<template>
  <div class="group-info-container">
    <n-card :bordered="false" class="group-info-card">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <h3 class="group-title">
              {{ group?.display_name || group?.name || "请选择分组" }}
              <code
                v-if="group"
                class="group-url"
                @click="copyUrl(`https://gpt-load.com/${group?.name}`)"
              >
                https://gpt-load.com/{{ group?.name }}
              </code>
            </h3>
          </div>
          <div class="header-actions">
            <n-button quaternary circle size="small" @click="handleEdit" title="编辑分组">
              <template #icon>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                  <path
                    d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"
                  />
                </svg>
              </template>
            </n-button>
            <n-button
              quaternary
              circle
              size="small"
              @click="handleDelete"
              title="删除分组"
              type="error"
            >
              <template #icon>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                  <path
                    d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"
                  />
                </svg>
              </template>
            </n-button>
          </div>
        </div>
      </template>

      <!-- 统计摘要区 -->
      <div class="stats-summary">
        <n-spin :show="loading" size="small">
          <n-grid :cols="5" :x-gap="12" :y-gap="12" responsive="screen">
            <n-grid-item span="1">
              <n-card
                :title="`${stats?.active_keys || 0} / ${stats?.total_keys || 0}`"
                size="large"
              >
                <template #header-extra><span class="status-title">密钥数量</span></template>
              </n-card>
            </n-grid-item>
            <n-grid-item span="1">
              <n-card
                class="status-card-failure"
                :title="formatPercentage(stats?.failure_rate_24h || 0)"
                size="large"
              >
                <template #header-extra><span class="status-title">失败率</span></template>
              </n-card>
            </n-grid-item>
            <n-grid-item span="1">
              <n-card :title="formatNumber(stats?.requests_1h || 0)" size="large">
                <template #header-extra><span class="status-title">近1小时</span></template>
              </n-card>
            </n-grid-item>
            <n-grid-item span="1">
              <n-card :title="formatNumber(stats?.requests_24h || 0)" size="large">
                <template #header-extra><span class="status-title">近24小时</span></template>
              </n-card>
            </n-grid-item>
            <n-grid-item span="1">
              <n-card :title="formatNumber(stats?.requests_7d || 0)" size="large">
                <template #header-extra><span class="status-title">近7天</span></template>
              </n-card>
            </n-grid-item>
          </n-grid>
        </n-spin>
      </div>

      <!-- 详细信息区（可折叠） -->
      <div class="details-section">
        <n-collapse>
          <n-collapse-item title="详细信息" name="details">
            <div class="details-content">
              <div class="detail-section">
                <h4 class="section-title">基础信息</h4>
                <n-descriptions :column="2" size="small">
                  <n-descriptions-item label="分组名称">
                    {{ group?.name || "-" }}
                  </n-descriptions-item>
                  <n-descriptions-item label="渠道类型">
                    {{ group?.channel_type || "openai" }}
                  </n-descriptions-item>
                  <n-descriptions-item label="排序">{{ group?.sort || 0 }}</n-descriptions-item>
                  <n-descriptions-item v-if="group?.description || ''" label="描述" :span="2">
                    {{ group?.description || "" }}
                  </n-descriptions-item>
                </n-descriptions>
              </div>

              <div class="detail-section">
                <h4 class="section-title">上游地址</h4>
                <n-descriptions :column="1" size="small">
                  <n-descriptions-item
                    v-for="(upstream, index) in group?.upstreams ?? []"
                    :key="index"
                    :label="`上游 ${index + 1}`"
                  >
                    <span class="upstream-url">{{ upstream.url }}</span>
                    <n-tag size="small" type="info" class="upstream-weight">
                      权重: {{ upstream.weight }}
                    </n-tag>
                  </n-descriptions-item>
                </n-descriptions>
              </div>

              <div class="detail-section">
                <h4 class="section-title">配置信息</h4>
                <n-descriptions :column="2" size="small">
                  <n-descriptions-item v-if="group?.config?.test_model || ''" label="测试模型">
                    {{ group?.config?.test_model || "" }}
                  </n-descriptions-item>
                  <n-descriptions-item v-if="group?.config?.request_timeout || 0" label="请求超时">
                    {{ group?.config?.request_timeout || 0 }}ms
                  </n-descriptions-item>
                  <n-descriptions-item
                    v-if="Object.keys(group?.config?.param_overrides || {}).length > 0"
                    label="参数覆盖"
                    :span="2"
                  >
                    <pre class="config-json">{{
                      JSON.stringify(group?.config?.param_overrides || "", null, 2)
                    }}</pre>
                  </n-descriptions-item>
                </n-descriptions>
              </div>
            </div>
          </n-collapse-item>
        </n-collapse>
      </div>
    </n-card>
  </div>
</template>

<style scoped>
.group-info-container {
  width: 100%;
}

:deep(.n-card-header) {
  padding: 12px 24px;
}

.group-info-card {
  background: rgba(255, 255, 255, 0.98);
  border-radius: var(--border-radius-lg);
  border: 1px solid rgba(255, 255, 255, 0.3);
  animation: fadeInUp 0.2s ease-out;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.header-left {
  flex: 1;
}

.group-title {
  font-size: 1.2rem;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 8px 0;
}

.group-url {
  font-size: 0.8rem;
  color: #2563eb;
  margin-left: 8px;
  font-family: monospace;
  background: rgba(37, 99, 235, 0.1);
  border-radius: 4px;
  padding: 2px 6px;
  margin-right: 4px;
}

/* .group-meta {
  display: flex;
  align-items: center;
  gap: 8px;
} */

.group-id {
  font-size: 0.75rem;
  color: #64748b;
  opacity: 0.7;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.stats-summary {
  margin-bottom: 12px;
  text-align: center;
}

:deep(.status-card-failure .n-card-header__main) {
  color: #d03050;
}

.status-title {
  color: #64748b;
  font-size: 12px;
}

.details-section {
  margin-top: 12px;
}

.details-content {
  margin-top: 12px;
}

.detail-section {
  margin-bottom: 24px;
}

.detail-section:last-child {
  margin-bottom: 0;
}

.section-title {
  font-size: 1rem;
  font-weight: 600;
  color: #374151;
  margin: 0 0 12px 0;
  padding-bottom: 8px;
  border-bottom: 2px solid rgba(102, 126, 234, 0.1);
}

.upstream-url {
  font-family: monospace;
  font-size: 0.9rem;
  color: #374151;
  margin-right: 8px;
}

.upstream-weight {
  margin-left: 8px;
}

.config-json {
  background: rgba(102, 126, 234, 0.05);
  border-radius: var(--border-radius-sm);
  padding: 12px;
  font-size: 0.8rem;
  color: #374151;
  margin: 8px 0;
  overflow-x: auto;
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

/* 响应式网格 */
:deep(.n-grid) {
  gap: 8px;
}

:deep(.n-grid-item) {
  min-width: 0;
}

@media (max-width: 768px) {
  :deep(.n-grid) {
    grid-template-columns: repeat(2, 1fr);
  }

  .group-title {
    font-size: 1rem;
  }

  .section-title {
    font-size: 0.9rem;
  }
}

@media (max-width: 480px) {
  :deep(.n-grid) {
    grid-template-columns: 1fr;
  }

  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .header-actions {
    align-self: flex-end;
  }
}
</style>
