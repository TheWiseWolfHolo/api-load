import App from "@/App.vue";
import "@/assets/style.css";
import router from "@/router";
import naive from "naive-ui";
import { createApp } from "vue";

createApp(App).use(router).use(naive).mount("#app");
