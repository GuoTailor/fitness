<template>
  <div>
    <el-card>
      <div class="toolbar">
        <el-radio-group v-model="status" @change="load">
          <el-radio-button value="">全部</el-radio-button>
          <el-radio-button value="pending">待支付</el-radio-button>
          <el-radio-button value="paid">已支付</el-radio-button>
          <el-radio-button value="refunding">退款中</el-radio-button>
          <el-radio-button value="refunded">已退款</el-radio-button>
        </el-radio-group>
        <el-input v-model="keyword" placeholder="订单号/标题" style="width: 240px" clearable @keyup.enter="load" />
        <el-button @click="load">搜索</el-button>
      </div>
      <el-table :data="list" v-loading="loading">
        <el-table-column prop="orderNo" label="订单号" width="230" />
        <el-table-column prop="title" label="商品" min-width="150" />
        <el-table-column label="学员" min-width="120">
          <template #default="{ row }">{{ row.userNickname || row.userId }}<span class="sub">{{ row.userPhone }}</span></template>
        </el-table-column>
        <el-table-column label="金额" width="90">
          <template #default="{ row }">¥{{ yuan(row.amount) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="下单时间" width="170">
          <template #default="{ row }">{{ fmtTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row.id)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination layout="total, prev, pager, next" :total="total" :page-size="pageSize"
        v-model:current-page="page" @current-change="load" style="margin-top: 12px" />
    </el-card>

    <!-- 待处理退款快捷区 -->
    <el-card v-if="pendingRefunds.length" style="margin-top: 16px">
      <template #header>待处理退款（{{ pendingRefunds.length }}）</template>
      <el-table :data="pendingRefunds">
        <el-table-column prop="refundNo" label="退款单号" width="240" />
        <el-table-column prop="orderId" label="订单ID" width="80" />
        <el-table-column prop="reason" label="原因" min-width="140" />
        <el-table-column label="申请时间" width="170">
          <template #default="{ row }">{{ fmtTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="success" @click="approve(row)">同意退款</el-button>
            <el-button link type="danger" @click="reject(row)">拒绝</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-drawer v-model="drawer" title="订单详情" size="480px">
      <template v-if="detail">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="订单号">{{ detail.order.orderNo }}</el-descriptions-item>
          <el-descriptions-item label="商品">{{ detail.order.title }}</el-descriptions-item>
          <el-descriptions-item label="金额">¥{{ yuan(detail.order.amount) }}（已退 ¥{{ yuan(detail.order.refundAmount) }}）</el-descriptions-item>
          <el-descriptions-item label="状态">{{ statusText(detail.order.status) }}</el-descriptions-item>
          <el-descriptions-item label="微信单号">{{ detail.order.transactionId || '—' }}</el-descriptions-item>
          <el-descriptions-item label="课时账户">
            <span v-if="detail.hasMemberPackage">
              {{ detail.memberPackage.packageTitle }}：{{ detail.memberPackage.totalSessions }} 节，已用 {{ detail.memberPackage.usedSessions }}，状态 {{ detail.memberPackage.status }}
            </span>
            <span v-else>无</span>
          </el-descriptions-item>
        </el-descriptions>
        <el-divider>退款记录</el-divider>
        <el-empty v-if="!detail.refunds.length" description="无" :image-size="60" />
        <div v-for="r in detail.refunds" :key="r.id" class="refund-row">
          <div>{{ r.refundNo }} · {{ refundStatusText(r.status) }} · ¥{{ yuan(r.amount) }}</div>
          <div class="sub">原因：{{ r.reason || '—' }} {{ r.rejectReason ? '｜拒绝理由：' + r.rejectReason : '' }}</div>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listOrders, getOrder, listRefunds, approveRefund, rejectRefund, yuan } from '../api/admin'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const status = ref('')
const keyword = ref('')
const loading = ref(false)
const pendingRefunds = ref([])
const drawer = ref(false)
const detail = ref(null)

const statusText = (s) => ({ pending: '待支付', paid: '已支付', closed: '已关闭', refunding: '退款中', refunded: '已退款' }[s] || s)
const statusType = (s) => ({ pending: 'warning', paid: 'success', closed: 'info', refunding: 'danger', refunded: 'info' }[s] || 'info')
const refundStatusText = (s) => ({ pending: '待处理', approved: '已同意', rejected: '已拒绝', refunded: '已退款', failed: '失败' }[s] || s)
const fmtTime = (t) => (t ? String(t).replace('T', ' ').slice(0, 19) : '—')

async function load() {
  loading.value = true
  try {
    const data = await listOrders({ page: page.value, page_size: pageSize, status: status.value, keyword: keyword.value })
    list.value = data.list
    total.value = data.total
    const r = await listRefunds({ status: 'pending', page: 1, page_size: 50 })
    pendingRefunds.value = r.list
  } finally {
    loading.value = false
  }
}

async function openDetail(id) {
  detail.value = await getOrder(id)
  drawer.value = true
}

async function approve(row) {
  await ElMessageBox.confirm('同意后剩余课时将被回收，退款金额按规则自动计算。继续？')
  const r = await approveRefund(row.id)
  ElMessage.success(`已退款 ¥${yuan(r.amount)}`)
  load()
}

async function reject(row) {
  const { value } = await ElMessageBox.prompt('拒绝理由', '拒绝退款', { inputPlaceholder: '填写拒绝理由' })
  await rejectRefund(row.id, value || '')
  ElMessage.success('已拒绝')
  load()
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; gap: 12px; margin-bottom: 12px; }
.sub { display: block; font-size: 12px; color: #999; }
.refund-row { margin-bottom: 10px; padding: 8px; background: #f8f9fa; border-radius: 6px; }
</style>
