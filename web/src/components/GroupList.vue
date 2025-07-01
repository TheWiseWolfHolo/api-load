<template>
  <div class="group-list" v-loading="groupStore.isLoading">
    <el-menu
      :default-active="groupStore.selectedGroupId?.toString() || undefined"
      @select="handleSelect"
    >
      <el-menu-item
        v-for="group in groupStore.groups"
        :key="group.id"
        :index="group.id.toString()"
      >
        <template #title>
          <span>{{ group.name }}</span>
          <el-tag v-if="group.is_default" size="small" style="margin-left: 8px"
            >默认</el-tag
          >
        </template>
      </el-menu-item>
    </el-menu>
    <div
      v-if="!groupStore.isLoading && groupStore.groups.length === 0"
      class="empty-state"
    >
      暂无分组
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue";
import { useGroupStore } from "@/stores/groupStore";
import { ElMenu, ElMenuItem, ElTag, vLoading } from "element-plus";

const groupStore = useGroupStore();

onMounted(() => {
  // 组件挂载时获取分组数据
  if (groupStore.groups.length === 0) {
    groupStore.fetchGroups();
  }
});

const handleSelect = (index: string) => {
  groupStore.selectGroup(Number(index));
};
</script>

<style scoped>
.group-list {
  border-right: 1px solid var(--el-border-color);
  height: 100%;
}
.el-menu {
  border-right: none;
}
.empty-state {
  text-align: center;
  color: var(--el-text-color-secondary);
  padding-top: 20px;
}
</style>
