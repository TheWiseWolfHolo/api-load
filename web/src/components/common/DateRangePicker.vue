<template>
  <el-date-picker
    v-model="dateRange"
    type="daterange"
    range-separator="至"
    start-placeholder="开始日期"
    end-placeholder="结束日期"
    :shortcuts="shortcuts"
    @change="handleChange"
  />
</template>

<script setup lang="ts">
import { ref, watch, defineProps, withDefaults, defineEmits } from 'vue';
import { ElDatePicker } from 'element-plus';

type DateRangeValue = [Date, Date];

const props = withDefaults(defineProps<{
  modelValue: DateRangeValue | null;
}>(), {
  modelValue: null,
});

const emit = defineEmits<{
  (e: 'update:modelValue', value: DateRangeValue | null): void;
}>();

const dateRange = ref<DateRangeValue | []>(props.modelValue || []);

watch(() => props.modelValue, (val) => {
  dateRange.value = val || [];
});

const handleChange = (value: DateRangeValue | null) => {
  emit('update:modelValue', value);
};

const shortcuts = [
  {
    text: '最近一周',
    value: () => {
      const end = new Date();
      const start = new Date();
      start.setTime(start.getTime() - 3600 * 1000 * 24 * 7);
      return [start, end];
    },
  },
  {
    text: '最近一个月',
    value: () => {
      const end = new Date();
      const start = new Date();
      start.setMonth(start.getMonth() - 1);
      return [start, end];
    },
  },
  {
    text: '最近三个月',
    value: () => {
      const end = new Date();
      const start = new Date();
      start.setMonth(start.getMonth() - 3);
      return [start, end];
    },
  },
];
</script>

<style scoped>
.el-date-picker {
  width: 100%;
}
</style>