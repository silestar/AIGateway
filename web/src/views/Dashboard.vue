<template>
  <n-card :bordered="false" class="glass-card">
    <!-- 统计卡片行 -->
    <n-grid :cols="5" :x-gap="16" :y-gap="16">
      <n-gi>
        <div class="stat-card">
          <div class="stat-icon">📊</div>
          <div class="stat-label">{{ t('dashboard.todayRequests') }}</div>
          <div class="stat-value">{{ overview.total_requests_today || 0 }}</div>
        </div>
      </n-gi>
      <n-gi>
        <div class="stat-card">
          <div class="stat-icon">✅</div>
          <div class="stat-label">{{ t('dashboard.successRate') }}</div>
          <div class="stat-value" :style="{ color: successRateColor }">
            {{ overview.success_rate != null ? overview.success_rate.toFixed(1) + '%' : '—' }}
          </div>
        </div>
      </n-gi>
      <n-gi>
        <div class="stat-card">
          <div class="stat-icon">⚡</div>
          <div class="stat-label">{{ t('dashboard.avgLatency') }}</div>
          <div class="stat-value" :style="{ color: latencyColor }">
            {{ overview.latency_display || (overview.avg_latency_ms != null ? Math.round(overview.avg_latency_ms) + 'ms' : '—') }}
          </div>
        </div>
      </n-gi>
      <n-gi>
        <div class="stat-card">
          <div class="stat-icon">🔑</div>
          <div class="stat-label">{{ t('dashboard.activeKeys') }}</div>
          <div class="stat-value">{{ overview.active_keys || 0 }}</div>
        </div>
      </n-gi>
      <n-gi>
        <div class="stat-card">
          <div class="stat-icon">🔌</div>
          <div class="stat-label">{{ t('dashboard.activeChannels') }}</div>
          <div class="stat-value">{{ overview.active_channels || 0 }}</div>
        </div>
      </n-gi>
    </n-grid>

    <!-- Token 统计数据行 -->
    <n-card :bordered="false" size="small" class="glass-card" style="margin-top: 20px">
      <template #header>
        <span>{{ t('dashboard.tokenStats') }}</span>
      </template>
      <template #header-extra>
        <n-button-group size="small">
          <n-button :type="tokenDays === 1 ? 'primary' : 'default'" @click="switchTokenDays(1)">{{ t('dashboard.today') }}</n-button>
          <n-button :type="tokenDays === 7 ? 'primary' : 'default'" @click="switchTokenDays(7)">7{{ t('dashboard.days') }}</n-button>
          <n-button :type="tokenDays === 30 ? 'primary' : 'default'" @click="switchTokenDays(30)">30{{ t('dashboard.days') }}</n-button>
        </n-button-group>
      </template>
      <n-grid :cols="5" :x-gap="16" :y-gap="16" v-if="tokenStats">
        <n-gi>
          <div class="stat-card">
            <div class="stat-icon">🔤</div>
            <div class="stat-label">{{ t('dashboard.totalTokens') }}</div>
            <div class="stat-value">{{ formatTokenNumber(tokenStats.total_tokens) }}</div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-icon">⏱️</div>
            <div class="stat-label">{{ t('dashboard.avgTPM') }}</div>
            <div class="stat-value">{{ formatDecimal(tokenStats.avg_tpm) }}</div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-icon">📨</div>
            <div class="stat-label">{{ t('dashboard.avgTPR') }}</div>
            <div class="stat-value">{{ formatDecimal(tokenStats.avg_tpr) }}</div>
          </div>
        </n-gi>
        <n-gi v-for="(m, idx) in tokenStats.top_3_models" :key="idx" :span="2">
          <div class="stat-card">
            <div class="stat-icon">{{ ['🥇','🥈','🥉'][Number(idx)] || '🏅' }}</div>
            <div class="stat-label" style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{{ m.model_name }}</div>
            <div class="stat-value" style="font-size: 18px;">{{ formatTokenNumber(m.total_tokens) }}</div>
          </div>
        </n-gi>
        <n-gi v-if="(!tokenStats.top_3_models || tokenStats.top_3_models.length === 0)" :span="2">
          <div class="stat-card">
            <div class="stat-icon">📭</div>
            <div class="stat-label">{{ t('dashboard.noData') }}</div>
            <div class="stat-value">—</div>
          </div>
        </n-gi>
      </n-grid>
      <n-empty v-else :description="t('dashboard.noData')" style="padding: 20px 0" />
    </n-card>

    <!-- 请求趋势图 -->
    <n-card :bordered="false" size="small" class="glass-card chart-card" style="margin-top: 20px">
      <template #header>
        <span>{{ t('dashboard.requestTrend') }}</span>
      </template>
      <template #header-extra>
        <n-button-group size="small">
          <n-button :type="trendDays === 1 ? 'primary' : 'default'" @click="switchTrend(1)">{{ t('dashboard.today') }}</n-button>
          <n-button :type="trendDays === 7 ? 'primary' : 'default'" @click="switchTrend(7)">7{{ t('dashboard.days') }}</n-button>
        </n-button-group>
      </template>
      <v-chart v-if="trendChartData.length > 0" class="chart" :option="trendOption" autoresize />
      <n-empty v-else :description="t('dashboard.noData')" style="padding: 40px 0" />
    </n-card>

    <!-- 模型分布 + 渠道负载 -->
    <n-grid :cols="2" :x-gap="16" :y-gap="16" style="margin-top: 20px">
      <n-gi>
        <n-card :bordered="false" size="small" class="glass-card chart-card">
          <template #header>{{ t('dashboard.modelDistribution') }}</template>
          <v-chart v-if="topModels.length > 0" class="chart" :option="modelPieOption" autoresize />
          <n-empty v-else :description="t('dashboard.noData')" style="padding: 40px 0" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card :bordered="false" size="small" class="glass-card chart-card">
          <template #header>{{ t('dashboard.channelLoad') }}</template>
          <v-chart v-if="topChannels.length > 0" class="chart" :option="channelBarOption" autoresize />
          <n-empty v-else :description="t('dashboard.noData')" style="padding: 40px 0" />
        </n-card>
      </n-gi>
    </n-grid>

    <!-- 最近异常请求 -->
    <n-card :bordered="false" size="small" class="glass-card" style="margin-top: 20px">
      <template #header>{{ t('dashboard.recentErrors') }}</template>
      <n-data-table
        v-if="recentErrors.length > 0"
        :columns="errorColumns"
        :data="recentErrors"
        :bordered="false"
        size="small"
        :row-props="errorRowProps"
      />
      <n-empty v-else :description="t('dashboard.noErrors')" style="padding: 30px 0" />
    </n-card>
  </n-card>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, PieChart, BarChart } from 'echarts/charts'
