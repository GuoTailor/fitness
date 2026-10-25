<template>
  <el-card>
    <template #header>老带新邀请记录（被邀请人首单实付后双方发券，奖励仅发券不结算现金）</template>
    <el-table :data="list" v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="邀请人" min-width="140">
        <template #default="{ row }">{{ row.inviterName }}（#{{ row.inviterId }}）</template>
      </el-table-column>
      <el-table-column label="被邀请人" min-width="140">
        <template #default="{ row }">{{ row.inviteeName }}（#{{ row.inviteeId }}）</template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="row.status === 2 ? 'success' : 'info'">{{ row.status === 2 ? '已奖励' : '已绑定' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="绑定时间" width="170">
        <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="奖励时间" width="170">
        <template #default="{ row }">{{ row.rewardedAt ? fmt(row.rewardedAt) : '—' }}</template>
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
import { listReferrals } from '../api/admin'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)

const fmt = (v) => (v ? String(v).replace('T', ' ').slice(0, 16) : '')

async function load(p = page.value) {
  page.value = p
  loading.value = true
  try {
    const data = await listReferrals({ page: p, pageSize })
    list.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(() => load(1))
</script>
