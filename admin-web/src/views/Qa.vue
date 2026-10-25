<template>
  <div>
    <el-card>
      <template #header>
        <div class="card-head">
          <span>问答区管理（先发后审：隐藏违规内容、置顶沉淀 FAQ）</span>
          <div class="filters">
            <el-radio-group v-model="status" size="small" @change="load">
              <el-radio-button value="">全部</el-radio-button>
              <el-radio-button value="visible">展示中</el-radio-button>
              <el-radio-button value="hidden">已隐藏</el-radio-button>
            </el-radio-group>
            <el-input v-model="keyword" placeholder="搜索内容" size="small" clearable style="width: 200px; margin-left: 12px"
                      @keyup.enter="onSearch" />
          </div>
        </div>
      </template>
      <el-table :data="list" v-loading="loading">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column label="问题" min-width="320">
          <template #default="{ row }">
            <div class="q-row">
              <el-tag v-if="row.pinned" size="small" type="warning">置顶</el-tag>
              <span class="q-content">{{ row.content }}</span>
            </div>
            <div class="q-sub">{{ row.packageTitle || ('课包 #' + row.id) }} · {{ row.asker }} · {{ fmtTime(row.createdAt) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="回复" min-width="380">
          <template #default="{ row }">
            <div v-for="a in row.answers" :key="a.id" class="a-row" :class="{ hidden: a.status === 'hidden' }">
              <el-tag size="small" :type="a.isCoach ? 'success' : 'info'">{{ a.isCoach ? '教练' : '学员' }}</el-tag>
              <span class="a-content">{{ a.content }}</span>
              <el-button link size="small" :type="a.status === 'hidden' ? 'success' : 'danger'"
                         @click="toggleAnswer(a)">{{ a.status === 'hidden' ? '恢复' : '隐藏' }}</el-button>
            </div>
            <div v-if="!row.answers.length" class="q-sub">暂无回复</div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'visible' ? 'success' : 'info'">{{ row.status === 'visible' ? '展示中' : '已隐藏' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button link :type="row.status === 'hidden' ? 'success' : 'danger'" @click="toggleQ(row)">
              {{ row.status === 'hidden' ? '恢复' : '隐藏' }}
            </el-button>
            <el-button link type="warning" @click="togglePin(row)">{{ row.pinned ? '取消置顶' : '置顶' }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination layout="prev, pager, next, total" :total="total" :page-size="pageSize"
                     :current-page="page" @current-change="onPage" style="margin-top: 12px" />
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listQa, setQaStatus, setQaAnswerStatus, setQaPin } from '../api/admin'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const status = ref('')
const keyword = ref('')
const loading = ref(false)

const fmtTime = (t) => String(t || '').replace('T', ' ').slice(0, 16)

async function load() {
  loading.value = true
  try {
    const data = await listQa({ page: page.value, pageSize, status: status.value, keyword: keyword.value })
    list.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}
function onSearch() { page.value = 1; load() }
function onPage(p) { page.value = p; load() }

async function toggleQ(row) {
  const next = row.status === 'hidden' ? 'visible' : 'hidden'
  await ElMessageBox.confirm(next === 'hidden' ? '隐藏后小程序端不再展示该问题及其可见回复' : '恢复公开展示？')
  await setQaStatus(row.id, next)
  ElMessage.success('已更新')
  load()
}

async function toggleAnswer(a) {
  const next = a.status === 'hidden' ? 'visible' : 'hidden'
  await setQaAnswerStatus(a.id, next)
  ElMessage.success('已更新')
  load()
}

async function togglePin(row) {
  await setQaPin(row.id, !row.pinned)
  ElMessage.success(row.pinned ? '已取消置顶' : '已置顶')
  load()
}

onMounted(load)
</script>

<style scoped>
.card-head { display: flex; justify-content: space-between; align-items: center; }
.filters { display: flex; align-items: center; }
.q-row { display: flex; align-items: center; gap: 6px; }
.q-content { font-weight: 500; }
.q-sub { font-size: 12px; color: #999; margin-top: 4px; }
.a-row { display: flex; align-items: center; gap: 6px; margin-bottom: 6px; }
.a-row.hidden .a-content { color: #bbb; text-decoration: line-through; }
.a-content { flex: 1; font-size: 13px; }
</style>
