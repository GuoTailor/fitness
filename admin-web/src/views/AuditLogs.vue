<template>
  <div>
    <el-card>
      <template #header>
        <div class="card-head">
          <span>操作审计日志（资金/敏感操作留痕，仅老板可查）</span>
          <div class="filters">
            <el-select v-model="action" size="small" clearable placeholder="全部操作" style="width: 170px" @change="onSearch">
              <el-option v-for="a in ACTIONS" :key="a.value" :label="a.label" :value="a.value" />
            </el-select>
            <el-date-picker v-model="range" type="daterange" size="small" value-format="YYYY-MM-DD"
                            start-placeholder="开始日期" end-placeholder="结束日期" style="margin-left: 12px; width: 240px"
                            @change="onSearch" />
          </div>
        </div>
      </template>
      <el-table :data="list" v-loading="loading">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="时间" width="160">
          <template #default="{ row }">{{ fmtTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作人" width="130">
          <template #default="{ row }">{{ row.username || ('#' + row.adminId) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="140">
          <template #default="{ row }">
            <el-tag size="small" :type="tagOf(row.action)">{{ labelOf(row.action) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="对象" width="160">
          <template #default="{ row }">{{ row.targetType }}#{{ row.targetId }}</template>
        </el-table-column>
        <el-table-column prop="detail" label="详情" min-width="320" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP" width="130" />
      </el-table>
      <el-pagination layout="prev, pager, next, total" :total="total" :page-size="pageSize"
                     :current-page="page" @current-change="onPage" style="margin-top: 12px" />
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { listAuditLogs } from '../api/admin'

const ACTIONS = [
  { value: 'refund.approve', label: '退款通过', tag: 'danger' },
  { value: 'refund.reject', label: '退款拒绝', tag: 'warning' },
  { value: 'member.adjust', label: '调整课时', tag: 'warning' },
  { value: 'coupon.save', label: '保存券模板', tag: 'primary' },
  { value: 'coupon.toggle', label: '券停发/恢复', tag: 'info' },
  { value: 'coupon.grant', label: '定向发券', tag: 'primary' },
  { value: 'package.save', label: '保存课包', tag: 'primary' },
  { value: 'coach.verify', label: '教练资质审核', tag: 'info' },
  { value: 'review.audit', label: '评价审核', tag: 'info' },
  { value: 'config.save', label: '保存配置', tag: 'info' },
  { value: 'booking.proxy', label: '代客预约', tag: 'info' },
  { value: 'verify.proxy', label: '代客核销', tag: 'info' },
]

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const action = ref('')
const range = ref([])
const loading = ref(false)

const dateFrom = computed(() => (range.value && range.value[0]) || '')
const dateTo = computed(() => (range.value && range.value[1]) || '')

const labelOf = (a) => ACTIONS.find((x) => x.value === a)?.label || a
const tagOf = (a) => ACTIONS.find((x) => x.value === a)?.tag || 'info'
const fmtTime = (t) => String(t || '').replace('T', ' ').slice(0, 19)

async function load() {
  loading.value = true
  try {
    const data = await listAuditLogs({
      page: page.value, pageSize, action: action.value,
      dateFrom: dateFrom.value, dateTo: dateTo.value,
    })
    list.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}
function onSearch() { page.value = 1; load() }
function onPage(p) { page.value = p; load() }

onMounted(load)
</script>

<style scoped>
.card-head { display: flex; justify-content: space-between; align-items: center; }
.filters { display: flex; align-items: center; }
</style>
