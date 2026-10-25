<template>
  <el-card>
    <template #header>
      <div class="card-head">
        <span>拼团管理（到期未成团自动原路退款，退款单见订单管理）</span>
        <el-select v-model="status" style="width: 130px" @change="onSearch">
          <el-option label="全部" :value="0" />
          <el-option label="报名中" :value="1" />
          <el-option label="已成团" :value="2" />
          <el-option label="未成团" :value="3" />
        </el-select>
      </div>
    </template>
    <el-table :data="list" v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="活动" min-width="120">
        <template #default="{ row }">
          <span v-if="row.activityTitle">{{ row.activityTitle }}</span>
          <span v-else style="color: #999">SKU 常开</span>
        </template>
      </el-table-column>
      <el-table-column label="课包/规格" min-width="160">
        <template #default="{ row }">SKU#{{ row.skuId }} · 课包#{{ row.packageId }}</template>
      </el-table-column>
      <el-table-column label="拼团价" width="100">
        <template #default="{ row }">¥{{ (row.price / 100).toFixed(0) }}</template>
      </el-table-column>
      <el-table-column label="进度" width="100">
        <template #default="{ row }">{{ row.paidCount }}/{{ row.capacity }}</template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 2 ? 'success' : (row.status === 3 ? 'danger' : 'info')">{{ row.statusText }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="开团人" width="90">
        <template #default="{ row }">#{{ row.leaderUserId }}</template>
      </el-table-column>
      <el-table-column label="截止" width="170">
        <template #default="{ row }">{{ fmt(row.expireAt) }}</template>
      </el-table-column>
      <el-table-column label="成团时间" width="170">
        <template #default="{ row }">{{ row.successAt ? fmt(row.successAt) : '—' }}</template>
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
import { listGroupBuys } from '../api/admin'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const status = ref(0)
const loading = ref(false)

const fmt = (t) => (t ? String(t).replace('T', ' ').slice(0, 16) : '')

async function load(p = page.value) {
  page.value = p
  loading.value = true
  try {
    const data = await listGroupBuys({ page: p, pageSize, status: status.value })
    list.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}
function onSearch() { load(1) }

onMounted(() => load(1))
</script>

<style scoped>
.card-head { display: flex; justify-content: space-between; align-items: center; }
</style>
