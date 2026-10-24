<template>
  <el-card>
    <div class="toolbar">
      <el-button type="primary" @click="openForm()">新增教练</el-button>
      <el-input v-model="keyword" placeholder="搜索姓名" style="width: 200px" clearable @keyup.enter="load" />
      <el-button @click="load">搜索</el-button>
    </div>
    <el-table :data="list" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column label="教练" min-width="140">
        <template #default="{ row }">{{ row.name }}<span class="sub">（{{ row.years }}年教龄）</span></template>
      </el-table-column>
      <el-table-column prop="specialties" label="擅长" min-width="120" />
      <el-table-column label="评分" width="100">
        <template #default="{ row }">{{ row.ratingAvg }}（{{ row.ratingCount }}）</template>
      </el-table-column>
      <el-table-column label="资质" width="90">
        <template #default="{ row }">
          <el-tag :type="row.certVerified === 1 ? 'success' : 'warning'">{{ row.certVerified === 1 ? '已认证' : '待审核' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="绑定账号" width="90">
        <template #default="{ row }">{{ row.userId || '—' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '在职' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openForm(row)">编辑</el-button>
          <el-button v-if="row.certVerified !== 1" link type="success" @click="verify(row, 1)">通过资质</el-button>
          <el-button v-else link type="info" @click="verify(row, 0)">取消资质</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination layout="total, prev, pager, next" :total="total" :page-size="pageSize"
      v-model:current-page="page" @current-change="load" style="margin-top: 12px" />

    <el-dialog v-model="dialog" :title="form.id ? '编辑教练' : '新增教练'" width="560px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="姓名" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="头像 URL"><el-input v-model="form.avatarUrl" /></el-form-item>
        <el-form-item label="性别">
          <el-radio-group v-model="form.gender">
            <el-radio :value="0">保密</el-radio><el-radio :value="1">男</el-radio><el-radio :value="2">女</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="从业年限"><el-input-number v-model="form.years" :min="0" /></el-form-item>
        <el-form-item label="擅长领域"><el-input v-model="form.specialties" placeholder="逗号分隔：减脂,增肌" /></el-form-item>
        <el-form-item label="资质证书"><el-input v-model="form.certImages" placeholder="图片 URL JSON 数组，如 [&quot;a.jpg&quot;]" /></el-form-item>
        <el-form-item label="自我介绍"><el-input v-model="form.intro" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="介绍视频"><el-input v-model="form.introVideoUrl" /></el-form-item>
        <el-form-item label="绑定账号ID">
          <el-input-number v-model="form.userId" :min="0" />
          <div class="sub">绑定学员账号后，该账号登录小程序即可使用教练工作台</div>
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" active-text="在职" inactive-text="停用" />
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
import { ElMessage } from 'element-plus'
import { listCoaches, saveCoach, verifyCoachCert } from '../api/admin'

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const keyword = ref('')
const loading = ref(false)
const dialog = ref(false)
const saving = ref(false)
const form = reactive({ id: 0 })

async function load() {
  loading.value = true
  try {
    const data = await listCoaches({ page: page.value, page_size: pageSize, keyword: keyword.value })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function openForm(row) {
  Object.assign(form, row ? { ...row } : {
    id: 0, name: '', avatarUrl: '', gender: 0, years: 0, specialties: '', certImages: '',
    intro: '', introVideoUrl: '', userId: 0, status: 1, sort: 0,
  })
  dialog.value = true
}

async function verify(row, verified) {
  await verifyCoachCert(row.id, verified)
  ElMessage.success('已更新')
  load()
}

async function save() {
  if (!form.name) return ElMessage.warning('请填写姓名')
  saving.value = true
  try {
    await saveCoach({ ...form })
    ElMessage.success('已保存')
    dialog.value = false
    load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; gap: 12px; margin-bottom: 12px; }
.sub { font-size: 12px; color: #999; }
</style>
