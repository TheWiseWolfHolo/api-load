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
  <n-button quaternary round class="logout-button" @click="handleLogout">
    <template #icon>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
        <path
          d="M17 7l-1.41 1.41L18.17 11H8v2h10.17l-2.58 2.59L17 17l5-5zM4 5h8V3H4c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h8v-2H4V5z"
        />
      </svg>
    </template>
    退出登录
  </n-button>
</template>

<style scoped>
.logout-button {
  color: #64748b;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  font-weight: 500;
  letter-spacing: 0.2px;
}

.logout-button:hover {
  color: #dc2626;
  background: rgba(239, 68, 68, 0.1);
  border-color: rgba(239, 68, 68, 0.2);
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
}

:deep(.n-button__content) {
  gap: 6px;
}
</style>
