import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/Login.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      name: 'dashboard',
      component: () => import('../views/Dashboard.vue'),
    },
    {
      path: '/consumers',
      name: 'consumers',
      component: () => import('../views/Consumers.vue'),
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
      path: '/plugins',
      name: 'plugins',
      component: () => import('../views/Plugins.vue'),
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/Settings.vue'),
    },
  ],
})

// 路由守卫：未登录跳转登录页
router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('agw_token')
  if (!to.meta.public && !token) {
    next({ name: 'login' })
  } else if (to.meta.public && token && to.name === 'login') {
    next({ name: 'dashboard' })
  } else {
    next()
  }
})

export default router
