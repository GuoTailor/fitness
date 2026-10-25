<template>
  <div>
    <el-card>
      <div class="toolbar">
        <el-button type="primary" @click="openForm()">新增课包</el-button>
        <el-input v-model="keyword" placeholder="搜索标题" style="width: 220px" clearable @keyup.enter="load" />
        <el-button @click="load">搜索</el-button>
      </div>
      <el-table :data="list" v-loading="loading">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="title" label="标题" min-width="160" />
        <el-table-column label="分类" width="90">
          <template #default="{ row }">{{ categoryText(row.category) }}</template>
        </el-table-column>
        <el-table-column prop="tags" label="标签" min-width="120" />
        <el-table-column label="规格" min-width="180">
          <template #default="{ row }">
            <span v-for="s in row.skus" :key="s.id" class="sku-chip">
              {{ s.name }} ¥{{ yuan(s.price) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="soldCount" label="销量" width="70" />
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '上架' : '下架' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openForm(row)">编辑</el-button>
            <el-button link :type="row.status === 1 ? 'info' : 'success'" @click="toggleStatus(row)">
              {{ row.status === 1 ? '下架' : '上架' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination layout="total, prev, pager, next" :total="total" :page-size="pageSize"
        v-model:current-page="page" @current-change="load" style="margin-top: 12px" />
    </el-card>

    <el-dialog v-model="dialog" :title="form.id ? '编辑课包' : '新增课包'" width="720px" destroy-on-close>
      <el-form :model="form" label-width="90px">
        <el-form-item label="标题" required><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="分类">
          <el-radio-group v-model="form.category">
            <el-radio value="private">私教 1v1</el-radio>
            <el-radio value="group">团课卡</el-radio>
            <el-radio value="trial">体验课</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="训练目标"><el-input v-model="form.tags" placeholder="逗号分隔：减脂,增肌,塑形,康复" /></el-form-item>
        <el-form-item label="难度">
          <el-rate v-model="form.difficulty" :max="5" />
        </el-form-item>
        <el-form-item label="封面图"><el-input v-model="form.coverUrl" placeholder="图片 URL" /></el-form-item>
        <el-form-item label="介绍视频"><el-input v-model="form.introVideoUrl" placeholder="视频 URL（nginx 静态目录地址）" /></el-form-item>
        <el-form-item label="课程大纲"><el-input v-model="form.outline" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="适合人群"><el-input v-model="form.suitable" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="关联教练">
          <el-select v-model="form.coachIds" multiple placeholder="选择在售教练" style="width: 100%">
            <el-option v-for="c in coaches" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" /></el-form-item>

        <el-divider content-position="left">规格（{{ form.skus.length }}）</el-divider>
        <div v-for="(s, i) in form.skus" :key="i" class="sku-row">
          <el-input v-model="s.name" placeholder="名称 如10次卡" style="width: 130px" />
          <el-input-number v-model="s.sessions" :min="1" placeholder="课时" style="width: 110px" />
          <el-input-number v-model="s.validDays" :min="1" placeholder="有效天数" style="width: 120px" />
          <el-input-number v-model="s.price" :min="0" :step="100" placeholder="售价(分)" style="width: 140px" />
          <el-input-number v-model="s.originalPrice" :min="0" :step="100" placeholder="原价(分)" style="width: 140px" />
          <el-input-number v-model="s.newcomerPrice" :min="0" :step="100" placeholder="新人价(分,0关)" style="width: 160px" />
          <el-input-number v-model="s.promoPrice" :min="0" :step="100" placeholder="活动价(分,0关)" style="width: 160px" />
          <el-date-picker v-model="s.promoStart" type="datetime" placeholder="活动开始"
                          value-format="YYYY-MM-DD HH:mm:ss" style="width: 190px" />
          <el-date-picker v-model="s.promoEnd" type="datetime" placeholder="活动结束"
                          value-format="YYYY-MM-DD HH:mm:ss" style="width: 190px" />
          <el-button link type="danger" @click="form.skus.splice(i, 1)">删除</el-button>
        </div>
        <el-button size="small" style="margin-top: 8px" @click="addSku">+ 添加规格</el-button>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listPackages, savePackage, listCoaches, yuan } from '../api/admin'

const list = ref([])
const coaches = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const keyword = ref('')
const loading = ref(false)
const dialog = ref(false)
const saving = ref(false)
const form = reactive({ id: 0, skus: [], coachIds: [] })

const categoryText = (c) => ({ private: '私教', group: '团课', trial: '体验' }[c] || c)

async function load() {
  loading.value = true
  try {
    const data = await listPackages({ page: page.value, page_size: pageSize, keyword: keyword.value })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function loadCoaches() {
  const data = await listCoaches({ page: 1, page_size: 50 })
  coaches.value = data.list
}

function addSku() {
  form.skus.push({ id: 0, name: '', sessions: 10, validDays: 365, price: 0, originalPrice: 0, newcomerPrice: 0, promoPrice: 0, promoStart: '', promoEnd: '', status: 1, sort: form.skus.length + 1 })
}

function openForm(row) {
  Object.assign(form, row
    ? {
        ...row,
        skus: row.skus.map((s) => ({
          newcomerPrice: 0, promoPrice: 0, promoStart: '', promoEnd: '', ...s,
          promoStart: fmtDT(s.promoStart), promoEnd: fmtDT(s.promoEnd),
        })),
        coachIds: [...(row.coachIds || [])],
      }
    : { id: 0, title: '', category: 'private', tags: '', difficulty: 3, coverUrl: '', introVideoUrl: '',
        outline: '', suitable: '', status: 1, sort: 0, skus: [], coachIds: [] })
  if (!form.skus.length) addSku()
  dialog.value = true
}

// 后端 ISO 时间 → 表单用的 YYYY-MM-DD HH:mm:ss（el-date-picker value-format）
function fmtDT(v) {
  if (!v) return ''
  const d = new Date(v)
  if (isNaN(d)) return ''
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

async function toggleStatus(row) {
  const target = row.status === 1 ? 0 : 1
  await ElMessageBox.confirm(`确认${target === 1 ? '上架' : '下架'}「${row.title}」？`)
  await savePackage({ ...row, skus: row.skus, coachIds: row.coachIds || [], status: target })
  ElMessage.success('已更新')
  load()
}

async function save() {
  if (!form.title) return ElMessage.warning('请填写标题')
  if (!form.skus.length) return ElMessage.warning('至少一个规格')
  saving.value = true
  try {
    await savePackage({ ...form })
    ElMessage.success('已保存')
    dialog.value = false
    load()
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  load()
  loadCoaches()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}
.sku-chip {
  margin-right: 8px;
  background: #f4f4f5;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
}
.sku-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}
</style>
