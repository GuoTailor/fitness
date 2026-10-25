<template>
  <el-card>
    <template #header>
      <div class="card-head">
        <span>教练入驻审核（通过后自动建档并绑定申请人小程序账号）</span>
        <el-select v-model="status" style="width: 130px" @change="onSearch">
          <el-option label="待审核" :value="0" />
          <el-option label="已通过" :value="1" />
          <el-option label="已驳回" :value="2" />
          <el-option label="全部" :value="-1" />
        </el-select>
      </div>
    </template>
    <el-table :data="list" v-loading="loading">
      <el-table-column label="ID" width="70">
        <template #default="{ row }">{{ row.application.id }}</template>
      </el-table-column>
      <el-table-column label="申请人" min-width="130">
        <template #default="{ row }">{{ row.application.name }}（{{ row.userNickname || '#' + row.application.userId }}）</template>
      </el-table-column>
      <el-table-column label="联系方式" width="130">
        <template #default="{ row }">{{ maskPhone(row.application.phone) || '—' }}</template>
      </el-table-column>
      <el-table-column prop="application.years" label="年限" width="70" />
      <el-table-column prop="application.specialties" label="擅长" min-width="120" />
      <el-table-column label="资质" min-width="180">
        <template #default="{ row }">
          <el-image v-for="(img, i) in imgs(row)" :key="i" :src="img" :preview-src-list="imgs(row)"
                    fit="cover" style="width: 44px; height: 44px; border-radius: 4px; margin-right: 6px" />
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.application.status === 1 ? 'success' : (row.application.status === 2 ? 'danger' : 'warning')">
            {{ statusText(row.application.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="申请时间" width="170">
        <template #default="{ row }">{{ fmt(row.application.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <template v-if="row.application.status === 0">
            <el-button link type="success" @click="onApprove(row)">通过</el-button>
            <el-button link type="danger" @click="onReject(row)">驳回</el-button>
          </template>
          <span v-else-if="row.application.status === 2" class="reason">{{ row.application.rejectReason }}</span>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      style="margin-top: 12px; justify-content: flex-end"
      layout="total, prev, pager, next"
      :total="total"
      :page-size="pageSize"
      :current-page="page"
      @current-change="load"
    />
  </el-card>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listCoachApplications, approveCoachApplication, rejectCoachApplication } from '../api/admin'
import { maskPhone } from '../utils/phone'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const status = ref(0)
const loading = ref(false)

const fmt = (t) => (t ? String(t).replace('T', ' ').slice(0, 16) : '')
const statusText = (s) => ({ 0: '待审核', 1: '已通过', 2: '已驳回' }[s] || s)
const imgs = (row) => {
  try { return JSON.parse(row.application.certImages || '[]') } catch { return [] }
}

async function load(p = page.value) {
  page.value = p
  loading.value = true
  try {
    const data = await listCoachApplications({ page: p, pageSize, status: status.value })
    list.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}
function onSearch() { load(1) }

async function onApprove(row) {
  await ElMessageBox.confirm(
    `确认通过「${row.application.name}」的入驻申请？将自动创建教练档案并绑定其小程序账号。`,
    '通过入驻申请', { type: 'warning' }
  )
  await approveCoachApplication(row.application.id)
  ElMessage.success('已通过')
  load()
}

async function onReject(row) {
  const { value } = await ElMessageBox.prompt('请填写驳回原因（将通知申请人）', '驳回申请', {
    inputPlaceholder: '如：资质材料不清晰',
  })
  await rejectCoachApplication(row.application.id, value || '')
  ElMessage.success('已驳回')
  load()
}

onMounted(() => load(1))
</script>

<style scoped>
.card-head { display: flex; justify-content: space-between; align-items: center; }
.reason { font-size: 12px; color: #f56c6c; }
</style>
