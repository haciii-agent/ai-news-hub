import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      component: () => import('@/views/Home.vue'),
    },
    {
      path: '/article/:id',
      component: () => import('@/views/Article.vue'),
    },
    {
      path: '/login',
      component: () => import('@/views/Login.vue'),
    },
    {
      path: '/profile',
      component: () => import('@/views/Profile.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/bookmarks',
      component: () => import('@/views/Bookmarks.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/history',
      component: () => import('@/views/History.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/recommendations',
      component: () => import('@/views/Recommendations.vue'),
    },
    {
      path: '/dashboard',
      component: () => import('@/views/Dashboard.vue'),
    },
    {
      path: '/trends',
      component: () => import('@/views/Trends.vue'),
    },
    {
      path: '/admin',
      component: () => import('@/views/Admin.vue'),
      meta: { requiresAuth: true, requiresAdmin: true },
    },
  ],
})

router.beforeEach(async (to, _from, next) => {
  const auth = useAuthStore()
  auth.init()

  if (to.meta.requiresAuth && !auth.loggedIn) {
    localStorage.setItem('login_redirect', to.fullPath)
    return next('/login')
  }

  if (to.meta.requiresAdmin) {
    if (!auth.loggedIn || auth.currentUser?.role !== 'admin') {
      return next('/')
    }
  }

  next()
})

export default router
