<script setup lang="ts">
import Layout from "@/components/Layout.vue";
import { NDialogProvider, NMessageProvider } from "naive-ui";
</script>

<template>
  <n-message-provider>
    <n-dialog-provider>
      <layout />
    </n-dialog-provider>
  </n-message-provider>
</template>
