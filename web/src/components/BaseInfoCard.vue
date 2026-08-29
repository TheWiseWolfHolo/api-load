<script setup lang="ts">
import type { DashboardStatsResponse, ProviderCapacity } from "@/types/models";
import {
  CheckmarkCircleOutline,
  GitNetworkOutline,
  LayersOutline,
  LinkOutline,
  PauseCircleOutline,
  RepeatOutline,
  WarningOutline,
} from "@vicons/ionicons5";
import { NIcon, NProgress, NSkeleton, NTag } from "naive-ui";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{ stats: DashboardStatsResponse | null }>();
const { t } = useI18n();

const capacity = computed<ProviderCapacity>(
  () =>
    props.stats?.provider_capacity ?? {
      total_credentials: 0,
      ready_credentials: 0,
      auto_disabled_credentials: 0,
      paused_credentials: 0,
      cooling_credentials: 0,
      resource_pools: 0,
      protocol_endpoints: 0,
      pool_bound_routes: 0,
      retry_attempts_24h: 0,
      retry_rate_24h: 0,
    }
);
const readiness = computed(() => {
  if (capacity.value.total_credentials === 0) {
    return 0;
  }
  return (
    Math.round((capacity.value.ready_credentials / capacity.value.total_credentials) * 1000) / 10
  );
});
const metrics = computed(() => [
  {
    key: "ready",
    label: t("dashboard.readyCredentials"),
    value: capacity.value.ready_credentials,
    help: t("dashboard.readyCredentialsHelp"),
    icon: CheckmarkCircleOutline,
    tone: "success",
  },
  {
    key: "invalid",
    label: t("dashboard.autoDisabledCredentials"),
    value: capacity.value.auto_disabled_credentials,
    help: t("dashboard.autoDisabledCredentialsHelp"),
    icon: WarningOutline,
    tone: "error",
  },
  {
    key: "paused",
    label: t("dashboard.pausedCredentials"),
    value: capacity.value.paused_credentials,
    help: t("dashboard.pausedCredentialsHelp"),
    icon: PauseCircleOutline,
    tone: "warning",
  },
  {
    key: "retry",
    label: t("dashboard.retryPressure"),
    value: `${capacity.value.retry_rate_24h.toFixed(1)}%`,
    help: t("dashboard.retryPressureHelp", { count: capacity.value.retry_attempts_24h }),
    icon: RepeatOutline,
    tone: "info",
  },
]);
const routeFacts = computed(() => [
  {
    key: "pools",
    icon: LayersOutline,
    label: t("dashboard.resourcePools"),
    value: capacity.value.resource_pools,
  },
  {
    key: "endpoints",
    icon: LinkOutline,
    label: t("dashboard.protocolEndpoints"),
    value: capacity.value.protocol_endpoints,
  },
  {
    key: "routes",
    icon: GitNetworkOutline,
    label: t("dashboard.poolBoundRoutes"),
    value: capacity.value.pool_bound_routes,
  },
  {
    key: "cooling",
    icon: RepeatOutline,
    label: t("dashboard.coolingCredentials"),
    value: capacity.value.cooling_credentials,
  },
]);
</script>

<template>
  <section class="capacity-panel">
    <header class="capacity-header">
      <div>
        <h2>{{ t("dashboard.providerCapacity") }}</h2>
        <p>{{ t("dashboard.providerCapacityHelp") }}</p>
      </div>
      <n-tag size="small" :bordered="false">
        {{ t("dashboard.totalCredentials", { count: capacity.total_credentials }) }}
      </n-tag>
    </header>

    <template v-if="stats">
      <div class="capacity-grid">
        <article v-for="metric in metrics" :key="metric.key" class="capacity-metric">
          <div class="metric-icon" :class="metric.tone">
            <n-icon :component="metric.icon" />
          </div>
          <div>
            <strong>{{ metric.value }}</strong>
            <span>{{ metric.label }}</span>
            <small>{{ metric.help }}</small>
          </div>
        </article>
      </div>

      <div class="readiness-row">
        <div>
          <strong>{{ t("dashboard.capacityReadiness") }}</strong>
          <span>{{ readiness.toFixed(1) }}%</span>
        </div>
        <n-progress
          type="line"
          :percentage="readiness"
          :show-indicator="false"
          :height="6"
          color="var(--metric-success)"
          rail-color="var(--bg-tertiary)"
        />
      </div>

      <div class="route-facts">
        <div v-for="fact in routeFacts" :key="fact.key">
          <n-icon :component="fact.icon" />
          <span>{{ fact.label }}</span>
          <strong>{{ fact.value }}</strong>
        </div>
      </div>
    </template>
    <div v-else class="capacity-loading">
      <n-skeleton v-for="index in 4" :key="index" height="96px" />
    </div>
  </section>
