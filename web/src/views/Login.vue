<template>
  <div class="login-container">
    <n-card class="login-card" title="Login">
      <n-space vertical>
        <n-input
          v-model:value="authKey"
          type="password"
          placeholder="Auth Key"
          @keyup.enter="handleLogin"
        />
        <n-button type="primary" block @click="handleLogin" :loading="loading">Login</n-button>
      </n-space>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { authService } from "@/services/auth";
import { NButton, NCard, NInput, NSpace, useMessage } from "naive-ui";
import { ref } from "vue";
import { useRouter } from "vue-router";

const authKey = ref("");
const loading = ref(false);
const router = useRouter();
const message = useMessage();

const handleLogin = async () => {
  if (!authKey.value) {
    message.error("Please enter Auth Key");
    return;
  }
  loading.value = true;
  const success = await authService.login(authKey.value);
  loading.value = false;
  if (success) {
    router.push("/");
  } else {
    message.error("Login failed, please check your Auth Key");
  }
};
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100vh;
  background-color: #f0f2f5;
}

.login-card {
  width: 400px;
}
</style>
