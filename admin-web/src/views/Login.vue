<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <h2 class="title">健身工作室 · 管理后台</h2>
      <el-form :model="form" @keyup.enter="onSubmit">
        <el-form-item>
          <el-input v-model="form.username" placeholder="用户名" size="large" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" placeholder="密码" size="large" show-password />
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loading" @click="onSubmit">
          登 录
        </el-button>
      </el-form>
      <p class="tip">首次启动默认账号 admin / fitness123（可在环境变量 ADMIN_INIT_PASSWORD 指定）</p>
    </el-card>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { login } from '../api/auth'

const router = useRouter()
const form = reactive({ username: 'admin', password: '' })
const loading = ref(false)

async function onSubmit() {
  if (!form.username || !form.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    const data = await login({ username: form.username, password: form.password })
    localStorage.setItem('admin_token', data.token)
    localStorage.setItem('admin_profile', JSON.stringify(data.admin))
    router.push('/')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #00b578 0%, #00694a 100%);
}

.login-card {
  width: 380px;
  padding: 8px 12px;
}

.title {
  text-align: center;
  margin-bottom: 24px;
  color: #222;
}

.tip {
  margin-top: 12px;
  font-size: 12px;
  color: #999;
  text-align: center;
}
</style>
