<template>
  <div class="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-md">
    <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">快捷操作</h3>
    <div class="flex space-x-4">
      <button
        @click="onAddKey"
        class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
      >
        <PlusIcon class="-ml-1 mr-2 h-5 w-5" aria-hidden="true" />
        添加密钥
      </button>
      <button
        @click="onCreateGroup"
        class="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md shadow-sm text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
      >
        <PlusCircleIcon class="-ml-1 mr-2 h-5 w-5 text-gray-400" aria-hidden="true" />
        创建分组
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
import { PlusIcon, PlusCircleIcon } from '@heroicons/vue/24/solid';

const router = useRouter();

const onAddKey = () => {
  // Assuming you have a route for adding a key, possibly on the keys page with a modal
  router.push({ name: 'keys', query: { action: 'add' } });
};

const onCreateGroup = () => {
  // Assuming you have a route for groups where a creation modal can be triggered
  router.push({ name: 'groups', query: { action: 'create' } });
};
</script>