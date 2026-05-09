<template>
  <n-card :bordered="false" class="glass-card">
    <!-- 统计卡片行 -->
    <n-grid :cols="5" :x-gap="16" :y-gap="16">
      <n-gi>
        <div class="stat-card" @click="">
          <div class="stat-label">{{ t('dashboard.todayRequests') }}</div>
          <div class="stat-value">{{ overview.total_requests_today || 0 }}</div>
          <div v-if="!overview.total_requests_today" class="stat-hint">{{ t('dashboard.noData') }}</div>
        </div>
      </n-gi>
      <n-gi>
        <div class="stat-card">
          <div class="stat-label">{{ t('dashboard.successRate') }}</div>
          <div class="stat-value" :style="{ color: successRateColor }">
            {{ overview.success_rate != null ? overview.success_rate.toFixed(1) + '%' : '—' }}
          </div>
          <div v-if="overview.total_requests_today === 0" class="stat-hint">{{ t('dashboard.noData') }}</div>
        </div>
      </n-gi>
      <n-gi>
        <div class="stat-card">
          <div class="stat-label">{{ t('dashboard.avgLatency') }}</div>
          <div class="stat-value" :style="{ color: latencyColor }">
            {{ overview.avg_latency_ms != null ? Math.round(overview.avg_latency_ms) + 'ms' : '—' }}
          </div>
          <div v-if="overview.total_requests_today === 0" class="stat-hint">{{ t('dashboard.noData') }}</div>
        </div>
      </n-gi>
      <n-gi>
        <div class="stat-card">
          <div class="stat-label">{{ t('dashboard.activeKeys') }}</div>
          <div class="stat-value">{{ overview.active_keys || 0 }}</div>
        </div>
      </n-gi>
      <n-gi>
        <div class="stat-card">
          <div class="stat-label">{{ t('dashboard.activeChannels') }}</div>
          <div class="stat-value">{{ overview.active_channels || 0 }}</div>
        </div>
      </n-gi>
    </n-grid>

    <!-- 请求趋势图 -->
    <n-card :bordered="false" size="small" class="glass-card chart-card" style="margin-top: 20px">
      <template #header>
        <span>{{ t('dashboard.requestTrend') }}</span>
      </template>
      <template #header-extra>
        <n-button-group size="small">
          <n-button :type="trendDays === 7 ? 'primary' : 'default'" @click="switchTrend(7)">7{{ t('dashboard.days') }}</n-button>
          <n-button :type="trendDays === 30 ? 'primary' : 'default'" @click="switchTrend(30)">30{{ t('dashboard.days') }}</n-button>
        </n-button-group>
      </template>
      <v-chart v-if="hourlyTrend.length > 0" class="chart" :option="trendOption" autoresize />
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

function switchTrend(days: number) {
  trendDays.value = days
  loadData()
}

// 趋势折线图
const trendOption = computed(() => {
  const hours = hourlyTrend.value.map((e: any) => {
    const parts = e.hour?.split(' ') || ['', '']
    return parts[1] || e.hour
  })
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    legend: { data: [t('dashboard.success'), t('dashboard.failed')], textStyle: { color: '#999' } },
    grid: { left: 50, right: 20, top: 40, bottom: 30 },
    xAxis: { type: 'category', data: hours, axisLine: { lineStyle: { color: '#333' } }, axisLabel: { color: '#999' } },
    yAxis: { type: 'value', axisLine: { lineStyle: { color: '#333' } }, splitLine: { lineStyle: { color: '#1e1e1e' } }, axisLabel: { color: '#999' } },
    series: [
      { name: t('dashboard.success'), type: 'line', data: hourlyTrend.value.map((e: any) => e.success), smooth: true, itemStyle: { color: '#52c41a' }, areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(82,196,26,0.25)' }, { offset: 1, color: 'rgba(82,196,26,0.02)' }] } } },
      { name: t('dashboard.failed'), type: 'line', data: hourlyTrend.value.map((e: any) => e.fail), smooth: true, itemStyle: { color: '#ff4d4f' }, areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(255,77,79,0.25)' }, { offset: 1, color: 'rgba(255,77,79,0.02)' }] } } },
    ],
  }
})

// 模型饼图
const modelPieOption = computed(() => {
  const data = topModels.value.map((e: any) => ({
    name: e.model_name,
    value: e.total_requests,
  }))
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { orient: 'vertical', right: 10, top: 'center', textStyle: { color: '#999' } },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      center: ['35%', '50%'],
      avoidLabelOverlap: true,
      itemStyle: { borderRadius: 6, borderColor: '#1a1a2e', borderWidth: 2 },
      label: { show: false },
      emphasis: { label: { show: true, fontSize: 14, fontWeight: 'bold', color: '#e8eaed' } },
      data,
      color: ['#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de', '#9a60b4'],
    }],
  }
})

// 渠道柱状图
const channelBarOption = computed(() => {
  const names = topChannels.value.map((e: any) => e.channel_name).reverse()
  const values = topChannels.value.map((e: any) => e.total_requests).reverse()
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 120, right: 30, top: 10, bottom: 20 },
    xAxis: { type: 'value', axisLine: { lineStyle: { color: '#333' } }, splitLine: { lineStyle: { color: '#1e1e1e' } }, axisLabel: { color: '#999' } },
    yAxis: { type: 'category', data: names, axisLine: { lineStyle: { color: '#333' } }, axisLabel: { color: '#ccc', width: 100, overflow: 'truncate' } },
    series: [{
      type: 'bar',
      data: values,
      itemStyle: {
        borderRadius: [0, 4, 4, 0],
        color: {
          type: 'linear', x: 0, y: 0, x2: 1, y2: 0,
          colorStops: [{ offset: 0, color: '#5470c6' }, { offset: 1, color: '#91cc75' }],
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
  { title: t('dashboard.colStatusCode'), key: 'status_code', width: 100, render: (row: any) => h('span', { style: { color: row.status_code >= 300 ? '#ff4d4f' : '#faad14' } }, row.status_code) },
  { title: t('dashboard.colLatency'), key: 'latency_ms', width: 100, render: (row: any) => h('span', { style: { color: row.latency_ms > 5000 ? '#ff4d4f' : '#faad14' } }, row.latency_ms + 'ms') },
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
  refreshTimer = setInterval(loadData, 30000)
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
  background: rgba(12, 16, 30, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 12px;
  padding: 20px 24px;
  transition: all 0.25s ease;
}
.stat-card:hover {
  border-color: rgba(0, 210, 255, 0.2);
  box-shadow: 0 4px 20px rgba(0, 210, 255, 0.06);
}
.stat-label {
  color: #8e94a0;
  font-size: 13px;
  margin-bottom: 8px;
}
.stat-value {
  color: #e8eaed;
  font-size: 28px;
  font-weight: 700;
}
.stat-hint {
  color: #666;
  font-size: 12px;
  margin-top: 4px;
}
.chart-card :deep(.n-card-header) {
  padding: 12px 20px;
}
.chart {
  height: 300px;
  width: 100%;
}
</style>
