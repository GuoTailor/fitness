import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/login', component: () => import('../views/Login.vue') },
  {
    path: '/',
    component: () => import('../views/Layout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '数据看板' } },
      { path: 'packages', component: () => import('../views/Packages.vue'), meta: { title: '课包管理' } },
      { path: 'coaches', component: () => import('../views/Coaches.vue'), meta: { title: '教练管理' } },
      { path: 'orders', component: () => import('../views/Orders.vue'), meta: { title: '订单管理' } },
      { path: 'members', component: () => import('../views/Members.vue'), meta: { title: '学员管理' } },
      { path: 'reviews', component: () => import('../views/Reviews.vue'), meta: { title: '评价管理' } },
      { path: 'schedule', component: () => import('../views/Schedule.vue'), meta: { title: '预约课表' } },
      { path: 'settings', component: () => import('../views/Settings.vue'), meta: { title: '运营配置' } },
    ],
  },
]

const router = createRouter({ history: createWebHashHistory(), routes })

router.beforeEach((to) => {
  if (to.path !== '/login' && !localStorage.getItem('admin_token')) {
    return '/login'
  }
})

export default router
