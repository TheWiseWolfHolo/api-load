<template>
  <el-dialog
    v-model="dialogVisible"
    :title="title"
    width="30%"
    :before-close="handleClose"
    center
  >
    <div class="dialog-content">
      <el-icon :class="['icon', type]">
        <WarningFilled v-if="type === 'warning'" />
        <CircleCloseFilled v-if="type === 'delete'" />
        <InfoFilled v-if="type === 'info'" />
      </el-icon>
      <span>{{ content }}</span>
    </div>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="handleCancel">{{ cancelText }}</el-button>
        <el-button :type="confirmButtonType" @click="handleConfirm">
          {{ confirmText }}
        </el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue';
import { ElDialog, ElButton, ElIcon } from 'element-plus';
import { WarningFilled, CircleCloseFilled, InfoFilled } from '@element-plus/icons-vue';

type DialogType = 'warning' | 'delete' | 'info';

const props = withDefaults(defineProps<{
  visible: boolean;
  title: string;
  content: string;
  type?: DialogType;
  confirmText?: string;
  cancelText?: string;
}>(), {
  visible: false,
  type: 'warning',
  confirmText: '确认',
  cancelText: '取消',
});

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void;
  (e: 'confirm'): void;
  (e: 'cancel'): void;
}>();

const dialogVisible = ref(props.visible);

watch(() => props.visible, (val) => {
  dialogVisible.value = val;
});

const confirmButtonType = computed(() => {
  switch (props.type) {
    case 'delete':
      return 'danger';
    case 'warning':
      return 'warning';
    default:
      return 'primary';
  }
});

const handleClose = (done: () => void) => {
  emit('update:visible', false);
  emit('cancel');
  done();
};

const handleConfirm = () => {
  emit('confirm');
  emit('update:visible', false);
};

const handleCancel = () => {
  emit('cancel');
  emit('update:visible', false);
};
</script>

<style scoped>
.dialog-content {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
}

.icon {
  font-size: 24px;
}

.icon.warning {
  color: #E6A23C;
}

.icon.delete {
  color: #F56C6C;
}

.icon.info {
  color: #909399;
}
</style>