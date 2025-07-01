<template>
  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
    <div
      v-for="stat in statsData"
      :key="stat.name"
      class="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-md"
    >
      <div class="flex items-center">
        <div class="flex-shrink-0">
          <component
            :is="stat.icon"
            class="h-8 w-8 text-gray-500"
            aria-hidden="true"
          />
        </div>
        <div class="ml-5 w-0 flex-1">
          <dl>
            <dt
              class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate"
            >
              {{ stat.name }}
            </dt>
            <dd class="flex items-baseline">
              <span
                class="text-2xl font-semibold text-gray-900 dark:text-white"
              >
                {{ stat.value }}
              </span>
            </dd>
          </dl>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent } from "vue";
import { storeToRefs } from "pinia";
import { useDashboardStore } from "@/stores/dashboardStore";
import { formatNumber } from "@/types/models";

const dashboardStore = useDashboardStore();
const { stats } = storeToRefs(dashboardStore);

const statsData = computed(() => [
  {
    name: "总密钥数",
    value: formatNumber(stats.value.total_keys || 0),
    icon: defineAsyncComponent(
      () => import("@heroicons/vue/24/outline/KeyIcon")
    ),
  },
  {
    name: "有效密钥数",
    value: formatNumber(stats.value.active_keys || 0),
    icon: defineAsyncComponent(
      () => import("@heroicons/vue/24/outline/CheckCircleIcon")
    ),
  },
  {
    name: "总请求数",
    value: formatNumber(stats.value.total_requests),
    icon: defineAsyncComponent(
      () => import("@heroicons/vue/24/outline/ArrowTrendingUpIcon")
    ),
  },
  {
    name: "成功率",
    value: `${(stats.value.success_rate * 100).toFixed(1)}%`,
    icon: defineAsyncComponent(
      () => import("@heroicons/vue/24/outline/ChartBarIcon")
    ),
  },
]);
</script>
