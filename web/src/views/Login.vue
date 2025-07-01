<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <div class="login-header">
          <h2>GPT Load - 登录</h2>
        </div>
      </template>
      
      <el-form
        ref="loginFormRef"
        :model="loginForm"
        :rules="loginRules"
        class="login-form"
        @submit.prevent="handleLogin"
      >
        <el-form-item prop="authKey">
          <el-input
            v-model="loginForm.authKey"
            type="password"
            placeholder="请输入认证密钥"
            size="large"
            show-password
            clearable
            @keyup.enter="handleLogin"
          >
            <template #prefix>
              <el-icon><Key /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        
        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            @click="handleLogin"
            class="login-button"
          >
            {{ loading ? '登录中...' : '登录' }}
          </el-button>
        </el-form-item>
      </el-form>
      
      <el-alert
        v-if="errorMessage"
        :title="errorMessage"
        type="error"
        :closable="false"
        class="error-alert"
      />
    </el-card>
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { Key } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/authStore'
import { login as loginAPI } from '../api/auth'

const router = useRouter()
const authStore = useAuthStore()

const loginFormRef = ref<FormInstance>()
const loading = ref(false)
const errorMessage = ref('')

const loginForm = reactive({
  authKey: ''
})

const loginRules = {
  authKey: [
    { required: true, message: '请输入认证密钥', trigger: 'blur' },
    { min: 1, message: '认证密钥不能为空', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  if (!loginFormRef.value) return
  
  try {
    const valid = await loginFormRef.value.validate()
    if (!valid) return
    
    loading.value = true
    errorMessage.value = ''
    
    // 调用后端API进行认证
    const response = await loginAPI(loginForm.authKey)
    
    if (response.success) {
      // 认证成功，保存认证密钥
      authStore.login(loginForm.authKey)
      
      ElMessage.success('登录成功')
      
      // 获取重定向路径，默认跳转到仪表盘
      const redirect = router.currentRoute.value.query.redirect as string
      await router.push(redirect || '/dashboard')
    } else {
      // 认证失败，显示错误信息
      errorMessage.value = response.message || '认证失败，请检查认证密钥'
    }
    
  } catch (error: any) {
    console.error('Login error:', error)
    
    // 处理网络错误和API错误
    if (error.response?.status === 401) {
      errorMessage.value = '认证密钥错误，请重新输入'
    } else if (error.response?.data?.message) {
      errorMessage.value = error.response.data.message
    } else if (error.message) {
      errorMessage.value = `登录失败: ${error.message}`
    } else {
      errorMessage.value = '登录失败，请检查网络连接'
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px;
}

.login-card {
  width: 100%;
  max-width: 400px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.1);
  border-radius: 12px;
}

.login-header {
  text-align: center;
}

.login-header h2 {
  margin: 0;
  color: #303133;
  font-weight: 600;
}

.login-form {
  padding: 0 20px;
}

.login-button {
  width: 100%;
  height: 45px;
  font-size: 16px;
  font-weight: 500;
}

.error-alert {
  margin-top: 16px;
}

:deep(.el-card__header) {
  padding: 20px 20px 10px 20px;
  border-bottom: 1px solid #ebeef5;
}

:deep(.el-card__body) {
  padding: 30px 20px 20px 20px;
}

:deep(.el-input__inner) {
  height: 45px;
  font-size: 14px;
}

:deep(.el-form-item) {
  margin-bottom: 20px;
}

:deep(.el-form-item:last-child) {
  margin-bottom: 0;
}
</style>