import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import LoginView from '@/views/LoginView.vue'
import { useUserStore } from '@/store/user'

const routes: Array<RouteRecordRaw> = [
  {
    path: '/login',
    name: 'Login',
    component: LoginView,
    meta: { requiresGuest: true },
  },
  {
    path: '/',
    name: 'Home',
    component: HomeView,
    meta: { requiresAuth: true },
  },
  {
    path: '/server-list',
    name: 'ServerList',
    component: () => import(/* webpackChunkName: "server-list" */ '@/views/ServerListView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/groups',
    name: 'Groups',
    component: () => import(/* webpackChunkName: "groups" */ '@/views/GroupManagementView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/policies',
    name: 'Policies',
    component: () => import(/* webpackChunkName: "policies" */ '@/views/PolicyManagementView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import(/* webpackChunkName: "settings" */ '@/views/SettingsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/about',
    name: 'About',
    component: () => import(/* webpackChunkName: "about" */ '@/views/AboutView.vue'),
    meta: { requiresAuth: true },
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

router.beforeEach((to, _from, next) => {
  const userStore = useUserStore();
  const isAuthenticated = userStore.isLoggedIn;
  console.log('[Router] beforeEach: Checking auth. Target route:', to.name, 'IsAuthenticated:', isAuthenticated);

  if (to.meta.requiresAuth && !isAuthenticated) {
    next({ name: 'Login', query: { redirect: to.fullPath } })
  } else if (to.meta.requiresGuest && isAuthenticated) {
    next({ name: 'Home' })
  } else {
    next()
  }
})

export default router
