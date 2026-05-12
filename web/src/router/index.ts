import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // === 公开页面 ===
    {
      path: '/',
      name: 'home',
      component: () => import('../views/Home.vue'),
      meta: { public: true },
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/Login.vue'),
      meta: { public: true },
    },
    {
      path: '/docs',
      name: 'docs',
      component: () => import('../views/Docs.vue'),
      meta: { public: true },
    },
    {
      path: '/about',
      name: 'about',
      component: () => import('../views/About.vue'),
      meta: { public: true },
    },
    // === 控制台（需登录） ===
    {
      path: '/console',
      name: 'dashboard',
      component: () => import('../views/Dashboard.vue'),
    },
    {
      path: '/keys',
      name: 'keys',
      component: () => import('../views/KeysList.vue'),
    },
    {
      path: '/channels',
      name: 'channels',
      component: () => import('../views/Channels.vue'),
    },
    {
      path: '/groups',
      name: 'groups',
      component: () => import('../views/Groups.vue'),
    },
    {
      path: '/stats',
      name: 'stats',
      component: () => import('../views/Stats.vue'),
    },
    {
      path: '/logs',
      name: 'logs',
      component: () => import('../views/Logs.vue'),
    },
    {
      path: '/models',
      name: 'models',
      component: () => import('../views/Models.vue'),
    },
    {
      path: '/plugins',
      name: 'plugins',
      component: () => import('../views/Plugins.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/Settings.vue'),
    },
    {
      path: '/settings/logs',
      name: 'system-logs',
      component: () => import('../views/SystemLogs.vue'),
    },
    {
      path: '/settings/monitor',
      name: 'system-monitor',
      component: () => import('../views/SystemMonitor.vue'),
    },
  ],
})

// 路由守卫：未登录跳转登录页
router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('agw_token')
  // 公开页面直接放行
  if (to.meta.public) {
    next()
    return
  }
  // 需要登录但没有 token → 跳转登录
  if (!token) {
    next({ name: 'login' })
    return
  }
  next()
})

export default router