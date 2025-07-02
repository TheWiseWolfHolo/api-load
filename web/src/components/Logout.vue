<template>
  <n-button quaternary round @click="handleLogout">退出</n-button>
</template>

<script setup lang="ts">
import { authService } from "@/services/auth";
import { useRouter } from "vue-router";

const router = useRouter();

const handleLogout = () => {
  authService.logout();
  router.push("/login");
};
</script>
