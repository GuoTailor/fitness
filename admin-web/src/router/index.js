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
      { path: 'coach-applications', component: () => import('../views/CoachApplications.vue'), meta: { title: '入驻审核' } },
      { path: 'orders', component: () => import('../views/Orders.vue'), meta: { title: '订单管理' } },
      { path: 'coupons', component: () => import('../views/Coupons.vue'), meta: { title: '优惠券' } },
      { path: 'referrals', component: () => import('../views/Referrals.vue'), meta: { title: '老带新邀请' } },
      { path: 'group-buys', component: () => import('../views/GroupBuys.vue'), meta: { title: '拼团管理' } },
      { path: 'group-buy-activities', component: () => import('../views/GroupBuyActivities.vue'), meta: { title: '拼团活动' } },
      { path: 'members', component: () => import('../views/Members.vue'), meta: { title: '学员管理' } },
      { path: 'reviews', component: () => import('../views/Reviews.vue'), meta: { title: '评价管理' } },
      { path: 'qa', component: () => import('../views/Qa.vue'), meta: { title: '问答管理' } },
      { path: 'audit-logs', component: () => import('../views/AuditLogs.vue'), meta: { title: '操作审计' } },
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
