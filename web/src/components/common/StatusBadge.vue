<template>
  <el-tag :type="tagType" effect="light" round>
    {{ statusText }}
  </el-tag>
</template>

<script setup lang="ts">
import { computed, defineProps, withDefaults } from "vue";
import { ElTag } from "element-plus";

type APIKeyStatus = "active" | "inactive" | "error";

const props = withDefaults(
  defineProps<{
    status: APIKeyStatus;
    statusMap?: Record<APIKeyStatus, string>;
  }>(),
  {
    status: "inactive",
    statusMap: () => ({
      active: "启用",
      inactive: "禁用",
      error: "错误",
    }),
  }
);

const tagType = computed(() => {
  switch (props.status) {
    case "active":
      return "success";
    case "inactive":
      return "warning";
    case "error":
      return "danger";
    default:
      return "info";
  }
});

const statusText = computed(() => {
  return props.statusMap[props.status] || "未知";
});
</script>

<style scoped>
.el-tag {
  cursor: default;
}
</style>
