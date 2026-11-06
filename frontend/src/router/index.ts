import { createRouter, createWebHistory, type RouteLocationNormalized } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    title?: string
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/Login.vue'),
      meta: {
        requiresAuth: false,
        title: 'Login',
      },
    },
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      meta: {
        requiresAuth: true,
      },
      children: [
        { path: '', name: 'dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: 'Dashboard' } },
        { path: 'containers', name: 'containers', component: () => import('@/views/Containers.vue'), meta: { title: 'Containers' } },
        { path: 'images', name: 'images', component: () => import('@/views/Images.vue'), meta: { title: 'Images' } },
        { path: 'networks', name: 'networks', component: () => import('@/views/Networks.vue'), meta: { title: 'Networks' } },
        { path: 'compose', name: 'compose', component: () => import('@/views/Compose.vue'), meta: { title: 'Compose' } },
        { path: 'nodes', name: 'nodes', component: () => import('@/views/Nodes.vue'), meta: { title: 'Nodes' } },
        { path: 'volumes', name: 'volumes', component: () => import('@/views/Volumes.vue'), meta: { title: 'Volumes' } },
        { path: 'registry', name: 'registry', component: () => import('@/views/Registry.vue'), meta: { title: 'Registry' } },
        { path: 'docker-config', name: 'dockerConfig', component: () => import('@/views/DockerConfig.vue'), meta: { title: 'Docker Config' } },
        { path: 'agent', name: 'agent', component: () => import('@/views/AgentChat.vue'), meta: { title: 'Agent' } },
        { path: 'settings/agent', name: 'agentSettings', component: () => import('@/views/AgentSettings.vue'), meta: { title: 'Agent Settings' } },
        { path: 'settings/notify', name: 'notifySettings', component: () => import('@/views/NotifySettings.vue'), meta: { title: 'Notify Binding' } },
        { path: 'settings/update', name: 'updateSettings', component: () => import('@/views/UpdateSettings.vue'), meta: { title: 'Update' } },
      ],
    },
  ],
})

router.beforeEach(async (to: RouteLocationNormalized) => {
  const authStore = useAuthStore()
  const requiresAuth = to.matched.some(record => record.meta.requiresAuth !== false)

  if (!requiresAuth || to.meta.requiresAuth === false) {
    if (to.name === 'login' && authStore.isAuthenticated) {
      return { name: 'dashboard' }
    }
    return true
  }

  if (!authStore.isAuthenticated) {
    return {
      name: 'login',
      query: { redirect: to.fullPath },
    }
  }

  if (!authStore.username) {
    const isValid = await authStore.checkAuth()
    if (!isValid) {
      return {
        name: 'login',
        query: { redirect: to.fullPath },
      }
    }
  }

  return true
})

router.afterEach((to: RouteLocationNormalized) => {
  const title = to.meta.title
  document.title = title ? `${title} - Rabbit Panel` : 'Rabbit Panel'
})

export default router
