<template>
  <el-card>
    <div class="toolbar">
      <el-date-picker v-model="date" type="date" value-format="YYYY-MM-DD" :clearable="false" @change="load" />
      <el-button @click="shift(-1)">前一天</el-button>
      <el-button @click="today">今天</el-button>
      <el-button @click="shift(1)">后一天</el-button>
      <div class="spacer" />
      <el-button type="primary" @click="verifyDialog = true">代客核销</el-button>
    </div>

    <el-row :gutter="16">
      <el-col :span="12">
        <h4>私教预约（{{ bookings.length }}）</h4>
        <el-table :data="bookings" v-loading="loading" size="small">
          <el-table-column label="时间" width="120">
            <template #default="{ row }">{{ row.startLabel }}</template>
          </el-table-column>
          <el-table-column prop="coachName" label="教练" width="90" />
          <el-table-column label="学员" min-width="120">
            <template #default="{ row }">{{ row.userName }}<span class="sub">{{ row.userPhone }}</span></template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="statusType(row.booking.status)">{{ statusText(row.booking.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="70">
            <template #default="{ row }">
              <el-button v-if="row.booking.status !== 'completed'" link type="danger" size="small" @click="cancel(row)">取消</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-col>
      <el-col :span="12">
        <h4>团课（{{ groupClasses.length }}）</h4>
        <el-table :data="groupClasses" v-loading="loading" size="small">
          <el-table-column label="时间" width="120">
            <template #default="{ row }">{{ row.startLabel }}</template>
          </el-table-column>
          <el-table-column prop="class.title" label="团课" min-width="120" />
          <el-table-column prop="coachName" label="教练" width="90" />
          <el-table-column label="报名" width="80">
            <template #default="{ row }">{{ row.class.enrolledCount }}/{{ row.class.capacity }}</template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{ row }">{{ row.class.status }}</template>
          </el-table-column>
        </el-table>
      </el-col>
    </el-row>

    <el-dialog v-model="verifyDialog" title="代客核销（前台）" width="400px">
      <el-input v-model="verifyCode" placeholder="输入学员出示的核销码" size="large" @keyup.enter="doVerify" />
      <div v-if="verifyResult" class="verify-result" :class="verifyResult.alreadyVerified ? 'warn' : 'ok'">
        {{ verifyResult.alreadyVerified ? '该码已核销过（幂等）' : '核销成功' }}：
        {{ verifyResult.bizType === 'private' ? '私教课' : '团课' }} · {{ verifyResult.userName }}
      </div>
      <template #footer>
        <el-button @click="verifyDialog = false">关闭</el-button>
        <el-button type="primary" @click="doVerify">核销</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import http from '../api/request'

const date = ref(new Date().toISOString().slice(0, 10))
const bookings = ref([])
const groupClasses = ref([])
const loading = ref(false)
const verifyDialog = ref(false)
const verifyCode = ref('')
const verifyResult = ref(null)

const statusText = (s) => ({ pending: '待确认', confirmed: '已确认', completed: '已完成', cancelled: '已取消', expired: '已过期' }[s] || s)
const statusType = (s) => ({ pending: 'warning', confirmed: 'success', completed: 'info' }[s] || 'info')

async function load() {
  loading.value = true
  try {
    const data = await http.get('/admin/api/v1/schedule', { params: { date: date.value } })
    bookings.value = data.bookings
    groupClasses.value = data.groupClasses
  } finally {
    loading.value = false
  }
}

function shift(n) {
  const d = new Date(date.value)
  d.setDate(d.getDate() + n)
  date.value = d.toISOString().slice(0, 10)
  load()
}
function today() {
  date.value = new Date().toISOString().slice(0, 10)
  load()
}

async function cancel(row) {
  await ElMessageBox.confirm('取消后将退还学员课时并释放时段，继续？', '取消预约')
  await http.post(`/admin/api/v1/bookings/${row.booking.id}/cancel`)
  ElMessage.success('已取消')
  load()
}

async function doVerify() {
  if (!verifyCode.value) return
  verifyResult.value = await http.post('/admin/api/v1/verify', { code: verifyCode.value.trim() })
  verifyCode.value = ''
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; gap: 12px; margin-bottom: 16px; align-items: center; }
.spacer { flex: 1; }
.sub { display: block; font-size: 12px; color: #999; }
.verify-result { margin-top: 12px; padding: 10px; border-radius: 6px; }
.verify-result.ok { background: #f0faf6; color: #00b578; }
.verify-result.warn { background: #fdf6ec; color: #e6a23c; }
</style>
