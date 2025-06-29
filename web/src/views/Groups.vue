<template>
  <div class="groups-view">
    <el-row :gutter="20" class="main-layout">
      <el-col :span="6" class="left-panel">
        <group-list />
      </el-col>
      <el-col :span="18" class="right-panel">
        <div class="config-section">
          <group-config-form />
        </div>
        <div class="keys-section">
          <key-table />
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import GroupList from '@/components/GroupList.vue';
import GroupConfigForm from '@/components/GroupConfigForm.vue';
import KeyTable from '@/components/KeyTable.vue';
import { ElRow, ElCol } from 'element-plus';
</script>

<style scoped>
.groups-view {
  height: 100%;
  padding: 20px;
  box-sizing: border-box;
}

.main-layout, .left-panel, .right-panel {
  height: 100%;
}

.left-panel {
  background-color: #fff;
  border-radius: 4px;
  overflow-y: auto;
}

.right-panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.config-section, .keys-section {
  background-color: #fff;
  border-radius: 4px;
}
</style>