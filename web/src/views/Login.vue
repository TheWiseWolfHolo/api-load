<script setup lang="ts">
import { useAuthService } from "@/services/auth";
import { NButton, NCard, NInput, NSpace, useMessage } from "naive-ui";
import { ref } from "vue";
import { useRouter } from "vue-router";

const authKey = ref("");
const loading = ref(false);
const router = useRouter();
const message = useMessage();
const { login } = useAuthService();

const handleLogin = async () => {
  if (!authKey.value) {
    message.error("Please enter Auth Key");
    return;
  }
  loading.value = true;
  const success = await login(authKey.value);
  loading.value = false;
  if (success) {
    router.push("/");
  } else {
    message.error("Login failed, please check your Auth Key");
  }
};
</script>

<template>
  <div class="login-container">
    <n-card class="login-card" title="登录">
      <n-space vertical>
        <n-input
          v-model:value="authKey"
          type="password"
          placeholder="Auth Key"
          @keyup.enter="handleLogin"
        />
        <n-button class="login-btn" type="primary" block @click="handleLogin" :loading="loading">
          Login
        </n-button>
      </n-space>
    </n-card>
  </div>
</template>

<style scoped>
.login-container {
  height: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
}

.login-card {
  max-width: 400px;
}

.login-btn {
  margin-top: 10px;
}
</style>
