<template>
  <div class="setting-item mb-4">
    <label class="block text-sm font-medium text-gray-700">{{ label }}</label>
    <input
      :type="type"
      :value="modelValue"
      @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      class="mt-1 block w-full px-3 py-2 bg-white border border-gray-300 rounded-md shadow-sm placeholder-gray-400 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
      :placeholder="placeholder"
    />
    <p v-if="description" class="mt-2 text-sm text-gray-500">{{ description }}</p>
    <p v-if="error" class="mt-1 text-sm text-red-600">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  modelValue: string | number;
  label: string;
  type?: string;
  placeholder?: string;
  description?: string;
  error?: string;
}>();

defineEmits(['update:modelValue']);
</script>

<style scoped>
.setting-item {
  /* Add any specific styling for the setting item here */
}
</style>