<template>
  <el-container class="layout">
    <el-aside width="220px">
      <div class="brand">健身工作室后台</div>
      <el-menu router :default-active="$route.path" background-color="#001529" text-color="#c8c9cc" active-text-color="#00b578">
        <el-menu-item index="/dashboard">数据看板</el-menu-item>
        <el-menu-item index="/packages">课包管理</el-menu-item>
        <el-menu-item index="/coaches">教练管理</el-menu-item>
        <el-menu-item index="/coach-applications">入驻审核</el-menu-item>
        <el-menu-item index="/orders">订单管理</el-menu-item>
        <el-menu-item index="/coupons">优惠券</el-menu-item>
        <el-menu-item index="/referrals">老带新邀请</el-menu-item>
        <el-menu-item index="/group-buys">拼团管理</el-menu-item>
        <el-menu-item index="/members">学员管理</el-menu-item>
        <el-menu-item index="/reviews">评价管理</el-menu-item>
        <el-menu-item index="/qa">问答管理</el-menu-item>
        <el-menu-item v-if="profile.role === 'boss'" index="/audit-logs">操作审计</el-menu-item>
        <el-menu-item index="/schedule">预约课表</el-menu-item>
        <el-menu-item index="/settings">运营配置</el-menu-item>
        <el-menu-item v-for="m in pending" :key="m.path" :index="m.path" disabled>
          {{ m.title }}
          <el-tag size="small" type="info" style="margin-left: 8px">{{ m.milestone }}</el-tag>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div>{{ $route.meta.title || '' }}</div>
        <div class="right">
          <span class="name">{{ profile.name }}（{{ profile.role === 'boss' ? '老板' : '前台' }}）</span>
          <el-button link type="danger" @click="onLogout">退出</el-button>
        </div>
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { adminLogout } from '../api/admin'

const router = useRouter()
const profile = computed(() => JSON.parse(localStorage.getItem('admin_profile') || '{}'))
const pending = []

async function onLogout() {
  try { await adminLogout() } catch { /* 本地仍清除 */ }
  localStorage.removeItem('admin_token')
  localStorage.removeItem('admin_profile')
  router.push('/login')
}
</script>

<style scoped>
.layout {
  height: 100vh;
}

.el-aside {
  background: #001529;
}

.brand {
  height: 60px;
  line-height: 60px;
  text-align: center;
  color: #fff;
  font-weight: 600;
  letter-spacing: 1px;
}

.el-menu {
  border-right: none;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-bottom: 1px solid #eee;
}

.right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.name {
  font-size: 14px;
  color: #666;
}
</style>
