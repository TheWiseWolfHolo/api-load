<template>
  <el-form :inline="true" :model="filterData" class="log-filter-form">
    <el-form-item label="分组">
      <el-select v-model="filterData.group_id" placeholder="所有分组" clearable>
        <el-option v-for="group in groups" :key="group.id" :label="group.name" :value="group.id" />
      </el-select>
    </el-form-item>
    <el-form-item label="时间范围">
      <el-date-picker
        v-model="dateRange"
        type="datetimerange"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        @change="handleDateChange"
      />
    </el-form-item>
    <el-form-item label="状态码">
      <el-input v-model.number="filterData.status_code" placeholder="例如 200" clearable />
    </el-form-item>
    <el-form-item>
      <el-button type="primary" @click="applyFilters">查询</el-button>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue';
import { useLogStore } from '@/stores/logStore';
import { useGroupStore } from '@/stores/groupStore';
import { storeToRefs } from 'pinia';
import type { LogQuery } from '@/api/logs';

const logStore = useLogStore();
const groupStore = useGroupStore();
const { groups } = storeToRefs(groupStore);

const filterData = reactive<LogQuery>({});
const dateRange = ref<[Date, Date] | null>(null);

onMounted(() => {
  groupStore.fetchGroups();
});

const handleDateChange = (dates: [Date, Date] | null) => {
  if (dates) {
    filterData.start_time = dates[0].toISOString();
    filterData.end_time = dates[1].toISOString();
  } else {
    filterData.start_time = undefined;
    filterData.end_time = undefined;
  }
};

const applyFilters = () => {
  logStore.setFilters(filterData);
};
</script>

<style scoped>
.log-filter-form {
  padding: 20px;
  background-color: #f5f7fa;
  border-radius: 4px;
}
</style>