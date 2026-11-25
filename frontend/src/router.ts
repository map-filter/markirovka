import { createRouter, createWebHistory } from 'vue-router'
import { onUnauthorized } from '@/api/http'
import { clearSession, isAuthenticated, loadConfig, loadSession } from '@/stores/session'

const titles: Record<string, string> = {
  login: 'Вход',
  dashboard: 'Что сделать сегодня',
  products: 'Товары',
  orders: 'Заказы кодов',
  order: 'Заказ кодов',
  codes: 'Коды',
  labels: 'Этикетки',
  print: 'Печать',
  circulation: 'Ввод в оборот',
  journal: 'Журнал',
  onec: 'Выгрузка в 1С',
  settings: 'Настройки',
}

export const router = createRouter({
  history: createWebHistory('/'),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    { path: '/', name: 'dashboard', component: () => import('@/views/DashboardView.vue') },
    { path: '/products', name: 'products', component: () => import('@/views/ProductsView.vue') },
    { path: '/orders', name: 'orders', component: () => import('@/views/OrdersView.vue') },
    { path: '/orders/:id(\\d+)', name: 'order', component: () => import('@/views/OrderView.vue') },
    { path: '/codes', name: 'codes', component: () => import('@/views/CodesView.vue') },
    { path: '/labels', name: 'labels', component: () => import('@/views/LabelsView.vue') },
    { path: '/print', name: 'print', component: () => import('@/views/PrintView.vue') },
    { path: '/circulation', name: 'circulation', component: () => import('@/views/CirculationView.vue') },
    { path: '/journal', name: 'journal', component: () => import('@/views/JournalView.vue') },
    { path: '/onec', name: 'onec', component: () => import('@/views/OnecView.vue') },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
    { path: '/:pathMatch(.*)*', redirect: { name: 'dashboard' } },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) {
    await loadConfig()
    if (to.name === 'login' && isAuthenticated()) {
      try {
        await loadSession()
        return { name: 'dashboard' }
      } catch {
        clearSession()
      }
    }
    return true
  }
  if (!isAuthenticated()) {
    return { name: 'login', query: to.fullPath !== '/' ? { redirect: to.fullPath } : {} }
  }
  try {
    await loadSession()
  } catch {
    if (!isAuthenticated()) return { name: 'login' }
  }
  return true
})

router.afterEach((to) => {
  const title = titles[String(to.name ?? '')]
  document.title = title ? `${title} · Маркировка за день` : 'Маркировка за день'
})

onUnauthorized(() => {
  clearSession()
  const current = router.currentRoute.value
  if (current.name !== 'login') {
    router.replace({ name: 'login', query: current.fullPath !== '/' ? { redirect: current.fullPath } : {} })
  }
})
