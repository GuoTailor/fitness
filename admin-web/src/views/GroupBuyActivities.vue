<template>
  <el-card>
    <template #header>
      <div class="card-head">
        <span>拼团限时活动（生效期内开团按活动配置，名额按开团数计；结束自动回落 SKU 常开拼团）</span>
        <el-button type="primary" @click="openForm(null)">新建活动</el-button>
      </div>
    </template>
    <el-table :data="list" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="title" label="标题" min-width="140" />
      <el-table-column label="规格" min-width="160">
        <template #default="{ row }">{{ row.pkgTitle }} · {{ row.skuName || `SKU#${row.skuId}` }}</template>
      </el-table-column>
      <el-table-column label="拼团价" width="90">
        <template #default="{ row }">¥{{ (row.groupPrice / 100).toFixed(0) }}</template>
      </el-table-column>
      <el-table-column label="成团" width="70">
        <template #default="{ row }">{{ row.groupCapacity }}人</template>
      </el-table-column>
      <el-table-column label="时间" width="230">
        <template #default="{ row }">{{ fmt(row.startAt) }} ~ {{ fmt(row.endAt) }}</template>
      </el-table-column>
      <el-table-column label="团数(开/限)" width="100">
        <template #default="{ row }">{{ row.openedCount }}/{{ row.maxGroups > 0 ? row.maxGroups : '不限' }}</template>
      </el-table-column>
      <el-table-column label="已成团" width="80">
        <template #default="{ row }">{{ row.successCount }}</template>
      </el-table-column>
      <el-table-column label="每团有效期" width="100">
        <template #default="{ row }">{{ row.expireHours > 0 ? row.expireHours + '小时' : '全局' }}</template>
      </el-table-column>
      <el-table-column label="生效状态" width="90">
        <template #default="{ row }">
          <el-tag :type="tagType(row.effectiveText)">{{ row.effectiveText }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="上架" width="70">
        <template #default="{ row }">
          <el-switch :model-value="row.status" :active-value="1" :inactive-value="0" @change="(v) => toggle(row, v)" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="130" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openForm(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="form.id ? '编辑活动' : '新建活动'" width="560px" destroy-on-close>
      <el-form :model="form" label-width="100px">
        <el-form-item label="活动标题" required><el-input v-model="form.title" placeholder="如：双十一直降 3 人团" /></el-form-item>
        <el-form-item label="作用规格" required>
          <el-select v-model="form.skuId" filterable placeholder="选择课包规格" style="width: 100%">
            <el-option-group v-for="p in packages" :key="p.id" :label="p.title">
              <el-option v-for="s in p.skus" :key="s.id" :value="s.id" :label="`${s.name}（¥${(s.price / 100).toFixed(0)}）`" />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item label="拼团价(分)" required>
          <el-input-number v-model="form.groupPrice" :min="1" :step="100" style="width: 180px" />
        </el-form-item>
        <el-form-item label="成团人数" required>
          <el-input-number v-model="form.groupCapacity" :min="2" :max="5" style="width: 180px" />
        </el-form-item>
        <el-form-item label="活动时间" required>
          <el-date-picker v-model="form.startAt" type="datetime" placeholder="开始时间" value-format="YYYY-MM-DD HH:mm:ss" style="width: 200px" />
          <span style="margin: 0 6px">~</span>
          <el-date-picker v-model="form.endAt" type="datetime" placeholder="结束时间" value-format="YYYY-MM-DD HH:mm:ss" style="width: 200px" />
        </el-form-item>
        <el-form-item label="总团数上限">
          <el-input-number v-model="form.maxGroups" :min="0" :step="1" style="width: 180px" />
          <span class="hint">0 = 不限（按开团数计，开团即占坑）</span>
        </el-form-item>
        <el-form-item label="每团有效期">
          <el-input-number v-model="form.expireHours" :min="0" :step="1" style="width: 180px" />
          <span class="hint">小时；0 = 用全局配置</span>
        </el-form-item>
        <el-form-item label="上架">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listGroupBuyActivities, saveGroupBuyActivity, deleteGroupBuyActivity, listPackages } from '../api/admin'

const list = ref([])
const packages = ref([])
const loading = ref(false)
const dialog = ref(false)
const saving = ref(false)
const form = reactive({ id: 0, title: '', skuId: 0, groupPrice: 0, groupCapacity: 3, startAt: '', endAt: '', maxGroups: 0, expireHours: 0, status: 1 })

const fmt = (t) => (t ? String(t).replace('T', ' ').slice(0, 16) : '')
const tagType = (t) => ({ 进行中: 'success', 未开始: 'info', 已结束: 'warning', 已下架: 'danger' }[t] || 'info')

// 后端 ISO 时间 → 表单用的 YYYY-MM-DD HH:mm:ss
function fmtDT(v) {
  if (!v) return ''
  const d = new Date(v)
  if (isNaN(d)) return ''
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

async function load() {
  loading.value = true
  try {
    const data = await listGroupBuyActivities()
    list.value = data.list || []
  } finally {
    loading.value = false
  }
}

async function loadPackages() {
  const data = await listPackages({ page: 1, page_size: 50 })
  packages.value = (data.list || []).filter((p) => p.status === 1)
}

function openForm(row) {
  Object.assign(form, row
    ? { ...row, startAt: fmtDT(row.startAt), endAt: fmtDT(row.endAt) }
    : { id: 0, title: '', skuId: 0, groupPrice: 0, groupCapacity: 3, startAt: '', endAt: '', maxGroups: 0, expireHours: 0, status: 1 })
  dialog.value = true
}

async function toggle(row, v) {
  await ElMessageBox.confirm(`确认${v === 1 ? '上架' : '下架'}「${row.title}」？`)
  await saveGroupBuyActivity({ ...row, status: v })
  ElMessage.success('已更新')
  load()
}

async function save() {
  if (!form.title) return ElMessage.warning('请填写活动标题')
  if (!form.skuId) return ElMessage.warning('请选择作用规格')
  if (!form.groupPrice) return ElMessage.warning('请填写拼团价')
  if (!form.startAt || !form.endAt) return ElMessage.warning('请选择活动起止时间')
  if (form.endAt <= form.startAt) return ElMessage.warning('结束时间需晚于开始时间')
  saving.value = true
  try {
    await saveGroupBuyActivity({ ...form })
    ElMessage.success('已保存')
    dialog.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除「${row.title}」？已有开团的活动不能删除。`)
  try {
    await deleteGroupBuyActivity(row.id)
    ElMessage.success('已删除')
    load()
  } catch { /* 后端拒绝时保留 */ }
}

onMounted(() => { load(); loadPackages() })
</script>

<style scoped>
.card-head { display: flex; justify-content: space-between; align-items: center; }
.hint { margin-left: 10px; color: #999; font-size: 12px; }
</style>
