<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { TaskInfo } from "@/types/models";
import { onBeforeUnmount, onMounted, ref } from "vue";

const taskInfo = ref<TaskInfo>({ is_running: false });
const visible = ref(false);
let pollTimer: number | null = null;

onMounted(() => {
  startPolling();
});

onBeforeUnmount(() => {
  stopPolling();
});

function startPolling() {
  stopPolling();
  pollTimer = setInterval(async () => {
    try {
      const task = await keysApi.getTaskStatus();
      taskInfo.value = task;
      visible.value = task.is_running;
    } catch (error) {
      console.error("获取任务状态失败:", error);
    }
  }, 1000);
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

function getProgressPercentage(): number {
  if (!taskInfo.value.total || taskInfo.value.total === 0) {
    return 0;
  }
  return Math.round(((taskInfo.value.processed || 0) / taskInfo.value.total) * 100);
}

function getProgressText(): string {
  const { processed = 0, total = 0 } = taskInfo.value;
  return `${processed}/${total}`;
}

function handleClose() {
  visible.value = false;
}
</script>

<template>
  <div v-if="visible" class="global-task-progress">
    <div class="progress-container">
      <div class="progress-header">
        <div class="progress-info">
          <span class="progress-icon">⚡</span>
          <div class="progress-details">
            <div class="progress-title">{{ taskInfo.task_name || "正在处理任务" }}</div>
            <div class="progress-subtitle">
              {{ getProgressText() }} ({{ getProgressPercentage() }}%)
            </div>
          </div>
        </div>
        <button @click="handleClose" class="close-btn" title="隐藏进度条">✕</button>
      </div>

      <div class="progress-bar-container">
        <div class="progress-bar" :style="{ width: `${getProgressPercentage()}%` }" />
      </div>

      <div v-if="taskInfo.message" class="progress-message">
        {{ taskInfo.message }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.global-task-progress {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 9999;
  width: 360px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
  border: 1px solid #e1e5e9;
  animation: slideIn 0.3s ease-out;
}

@keyframes slideIn {
  from {
    transform: translateX(100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

.progress-container {
  padding: 16px;
}

.progress-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.progress-info {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
}

.progress-icon {
  font-size: 20px;
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.1);
  }
}

.progress-details {
  flex: 1;
}

.progress-title {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin-bottom: 2px;
}

.progress-subtitle {
  font-size: 12px;
  color: #666;
}

.close-btn {
  width: 24px;
  height: 24px;
  border: none;
  background: none;
  cursor: pointer;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #999;
  font-size: 14px;
  transition: all 0.2s;
}

.close-btn:hover {
  background: #f5f5f5;
  color: #666;
}

.progress-bar-container {
  width: 100%;
  height: 6px;
  background: #f0f0f0;
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 8px;
}

.progress-bar {
  height: 100%;
  background: linear-gradient(90deg, #4ade80, #22c55e);
  border-radius: 3px;
  transition: width 0.3s ease;
  position: relative;
}

.progress-bar::after {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0;
  right: 0;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.3), transparent);
  animation: shimmer 2s infinite;
}

@keyframes shimmer {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(100%);
  }
}

.progress-message {
  font-size: 12px;
  color: #666;
  text-align: center;
  padding: 8px;
  background: #f8f9fa;
  border-radius: 4px;
  margin-top: 8px;
}

@media (max-width: 768px) {
  .global-task-progress {
    left: 20px;
    right: 20px;
    width: auto;
    top: 10px;
  }
}
</style>
