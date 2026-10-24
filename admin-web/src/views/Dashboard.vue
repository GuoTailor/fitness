<template>
  <div v-loading="loading">
    <!-- 待办提醒 -->
    <el-alert v-if="d.pendingReviews || d.pendingRefunds" type="warning" :closable="false" style="margin-bottom: 16px">
      <template #title>
        待处理：
        <el-link type="primary" @click="$router.push('/reviews')">待审核评价 {{ d.pendingReviews }} 条</el-link>
        <span style="margin: 0 8px">·</span>
        <el-link type="primary" @click="$router.push('/orders')">待处理退款 {{ d.pendingRefunds }} 单</el-link>
      </template>
    </el-alert>

    <!-- 指标卡片 -->
    <div class="cards">
      <el-card class="stat-card" v-for="c in cards" :key="c.label">
        <div class="stat-v">{{ c.value }}</div>
        <div class="stat-l">{{ c.label }}</div>
        <div class="stat-sub" v-if="c.sub">{{ c.sub }}</div>
      </el-card>
    </div>

    <!-- 趋势图 -->
    <el-card style="margin-top: 16px">
      <template #header>近 30 天经营趋势</template>
      <div ref="trendRef" class="chart"></div>
    </el-card>

    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :span="14">
        <el-card>
          <template #header>教练本月课时排行</template>
          <div ref="rankRef" class="chart"></div>
        </el-card>
      </el-col>
      <el-col :span="10">
        <el-card>
          <template #header>经营指标说明</template>
          <div class="defs">
            <p><b>净 GMV</b>：本月已支付订单实付总额 − 已退金额</p>
            <p><b>课消率</b>：非退款课包的 已消耗课时 ÷ 总购入课时，反映库存消耗速度</p>
            <p><b>复购率</b>：支付 ≥2 单的学员 ÷ 所有支付学员</p>
            <p><b>课时消耗</b>：本月完成核销的训练记录数（私教+团课）</p>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import * as echarts from 'echarts'
import http from '../api/request'
import { yuan } from '../api/admin'

const loading = ref(false)
const d = ref({})
const trendRef = ref()
const rankRef = ref()
let trendChart, rankChart

const pct = (v) => (v == null ? '—' : (v * 100).toFixed(1) + '%')

const cards = computed(() => {
  const x = d.value
  return [
    { label: '本月净 GMV', value: '¥' + yuan(x.monthNetGmv ?? 0), sub: `含退款前 ¥${yuan(x.monthGmv ?? 0)}` },
    { label: '今日 GMV', value: '¥' + yuan(x.todayGmv ?? 0), sub: `本月订单 ${x.monthOrders ?? 0} 单` },
    { label: '本月课时消耗', value: x.monthLessons ?? 0, sub: `合计 ${x.monthMinutes ?? 0} 分钟` },
    { label: '课消率', value: pct(x.burnRate), sub: `学员 ${x.totalUsers ?? 0} 人` },
    { label: '复购率', value: pct(x.repurchaseRate), sub: '按支付订单口径' },
  ]
})

async function load() {
  loading.value = true
  try {
    d.value = await http.get('/admin/api/v1/dashboard')
    await nextTick()
    renderTrend()
    renderRank()
  } finally {
    loading.value = false
  }
}

function renderTrend() {
  if (!trendRef.value) return
  trendChart = trendChart || echarts.init(trendRef.value)
  const t = d.value.trend || {}
  trendChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['GMV（元）', '课时消耗'] },
    grid: { left: 50, right: 50, top: 40, bottom: 30 },
    xAxis: { type: 'category', data: t.dates || [] },
    yAxis: [
      { type: 'value', name: 'GMV（元）' },
      { type: 'value', name: '课时', minInterval: 1 },
    ],
    series: [
      {
        name: 'GMV（元）', type: 'bar', itemStyle: { color: '#00b578' },
        data: (t.gmv || []).map((v) => v / 100),
      },
      { name: '课时消耗', type: 'line', yAxisIndex: 1, smooth: true, itemStyle: { color: '#4a90ff' }, data: t.lessons || [] },
    ],
  })
}

function renderRank() {
  if (!rankRef.value) return
  rankChart = rankChart || echarts.init(rankRef.value)
  const rank = d.value.coachRank || []
  rankChart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 90, right: 30, top: 20, bottom: 30 },
    xAxis: { type: 'value', minInterval: 1 },
    yAxis: {
      type: 'category', inverse: true,
      data: rank.map((r) => r.name).reverse(),
      axisLabel: { formatter: (v, i) => `${v}（★${(rank[rank.length - 1 - i].ratingAvg ?? 0).toFixed(1)}）` },
    },
    series: [{ type: 'bar', itemStyle: { color: '#00b578' }, barWidth: 18, data: rank.map((r) => r.lessons).reverse() }],
  })
}

function onResize() {
  trendChart && trendChart.resize()
  rankChart && rankChart.resize()
}

onMounted(() => {
  load()
  window.addEventListener('resize', onResize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  trendChart && trendChart.dispose()
  rankChart && rankChart.dispose()
})
</script>

<style scoped>
.cards {
  display: flex;
  gap: 16px;
}

.stat-card {
  flex: 1;
  margin-bottom: 0;
}

.stat-v {
  font-size: 26px;
  font-weight: 700;
  color: #303133;
}

.stat-l {
  color: #909399;
  font-size: 13px;
  margin-top: 6px;
}

.stat-sub {
  color: #c0c4cc;
  font-size: 12px;
  margin-top: 4px;
}

.chart {
  width: 100%;
  height: 320px;
}

.defs p {
  color: #606266;
  font-size: 13px;
  line-height: 2;
  margin: 0;
}
</style>
