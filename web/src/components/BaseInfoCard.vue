<script setup lang="ts">
import type { DashboardStatsResponse } from "@/types/models";
import {
  KeyOutline,
  ShieldCheckmarkOutline,
  SpeedometerOutline,
  StatsChartOutline,
} from "@vicons/ionicons5";
import { NCard, NGrid, NGridItem, NIcon, NSpace, NTag, NTooltip } from "naive-ui";
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n();

// Props
interface Props {
  stats: DashboardStatsResponse | null;
  loading?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
});

// 使用计算属性代替ref
const stats = computed(() => props.stats);
const animatedValues = ref<Record<string, number>>({});

// 格式化数值显示
const formatValue = (value: number, type: "count" | "rate" = "count"): string => {
  if (type === "rate") {
    return `${value.toFixed(1)}%`;
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)}K`;
  }
  return value.toString();
};

// 格式化趋势显示
const formatTrend = (trend: number): string => {
  const sign = trend >= 0 ? "+" : "";
  return `${sign}${trend.toFixed(1)}%`;
};

// 监听stats变化并更新动画值
const updateAnimatedValues = () => {
  if (stats.value) {
    setTimeout(() => {
      animatedValues.value = {
        key_count:
          (stats.value?.key_count?.value ?? 0) /
          ((stats.value?.key_count?.value ?? 1) + (stats.value?.key_count?.sub_value ?? 1)),
        rpm: Math.min(100 + (stats.value?.rpm?.trend ?? 0), 100) / 100,
        request_count: Math.min(100 + (stats.value?.request_count?.trend ?? 0), 100) / 100,
        error_rate: (100 - (stats.value?.error_rate?.value ?? 0)) / 100,
      };
    }, 0);
  }
};

// 监听stats变化
onMounted(() => {
  updateAnimatedValues();
});
</script>

<template>
  <div class="stats-container">
    <n-space vertical size="medium">
      <n-grid cols="2 s:4" :x-gap="20" :y-gap="20" responsive="screen">
        <!-- 密钥数量 -->
        <n-grid-item span="1">
          <n-card :bordered="false" class="stat-card" style="animation-delay: 0s">
            <div class="stat-header">
              <div class="stat-icon key-icon">
                <n-icon :component="KeyOutline" />
              </div>
              <n-tooltip v-if="stats?.key_count.sub_value" trigger="hover">
                <template #trigger>
                  <n-tag type="error" size="small" class="stat-trend">
                    {{ stats.key_count.sub_value }}
                  </n-tag>
                </template>
                {{ stats.key_count.sub_value_tip }}
              </n-tooltip>
            </div>

            <div class="stat-content">
              <div class="stat-value">
                {{ stats?.key_count?.value ?? 0 }}
              </div>
              <div class="stat-title">{{ t("dashboard.totalKeys") }}</div>
            </div>

            <div class="stat-bar">
              <div
                class="stat-bar-fill key-bar"
                :style="{
                  width: `${(animatedValues.key_count ?? 0) * 100}%`,
                }"
              />
            </div>
          </n-card>
        </n-grid-item>

        <!-- RPM (10分钟) -->
        <n-grid-item span="1">
          <n-card :bordered="false" class="stat-card" style="animation-delay: 0.05s">
            <div class="stat-header">
              <div class="stat-icon rpm-icon">
                <n-icon :component="SpeedometerOutline" />
              </div>
              <n-tag
                v-if="stats?.rpm && stats.rpm.trend !== undefined"
                :type="stats?.rpm.trend_is_growth ? 'success' : 'error'"
                size="small"
                class="stat-trend"
              >
                {{ stats ? formatTrend(stats.rpm.trend) : "--" }}
              </n-tag>
            </div>

            <div class="stat-content">
              <div class="stat-value">
                {{ stats?.rpm?.value.toFixed(1) ?? 0 }}
              </div>
              <div class="stat-title">{{ t("dashboard.rpm10Min") }}</div>
            </div>

            <div class="stat-bar">
              <div
                class="stat-bar-fill rpm-bar"
                :style="{
                  width: `${(animatedValues.rpm ?? 0) * 100}%`,
                }"
              />
            </div>
          </n-card>
        </n-grid-item>

        <!-- 24小时请求 -->
        <n-grid-item span="1">
          <n-card :bordered="false" class="stat-card" style="animation-delay: 0.1s">
            <div class="stat-header">
              <div class="stat-icon request-icon">
                <n-icon :component="StatsChartOutline" />
              </div>
              <n-tag
                v-if="stats?.request_count && stats.request_count.trend !== undefined"
                :type="stats?.request_count.trend_is_growth ? 'success' : 'error'"
                size="small"
                class="stat-trend"
              >
                {{ stats ? formatTrend(stats.request_count.trend) : "--" }}
              </n-tag>
            </div>

            <div class="stat-content">
              <div class="stat-value">
                {{ stats ? formatValue(stats.request_count.value) : "--" }}
              </div>
              <div class="stat-title">{{ t("dashboard.requests24h") }}</div>
            </div>

            <div class="stat-bar">
              <div
                class="stat-bar-fill request-bar"
                :style="{
                  width: `${(animatedValues.request_count ?? 0) * 100}%`,
                }"
              />
            </div>
          </n-card>
        </n-grid-item>

        <!-- 24小时错误率 -->
        <n-grid-item span="1">
          <n-card :bordered="false" class="stat-card" style="animation-delay: 0.15s">
            <div class="stat-header">
              <div class="stat-icon error-icon">
                <n-icon :component="ShieldCheckmarkOutline" />
              </div>
              <n-tag
                v-if="stats?.error_rate.trend !== 0"
                :type="stats?.error_rate.trend_is_growth ? 'success' : 'error'"
                size="small"
                class="stat-trend"
              >
                {{ stats ? formatTrend(stats.error_rate.trend) : "--" }}
              </n-tag>
            </div>

            <div class="stat-content">
              <div class="stat-value">
                {{ stats ? formatValue(stats.error_rate.value ?? 0, "rate") : "--" }}
              </div>
              <div class="stat-title">{{ t("dashboard.errorRate24h") }}</div>
            </div>

            <div class="stat-bar">
              <div
                class="stat-bar-fill error-bar"
                :style="{
                  width: `${(animatedValues.error_rate ?? 0) * 100}%`,
                }"
              />
            </div>
          </n-card>
        </n-grid-item>
      </n-grid>
    </n-space>
  </div>
</template>

<style scoped>
.stats-container {
  width: 100%;
  animation: fadeInUp 0.2s ease-out;
  margin-bottom: 16px;
}

.stat-card {
  background: var(--card-bg-solid);
  border-radius: var(--border-radius-lg);
  border: 1px solid var(--border-color-light);
  position: relative;
  overflow: hidden;
  animation: slideInUp 0.2s ease-out both;
  transition: all 0.2s ease;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.stat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--border-radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.35rem;
  border: 1px solid transparent;
}

.key-icon {
  background: rgba(193, 95, 60, 0.08);
  border-color: rgba(193, 95, 60, 0.18);
  color: var(--primary-color);
}

.rpm-icon {
  background: rgba(84, 112, 131, 0.08);
  border-color: rgba(84, 112, 131, 0.18);
  color: var(--metric-info);
}

.request-icon {
  background: rgba(67, 132, 92, 0.08);
  border-color: rgba(67, 132, 92, 0.18);
  color: var(--metric-success);
}

.error-icon {
  background: rgba(183, 78, 73, 0.08);
  border-color: rgba(183, 78, 73, 0.18);
  color: var(--metric-error);
}

.stat-trend {
  font-weight: 600;
}

.stat-trend:before {
  content: "";
  display: inline-block;
  width: 0;
  height: 0;
  margin-right: 4px;
  vertical-align: middle;
}

.stat-content {
  margin-bottom: 16px;
}

.stat-value {
  font-family: var(--font-display);
  font-size: 1.9rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
  color: var(--text-primary);
  margin-bottom: 4px;
}

.stat-title {
  font-size: 0.95rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.stat-bar {
  width: 100%;
  height: 4px;
  background: var(--border-color);
  border-radius: 2px;
  overflow: hidden;
  position: relative;
}

.stat-bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.5s ease-out;
  transition-delay: 0.2s;
}

.key-bar {
  background: var(--primary-color);
}

.rpm-bar {
  background: var(--metric-info);
}

.request-bar {
  background: var(--metric-success);
}

.error-bar {
  background: var(--metric-error);
}

@keyframes slideInUp {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
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
:deep(.n-grid-item) {
  min-width: 0;
}
</style>
