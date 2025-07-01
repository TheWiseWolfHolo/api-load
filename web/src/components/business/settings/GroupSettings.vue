<template>
  <div class="p-6 bg-white shadow-md rounded-lg">
    <h3 class="text-lg font-semibold leading-6 text-gray-900 mb-6">分组设置</h3>
    <div class="mb-4">
      <label for="group-select" class="block text-sm font-medium text-gray-700"
        >选择分组</label
      >
      <select
        id="group-select"
        v-model="selectedGroup"
        class="mt-1 block w-full pl-3 pr-10 py-2 text-base border-gray-300 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm rounded-md"
      >
        <option v-for="group in groups" :key="group.id" :value="group.id">
          {{ group.name }}
        </option>
      </select>
    </div>

    <div v-if="selectedGroup">
      <p class="text-sm text-gray-600">
        为
        <strong>{{ selectedGroupName }}</strong>
        分组设置覆盖配置。这些配置将优先于系统默认配置。
      </p>
      <!-- Add group-specific setting items here later -->
      <div class="mt-4 p-4 border border-dashed rounded-md">
        <p class="text-center text-gray-500">分组配置项待实现。</p>
      </div>
    </div>
    <div v-else>
      <p class="text-center text-gray-500">请先选择一个分组以查看其配置。</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useGroupStore } from "@/stores/groupStore";
import { storeToRefs } from "pinia";

const groupStore = useGroupStore();
const { groups } = storeToRefs(groupStore);

const selectedGroup = ref<number | null>(null);

const selectedGroupName = computed(() => {
  return groups.value.find((g) => g.id === selectedGroup.value)?.name || "";
});

onMounted(() => {
  groupStore.fetchGroups();
});
</script>
