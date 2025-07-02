<script setup lang="ts">
import { useAuthService } from "@/services/auth";
import { useRouter } from "vue-router";

const router = useRouter();
const { logout } = useAuthService();

const handleLogout = () => {
  logout();
  router.replace("/login");
};
</script>

<template>
  <n-button quaternary round @click="handleLogout">退出</n-button>
</template>
