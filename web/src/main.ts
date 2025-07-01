import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './style.css'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import { useAuthStore } from './stores/authStore'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus)

// 确保认证状态在应用启动时初始化
const authStore = useAuthStore()
authStore.initializeAuth()

app.mount('#app')
