import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "../stores/authStore";
import Dashboard from "../views/Dashboard.vue";
import Login from "../views/Login.vue";
import MainLayout from "../layouts/MainLayout.vue";

const routes = [
  {
    path: "/login",
    name: "Login",
    component: Login,
    meta: { requiresAuth: false },
  },
  {
    path: "/",
    component: MainLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: "",
        redirect: "/dashboard",
      },
      {
        path: "/dashboard",
        name: "Dashboard",
        component: Dashboard,
        meta: { requiresAuth: true },
      },
      {
        path: "/groups",
        name: "Groups",
        component: () => import("../views/Groups.vue"),
        meta: { requiresAuth: true },
      },
      {
        path: "/logs",
        name: "Logs",
        component: () => import("../views/Logs.vue"),
        meta: { requiresAuth: true },
      },
      {
        path: "/settings",
        name: "Settings",
        component: () => import("../views/Settings.vue"),
        meta: { requiresAuth: true },
      },
    ],
  },
];

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
});

// 路由守卫
router.beforeEach((to, _from, next) => {
  const authStore = useAuthStore();
  const isAuthenticated = authStore.isAuthenticated;
  const requiresAuth = to.matched.some(
    (record) => record.meta.requiresAuth !== false
  );

  if (requiresAuth && !isAuthenticated) {
    // 需要认证但未登录，重定向到登录页
    next({
      name: "Login",
      query: { redirect: to.fullPath },
    });
  } else if (to.name === "Login" && isAuthenticated) {
    // 已登录用户访问登录页，重定向到仪表盘
    next({ name: "Dashboard" });
  } else {
    // 正常访问
    next();
  }
});

export default router;
