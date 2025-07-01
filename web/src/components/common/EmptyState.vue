<template>
  <div class="empty-state-container">
    <el-empty :description="description">
      <template #image>
        <slot name="image">
          <img v-if="image" :src="image" alt="Empty state" />
        </slot>
      </template>
      <template #default>
        <slot name="actions">
          <el-button v-if="actionText" type="primary" @click="$emit('action')">
            {{ actionText }}
          </el-button>
        </slot>
      </template>
    </el-empty>
  </div>
</template>

<script setup lang="ts">
import { defineProps, withDefaults, defineEmits } from 'vue';
import { ElEmpty, ElButton } from 'element-plus';

withDefaults(defineProps<{
  image?: string;
  description?: string;
  actionText?: string;
}>(), {
  image: '',
  description: '暂无数据',
  actionText: '',
});

defineEmits<{
  (e: 'action'): void;
}>();
</script>

<style scoped>
.empty-state-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  width: 100%;
  padding: 40px 0;
}
.el-empty__image img {
  max-width: 150px;
  user-select: none;
}
</style>