import {
  TitleComponent, TooltipComponent, LegendComponent,
  GridComponent, DatasetComponent,
} from 'echarts/components'
import VChart from 'vue-echarts'
import { statsApi } from '../api/stats'

use([
  CanvasRenderer, LineChart, PieChart, BarChart,
  TitleComponent, TooltipComponent, LegendComponent,
  GridComponent, DatasetComponent,
])

const { t } = useI18n()
const router = useRouter()

// 数据
const overview = ref<Record<string, any>>({})
const hourlyTrend = ref<any[]>([])
const topModels = ref<any[]>([])
const topChannels = ref<any[]>([])
const recentErrors = ref<any[]>([])
const loading = ref(false)
const trendDays = ref(7)
const tokenDays = ref(1)
const tokenStats = ref<any>(null)

let refreshTimer: ReturnType<typeof setInterval> | null = null

// 颜色规则
const successRateColor = computed(() => {
  const r = overview.value.success_rate ?? 0
  if (r > 95) return '#52c41a'
  if (r > 80) return '#faad14'
  return '#ff4d4f'
})

const latencyColor = computed(() => {
  const ms = overview.value.avg_latency_ms ?? 0
  if (ms < 2000) return '#52c41a'
  if (ms < 5000) return '#faad14'
  return '#ff4d4f'
})

// 格式化
function formatTokenNumber(n: number): string {
  if (n == null) return '—'
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return Math.round(n).toString()
}

function formatDecimal(n: number): string {
  if (n == null) return '—'
  if (n >= 1000) return (n / 1000).toFixed(2) + 'K'
  return n.toFixed(2)
}

// 趋势图数据（适配小时/每日）
const trendChartData = computed(() => {
  if (trendDays.value === 1 && hourlyTrend.value.length > 0) {
    return hourlyTrend.value
  }
  // 7天模式用 daily_trend
  const dt = overview.value.daily_trend || []
  return dt
})

// 加载数据
async function loadData() {
  loading.value = true
  try {
    const res = await statsApi.dashboard(trendDays.value)
    if (res.data?.data) {
      const d = res.data.data
      overview.value = d
      hourlyTrend.value = d.hourly_trend || []
      topModels.value = d.top_models || []
      topChannels.value = d.top_channels || []
      recentErrors.value = d.recent_errors || []
    }
  } catch { /* ignore */ } finally {
    loading.value = false
  }
}

async function loadTokenStats() {
  try {
    const res = await statsApi.tokenStats(tokenDays.value)
    if (res.data?.data) {
      tokenStats.value = res.data.data
    }
  } catch { /* ignore */ }
}

function switchTrend(days: number) {
  trendDays.value = days
  loadData()
}

function switchTokenDays(days: number) {
  tokenDays.value = days
  loadTokenStats()
}

