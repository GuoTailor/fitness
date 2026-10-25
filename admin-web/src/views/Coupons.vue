<template>
  <div>
    <el-card>
      <template #header>
        <div class="card-head">
          <span>优惠券（领取率/使用率对账）</span>
          <el-button size="small" type="primary" @click="add">新建优惠券</el-button>
        </div>
      </template>
      <el-table :data="list" v-loading="loading">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column label="名称 / 优惠" min-width="150">
          <template #default="{ row }">
            <div>{{ row.name }}</div>
            <el-tag size="small" type="danger">{{ row.label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="适用范围" width="110">
          <template #default="{ row }">{{ row.packageId ? '课包 #' + row.packageId : '全场通用' }}</template>
        </el-table-column>
        <el-table-column label="发放（已发/总量）" width="130">
          <template #default="{ row }">{{ row.issuedCount }} / {{ row.totalCount || '不限' }}</template>
        </el-table-column>
        <el-table-column prop="heldCount" label="持券人数" width="90" />
        <el-table-column prop="usedCount" label="已核销" width="80" />
        <el-table-column label="使用率" width="90">
          <template #default="{ row }">
            {{ row.issuedCount ? Math.round((row.usedCount / row.issuedCount) * 100) : 0 }}%
          </template>
        </el-table-column>
        <el-table-column label="每人限领 / 有效期" width="130">
          <template #default="{ row }">{{ row.perUserLimit }} 张 / {{ row.validDays }} 天</template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '发放中' : '已停发' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="grant(row)">定向发放</el-button>
            <el-button link type="warning" @click="toggle(row)">{{ row.status === 1 ? '停发' : '恢复' }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialog" :title="form.id ? '编辑优惠券' : '新建优惠券'" width="560px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="券名称" required><el-input v-model="form.name" placeholder="如：开业满减券" /></el-form-item>
        <el-form-item label="券类型">
          <el-radio-group v-model="form.type" :disabled="form.id > 0">
            <el-radio value="reduce">满减券</el-radio>
            <el-radio value="discount">折扣券</el-radio>
            <el-radio value="gift">赠课券</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="amountLabel">
          <el-input-number v-model="form.amountYuan" :min="amountMin" :max="amountMax" :step="form.type === 'gift' ? 1 : 1" :precision="form.type === 'gift' ? 0 : 2" />
          <span class="tip">{{ amountTip }}</span>
        </el-form-item>
        <el-form-item label="用券门槛（元）" v-if="form.type !== 'gift'">
          <el-input-number v-model="form.thresholdYuan" :min="0" :max="99999" :step="1" :precision="2" />
          <span class="tip">0 = 无门槛</span>
        </el-form-item>
        <el-form-item label="限定课包 ID">
          <el-input-number v-model="form.packageId" :min="0" :step="1" :precision="0" />
          <span class="tip">0 = 全场通用</span>
        </el-form-item>
        <el-form-item label="发放总量"><el-input-number v-model="form.totalCount" :min="0" :step="10" /><span class="tip">0 = 不限</span></el-form-item>
        <el-form-item label="每人限领"><el-input-number v-model="form.perUserLimit" :min="1" :max="100" /></el-form-item>
        <el-form-item label="领取后有效"><el-input-number v-model="form.validDays" :min="1" :max="3650" /> 天</el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="grantDialog" title="定向发放（绕过每人限领，仍受总量约束）" width="420px">
      <el-form label-width="110px">
        <el-form-item label="券">
          <el-tag type="danger">{{ grantRow ? grantRow.name + ' ' + grantRow.label : '' }}</el-tag>
        </el-form-item>
        <el-form-item label="学员用户 ID" required>
          <el-input-number v-model="grantUserId" :min="1" :step="1" :precision="0" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="grantDialog = false">取消</el-button>
        <el-button type="primary" @click="doGrant">发放</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listCoupons, saveCoupon, toggleCoupon, grantCoupon } from '../api/admin'

const list = ref([])
const loading = ref(false)
const dialog = ref(false)
const form = reactive({})
const grantDialog = ref(false)
const grantRow = ref(null)
const grantUserId = ref(0)

const amountLabel = computed(() =>
  ({ reduce: '减免金额（元）', discount: '折扣（折）', gift: '赠送课时数' }[form.type] || '金额'))
const amountTip = computed(() =>
  ({ discount: '填 8.8 表示 8.8 折', gift: '用券购课即赠 N 节课时（不降价，可与活动价叠加）', reduce: '' }[form.type] || ''))
const amountMin = computed(() => (form.type === 'gift' ? 1 : 0.01))
const amountMax = computed(() => (form.type === 'gift' ? 10 : 99999))

async function load() {
  loading.value = true
  try {
    const data = await listCoupons()
    list.value = data.list || []
  } finally {
    loading.value = false
  }
}

function add() {
  Object.assign(form, {
    id: 0, name: '', type: 'reduce', amountYuan: 10, thresholdYuan: 0,
    packageId: 0, totalCount: 0, perUserLimit: 1, validDays: 30, remark: '',
  })
  dialog.value = true
}

async function save() {
  // 折扣：8.8（折）→ 8800 万分比；满减：元 → 分；赠课：课时数原样
  const amount = form.type === 'discount'
    ? Math.round(form.amountYuan * 1000)
    : form.type === 'gift'
      ? Math.round(form.amountYuan)
      : Math.round(form.amountYuan * 100)
  await saveCoupon({
    id: form.id, name: form.name, type: form.type,
    threshold: form.type === 'gift' ? 0 : Math.round(form.thresholdYuan * 100),
    amount,
    packageId: form.packageId, totalCount: form.totalCount,
    perUserLimit: form.perUserLimit, validDays: form.validDays, remark: form.remark,
    status: 1,
  })
  ElMessage.success('已保存')
  dialog.value = false
  load()
}

async function toggle(row) {
  const next = row.status === 1 ? 0 : 1
  await ElMessageBox.confirm(next === 0 ? `确认停发「${row.name}」？已领取的券仍可使用。` : `确认恢复发放「${row.name}」？`)
  await toggleCoupon(row.id, next)
  ElMessage.success('已更新')
  load()
}

function grant(row) {
  grantRow.value = row
  grantUserId.value = 0
  grantDialog.value = true
}

async function doGrant() {
  if (!grantUserId.value) return ElMessage.warning('请输入学员用户 ID（学员管理页可查）')
  await grantCoupon(grantRow.value.id, grantUserId.value)
  ElMessage.success('已发放')
  grantDialog.value = false
  load()
}

onMounted(load)
</script>

<style scoped>
.card-head { display: flex; justify-content: space-between; align-items: center; }
.tip { margin-left: 8px; font-size: 12px; color: #999; }
</style>
