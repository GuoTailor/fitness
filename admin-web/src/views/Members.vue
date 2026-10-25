<template>
  <div>
    <el-card>
      <div class="toolbar">
        <el-input v-model="keyword" placeholder="昵称/手机号" style="width: 220px" clearable @keyup.enter="load" />
        <el-button type="primary" @click="load">搜索</el-button>
      </div>
      <el-table :data="list" v-loading="loading">
        <el-table-column prop="user.id" label="ID" width="60" />
        <el-table-column label="昵称" min-width="120">
          <template #default="{ row }">{{ row.user.nickname || '—' }}</template>
        </el-table-column>
        <el-table-column label="手机号" width="130">
          <template #default="{ row }">{{ row.user.phone ? maskPhone(row.user.phone) : '未绑定' }}</template>
        </el-table-column>
        <el-table-column prop="packageCount" label="持有课包" width="90" />
        <el-table-column prop="remainingSessions" label="剩余课时" width="90" />
        <el-table-column prop="paidOrderCount" label="已购订单" width="90" />
        <el-table-column label="注册时间" width="170">
          <template #default="{ row }">{{ fmtTime(row.user.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openPkgs(row.user.id)">课时管理</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination layout="total, prev, pager, next" :total="total" :page-size="pageSize"
        v-model:current-page="page" @current-change="load" style="margin-top: 12px" />
    </el-card>

    <el-drawer v-model="drawer" title="课时管理" size="560px">
      <el-table :data="pkgs">
        <el-table-column prop="packageTitle" label="课包" min-width="140" />
        <el-table-column label="剩余/总课时" width="110">
          <template #default="{ row }">{{ row.totalSessions - row.usedSessions }}/{{ row.totalSessions }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="80" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :disabled="row.status === 'refunded'" @click="openAdjust(row)">调整</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-drawer>

    <el-dialog v-model="adjustDialog" title="手动调整课时（赠课/扣课）" width="420px">
      <el-form label-width="80px">
        <el-form-item label="调整值">
          <el-input-number v-model="adjustForm.change" :step="1" />
          <div class="sub">正数赠课，负数扣课</div>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="adjustForm.remark" placeholder="如：开业赠送 / 补课" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="adjustDialog = false">取消</el-button>
        <el-button type="primary" @click="doAdjust">确认调整</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listMembers, memberPackages, adjustSessions } from '../api/admin'
import { maskPhone } from '../utils/phone'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const keyword = ref('')
const loading = ref(false)
const drawer = ref(false)
const pkgs = ref([])
const adjustDialog = ref(false)
const adjustForm = reactive({ mpId: 0, userId: 0, change: 1, remark: '', clientKey: '' })

const fmtTime = (t) => (t ? String(t).replace('T', ' ').slice(0, 19) : '—')

async function load() {
  loading.value = true
  try {
    const data = await listMembers({ page: page.value, page_size: pageSize, keyword: keyword.value })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function openPkgs(uid) {
  pkgs.value = await memberPackages(uid)
  drawer.value = true
}

function openAdjust(row) {
  adjustForm.mpId = row.id
  adjustForm.userId = row.userId
  adjustForm.change = 1
  adjustForm.remark = ''
  adjustForm.clientKey = 'adj-' + Date.now() + '-' + Math.random().toString(36).slice(2, 10)
  adjustDialog.value = true
}

async function doAdjust() {
  if (!adjustForm.change) return ElMessage.warning('调整值不能为 0')
  await adjustSessions(adjustForm.userId, adjustForm.mpId, {
    change: adjustForm.change, remark: adjustForm.remark || '后台调整', clientKey: adjustForm.clientKey,
  })
  ElMessage.success('已调整')
  adjustDialog.value = false
  pkgs.value = await memberPackages(adjustForm.userId)
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; gap: 12px; margin-bottom: 12px; }
.sub { font-size: 12px; color: #999; }
</style>