// 趋势折线图
const trendOption = computed(() => {
  const data = trendChartData.value

  // 当天模式（小时）或每日模式
  const isHourly = trendDays.value === 1 && hourlyTrend.value.length > 0

  const labels = data.map((e: any) => {
    if (isHourly) {
      const parts = e.hour?.split(' ') || ['', '']
      return parts[1] || e.hour
    }
    return e.date || e.hour || ''
  })

  const successData = isHourly
    ? data.map((e: any) => e.success || 0)
    : data.map((e: any) => e.total_requests || 0)

  const failData = isHourly
    ? data.map((e: any) => e.fail || 0)
    : data.map((e: any) => e.fail_requests || 0)

  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    legend: { data: [t('dashboard.success'), t('dashboard.failed')], textStyle: { color: '#bbb' } },
    grid: { left: 50, right: 20, top: 40, bottom: 30 },
    xAxis: { type: 'category', data: labels, axisLine: { lineStyle: { color: '#444' } }, axisLabel: { color: '#bbb' } },
    yAxis: { type: 'value', axisLine: { lineStyle: { color: '#444' } }, splitLine: { lineStyle: { color: '#2a2a3e' } }, axisLabel: { color: '#bbb' } },
    series: [
      { name: t('dashboard.success'), type: 'line', data: successData, smooth: true, itemStyle: { color: '#73d13d' }, lineStyle: { width: 2 }, areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(115,209,61,0.3)' }, { offset: 1, color: 'rgba(115,209,61,0.02)' }] } } },
      { name: t('dashboard.failed'), type: 'line', data: failData, smooth: true, itemStyle: { color: '#ff7875' }, lineStyle: { width: 2 }, areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(255,120,117,0.3)' }, { offset: 1, color: 'rgba(255,120,117,0.02)' }] } } },
    ],
  }
})

// 模型饼图 - 明亮配色
const modelPieOption = computed(() => {
  const data = topModels.value.map((e: any) => ({
    name: e.model_name,
    value: e.total_requests,
  }))
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { orient: 'vertical', right: 10, top: 'center', textStyle: { color: '#ccc', fontSize: 12 } },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      center: ['35%', '50%'],
      avoidLabelOverlap: true,
      itemStyle: { borderRadius: 6, borderColor: '#2a2a3e', borderWidth: 2 },
      label: { show: false },
      emphasis: { label: { show: true, fontSize: 14, fontWeight: 'bold', color: '#fff' } },
      data,
      color: ['#69b1ff', '#95de64', '#ffd666', '#ff7875', '#b37feb', '#5cdbd3'],
    }],
  }
})

// 渠道柱状图 - 明亮渐变
const channelBarOption = computed(() => {
  const names = topChannels.value.map((e: any) => e.channel_name).reverse()
  const values = topChannels.value.map((e: any) => e.total_requests).reverse()
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 120, right: 30, top: 10, bottom: 20 },
    xAxis: { type: 'value', axisLine: { lineStyle: { color: '#444' } }, splitLine: { lineStyle: { color: '#2a2a3e' } }, axisLabel: { color: '#bbb' } },
    yAxis: { type: 'category', data: names, axisLine: { lineStyle: { color: '#444' } }, axisLabel: { color: '#ccc', width: 100, overflow: 'truncate' } },
    series: [{
      type: 'bar',
      data: values,
      itemStyle: {
        borderRadius: [0, 4, 4, 0],
        color: {
          type: 'linear', x: 0, y: 0, x2: 1, y2: 0,
          colorStops: [{ offset: 0, color: '#69b1ff' }, { offset: 1, color: '#95de64' }],
        },
      },
      barWidth: 16,
    }],
  }
})

// 异常表格
const errorColumns = computed(() => [
  { title: t('dashboard.colTime'), key: 'timestamp', width: 160 },
  { title: t('dashboard.colModel'), key: 'model_name', width: 160 },
  { title: t('dashboard.colStatusCode'), key: 'status_code', width: 100, render: (row: any) => h('span', { style: { color: row.status_code >= 300 ? '#ff7875' : '#ffd666' } }, row.status_code) },
  { title: t('dashboard.colLatency'), key: 'latency_ms', width: 100, render: (row: any) => h('span', { style: { color: row.latency_ms > 5000 ? '#ff7875' : '#ffd666' } }, row.latency_ms + 'ms') },
  { title: t('dashboard.colError'), key: 'error_msg', ellipsis: { tooltip: true } },
])

function errorRowProps(row: any) {
  return {
    style: 'cursor: pointer',
    onClick: () => {
      router.push({ name: 'logs', query: { keyword: row.trace_id } })
    },
  }
}

// 生命周期
onMounted(() => {
  loadData()
  loadTokenStats()
  refreshTimer = setInterval(() => {
    loadData()
    loadTokenStats()
  }, 30000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<style scoped>
.stat-card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg, 12px);
  padding: 20px 24px;
  transition: all 0.25s ease;
}
.stat-card:hover {
  border-color: var(--border-light);
  box-shadow: var(--shadow-hover);
}
.stat-icon {
  font-size: 20px;
  margin-bottom: 8px;
}
.stat-label {
  color: var(--text-secondary);
  font-size: 13px;
  margin-bottom: 6px;
}
.stat-value {
  color: var(--text-primary);
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
}
.chart-card :deep(.n-card-header) {
  padding: 12px 20px;
}
.chart {
  height: 300px;
  width: 100%;
}
</style>