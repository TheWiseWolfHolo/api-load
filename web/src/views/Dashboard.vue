<script setup lang="ts">
import { getDashboardStats } from "@/api/dashboard";
import BaseInfoCard from "@/components/BaseInfoCard.vue";
import EncryptionMismatchAlert from "@/components/EncryptionMismatchAlert.vue";
import LineChart from "@/components/LineChart.vue";
import SecurityAlert from "@/components/SecurityAlert.vue";
import type { DashboardStatsResponse } from "@/types/models";
import { ListOutline, PulseOutline } from "@vicons/ionicons5";
import { NButton, NIcon, NSpace } from "naive-ui";
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

const dashboardStats = ref<DashboardStatsResponse | null>(null);
const { t } = useI18n();
const router = useRouter();

onMounted(async () => {
  try {
    const response = await getDashboardStats();
    dashboardStats.value = response.data;
  } catch (error) {
    console.error("Failed to load dashboard stats:", error);
  }
});
</script>

<template>
  <div class="dashboard-container">
    <n-space vertical size="large" style="gap: 0 16px">
      <header class="dashboard-intro">
        <div>
          <h1>{{ t("dashboard.providerOperations") }}</h1>
          <p>{{ t("dashboard.providerOperationsHelp") }}</p>
        </div>
        <div class="dashboard-actions">
          <n-button secondary @click="router.push('/logs')">
            <template #icon><n-icon :component="ListOutline" /></template>
            {{ t("dashboard.inspectRequests") }}
          </n-button>
          <n-button type="primary" @click="router.push('/resource-pools')">
            <template #icon><n-icon :component="PulseOutline" /></template>
            {{ t("dashboard.manageCapacity") }}
          </n-button>
        </div>
      </header>
      <!-- 加密配置错误警告（优先级最高） -->
      <encryption-mismatch-alert />

      <!-- 安全警告横幅 -->
      <security-alert
        v-if="dashboardStats?.security_warnings"
        :warnings="dashboardStats.security_warnings"
      />

      <base-info-card :stats="dashboardStats" />
      <line-chart class="dashboard-chart" />
    </n-space>
  </div>
</template>

<style scoped>
.dashboard-container {
  display: grid;
  gap: 16px;
}
.dashboard-intro,
.dashboard-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.dashboard-intro {
  justify-content: space-between;
  padding: 4px 0 8px;
}
.dashboard-intro h1 {
  margin: 0;
  color: var(--text-primary);
  font-size: 1.55rem;
  text-wrap: balance;
}
.dashboard-intro p {
  max-width: 72ch;
  margin: 5px 0 0;
  color: var(--text-secondary);
  font-size: 0.88rem;
}
.dashboard-actions {
  flex-shrink: 0;
}
@media (max-width: 720px) {
  .dashboard-intro {
    align-items: stretch;
    flex-direction: column;
  }
  .dashboard-actions > :deep(*) {
    flex: 1;
  }
}
</style>