</template>

<style scoped>
.capacity-panel {
  overflow: hidden;
  background: var(--card-bg-solid);
  border: 1px solid var(--border-color-light);
  border-radius: var(--border-radius-lg);
}
.capacity-header,
.capacity-grid,
.readiness-row,
.route-facts,
.capacity-loading {
  padding: 18px 20px;
}
.capacity-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  border-bottom: 1px solid var(--border-color-light);
}
.capacity-header h2 {
  margin: 0;
  color: var(--text-primary);
  font-size: 1.05rem;
}
.capacity-header p {
  max-width: 70ch;
  margin: 4px 0 0;
  color: var(--text-secondary);
  font-size: 0.82rem;
}
.capacity-grid,
.capacity-loading {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0;
}
.capacity-metric {
  display: flex;
  gap: 12px;
  min-width: 0;
  padding: 2px 18px;
  border-right: 1px solid var(--border-color-light);
}
.capacity-metric:first-child {
  padding-left: 0;
}
.capacity-metric:last-child {
  padding-right: 0;
  border-right: 0;
}
.metric-icon {
  display: grid;
  flex: 0 0 34px;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: var(--border-radius-sm);
  background: var(--bg-secondary);
  color: var(--text-secondary);
}
.metric-icon.success {
  color: var(--metric-success);
}
.metric-icon.error {
  color: var(--metric-error);
}
.metric-icon.warning {
  color: var(--metric-warning);
}
.metric-icon.info {
  color: var(--metric-info);
}
.capacity-metric > div:last-child {
  display: grid;
  min-width: 0;
}
.capacity-metric strong {
  color: var(--text-primary);
  font-size: 1.45rem;
  font-variant-numeric: tabular-nums;
  line-height: 1.1;
}
.capacity-metric span {
  margin-top: 4px;
  color: var(--text-primary);
  font-size: 0.82rem;
  font-weight: 600;
}
.capacity-metric small {
  margin-top: 3px;
  color: var(--text-secondary);
  font-size: 0.72rem;
  line-height: 1.4;
}
.readiness-row {
  display: grid;
  grid-template-columns: 210px minmax(0, 1fr);
  gap: 20px;
  align-items: center;
  border-top: 1px solid var(--border-color-light);
}
.readiness-row > div {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: var(--text-primary);
  font-size: 0.8rem;
}
.readiness-row span {
  color: var(--metric-success);
  font-variant-numeric: tabular-nums;
}
.route-facts {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 24px;
  background: var(--bg-secondary);
  border-top: 1px solid var(--border-color-light);
}
.route-facts > div {
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--text-secondary);
  font-size: 0.78rem;
}
.route-facts strong {
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}
@media (max-width: 900px) {
  .capacity-grid,
  .capacity-loading {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px 0;
  }
  .capacity-metric:nth-child(2) {
    border-right: 0;
  }
  .capacity-metric:nth-child(3) {
    padding-left: 0;
  }
}
@media (max-width: 560px) {
  .capacity-header {
    flex-direction: column;
  }
  .capacity-grid,
  .capacity-loading {
    grid-template-columns: 1fr;
  }
  .capacity-metric,
  .capacity-metric:nth-child(3) {
    padding: 0 0 14px;
    border-right: 0;
    border-bottom: 1px solid var(--border-color-light);
  }
  .capacity-metric:last-child {
    padding-bottom: 0;
    border-bottom: 0;
  }
  .readiness-row {
    grid-template-columns: 1fr;
    gap: 10px;
  }
}
</style>
