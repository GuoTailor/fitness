<template>
  <div>
    <el-card>
      <div class="toolbar">
        <el-radio-group v-model="status" @change="load">
          <el-radio-button value="">全部</el-radio-button>
          <el-radio-button value="pending">待审核</el-radio-button>
          <el-radio-button value="approved">已通过</el-radio-button>
          <el-radio-button value="hidden">已隐藏</el-radio-button>
        </el-radio-group>
        <el-radio-group v-model="targetType" @change="load">
          <el-radio-button value="">全部维度</el-radio-button>
          <el-radio-button value="package">课包评价</el-radio-button>
          <el-radio-button value="coach">教练评价</el-radio-button>
        </el-radio-group>
      </div>
      <el-table :data="list" v-loading="loading">
        <el-table-column label="学员" width="130">
          <template #default="{ row }">{{ row.nickname || row.review.userId }}</template>
        </el-table-column>
        <el-table-column label="维度" width="90">
          <template #default="{ row }">
            <el-tag :type="row.targetType === 'coach' ? 'success' : 'primary'" size="small">
              {{ row.targetType === 'coach' ? '教练' : '课包' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="评分" width="130">
          <template #default="{ row }">
            <el-rate :model-value="row.review.rating" disabled size="small" />
          </template>
        </el-table-column>
        <el-table-column label="内容" min-width="220">
          <template #default="{ row }">
            <div>{{ row.review.content || '（未填写）' }}</div>
            <div class="reply" v-if="row.review.coachReply">教练回复：{{ row.review.coachReply }}</div>
          </template>
        </el-table-column>
        <el-table-column label="对象" width="140">
          <template #default="{ row }">
            {{ row.targetType === 'coach' ? '教练 #' + row.review.coachId : '课包 #' + row.review.packageId }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusType(row.review.status)">{{ statusText(row.review.status) }}</el-tag>
            <el-tag v-if="row.review.featured" type="warning" size="small" style="margin-left: 4px">精选</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="160">
          <template #default="{ row }">{{ fmtTime(row.review.createdAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.review.status !== 'approved'" link type="success" @click="audit(row, 'approve')">
              通过
            </el-button>
            <el-button v-if="row.review.status !== 'hidden'" link type="danger" @click="audit(row, 'hide')">
              隐藏
            </el-button>
            <el-button v-if="row.review.status === 'approved' && !row.review.featured" link type="warning"
                       @click="feature(row, true)">精选</el-button>
            <el-button v-if="row.review.featured" link @click="feature(row, false)">取消精选</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination layout="total, prev, pager, next" :total="total" :page-size="pageSize"
        v-model:current-page="page" @current-change="load" style="margin-top: 12px" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listReviews, auditReview, featureReview } from '../api/admin'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const status = ref('pending')
const targetType = ref('')

const statusText = (s) => ({ pending: '待审核', approved: '已通过', hidden: '已隐藏' }[s] || s)
const statusType = (s) => ({ pending: 'warning', approved: 'success', hidden: 'info' }[s] || 'info')
const fmtTime = (t) => String(t || '').replace('T', ' ').slice(0, 16)

async function load() {
  loading.value = true
  try {
    const d = await listReviews({ status: status.value, targetType: targetType.value, page: page.value, pageSize })
    list.value = d.items || []
    total.value = d.total || 0
  } finally {
    loading.value = false
  }
}

async function audit(row, action) {
  const label = action === 'approve' ? '通过（对外可见）' : '隐藏（不再展示）'
  await ElMessageBox.confirm(`确认${label}该评价？`, '审核确认', { type: 'warning' })
  await auditReview(row.review.id, action)
  ElMessage.success('已处理')
  load()
}

async function feature(row, featured) {
  await featureReview(row.review.id, featured)
  ElMessage.success(featured ? '已设为精选' : '已取消精选')
  load()
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.reply {
  color: #888;
  font-size: 12px;
  background: #f6f8fa;
  border-radius: 4px;
  padding: 4px 8px;
  margin-top: 4px;
}
</style>
