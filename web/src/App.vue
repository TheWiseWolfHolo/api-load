<script setup lang="ts">
import Layout from "@/components/Layout.vue";
import { useAuthKey } from "@/services/auth";
import { NDialogProvider, NMessageProvider } from "naive-ui";
import { computed } from "vue";

const authKey = useAuthKey();
const isLoggedIn = computed(() => !!authKey.value);
</script>

<template>
  <n-message-provider>
    <n-dialog-provider>
      <layout v-if="isLoggedIn" />
      <router-view v-else />
    </n-dialog-provider>
  </n-message-provider>
</template>
