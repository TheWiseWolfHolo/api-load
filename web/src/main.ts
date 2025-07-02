import naive from 'naive-ui'
import { createApp } from 'vue'
import App from './App.vue'
import './style.css'
import router from './utils/router'

createApp(App).use(router).use(naive).mount('#app')
