<template>
  <div>
    <el-card>
      <template #header>运营规则</template>
      <el-table :data="configs" v-loading="loading">
        <el-table-column prop="key" label="配置项" width="260" />
        <el-table-column label="值" width="140">
          <template #default="{ row }">
            <el-input v-model="row.value" style="width: 120px" />
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="说明" min-width="220" />
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button link type="primary" @click="save(row)">保存</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card style="margin-top: 16px">
      <template #header>
        <div class="card-head">
          <span>首页 Banner</span>
          <el-button size="small" type="primary" @click="addBanner">新增</el-button>
        </div>
      </template>
      <el-table :data="banners">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column label="图片" min-width="200">
          <template #default="{ row }">
            <el-image v-if="row.imageUrl" :src="row.imageUrl" fit="cover" style="width: 120px; height: 45px; border-radius: 4px" />
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column label="跳转" min-width="140">
          <template #default="{ row }">{{ row.linkType }} {{ row.linkValue }}</template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="70" />
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="editBanner(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialog" :title="form.id ? '编辑 Banner' : '新增 Banner'" width="480px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="图片 URL" required><el-input v-model="form.imageUrl" /></el-form-item>
        <el-form-item label="跳转类型">
          <el-select v-model="form.linkType">
            <el-option value="none" label="无跳转" />
            <el-option value="package" label="课包" />
            <el-option value="coach" label="教练" />
          </el-select>
        </el-form-item>
        <el-form-item label="跳转目标"><el-input v-model="form.linkValue" placeholder="课包/教练 ID" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" /></el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="saveBannerRow">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listConfigs, saveConfig, listBanners, saveBanner, deleteBanner } from '../api/admin'

const configs = ref([])
const banners = ref([])
const loading = ref(false)
const dialog = ref(false)
const form = reactive({ id: 0 })

async function load() {
  loading.value = true
  try {
    configs.value = await listConfigs()
    banners.value = await listBanners()
  } finally {
    loading.value = false
  }
}

async function save(row) {
  await saveConfig({ key: row.key, value: row.value, remark: row.remark })
  ElMessage.success('已保存')
}

function addBanner() {
  Object.assign(form, { id: 0, imageUrl: '', linkType: 'none', linkValue: '', sort: 0, status: 1 })
  dialog.value = true
}
function editBanner(row) {
  Object.assign(form, row)
  dialog.value = true
}
async function saveBannerRow() {
  await saveBanner({ ...form })
  ElMessage.success('已保存')
  dialog.value = false
  load()
}
async function remove(row) {
  await ElMessageBox.confirm('确认删除该 Banner？')
  await deleteBanner(row.id)
  load()
}

onMounted(load)
</script>

<style scoped>
.card-head { display: flex; justify-content: space-between; align-items: center; }
</style>
