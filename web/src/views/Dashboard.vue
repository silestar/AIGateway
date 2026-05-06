<template>
  <n-card :bordered="false" class="glass-card">
    <n-grid :cols="4" :x-gap="16" :y-gap="16">
      <n-gi>
        <n-card :bordered="false" size="small" class="stat-card">
          <n-statistic :label="t('dashboard.todayRequests')" :value="stats.total_requests_today" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card :bordered="false" size="small" class="stat-card">
          <n-statistic :label="t('dashboard.successRate')" :value="stats.success_rate.toFixed(1)">
            <template #suffix>%</template>
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card :bordered="false" size="small" class="stat-card">
          <n-statistic :label="t('dashboard.avgLatency')">
            <template #default>{{ stats.avg_latency_ms }}</template>
            <template #suffix>ms</template>
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card :bordered="false" size="small" class="stat-card">
          <n-statistic :label="t('dashboard.activeKeys')" :value="stats.active_keys" />
        </n-card>
      </n-gi>
    </n-grid>

    <n-grid :cols="2" :x-gap="16" :y-gap="16" style="margin-top: 20px">
      <n-gi>
        <n-card :bordered="false" :title="t('dashboard.requestTrend')" size="small" class="glass-card">
          <n-data-table v-if="trend.length > 0" :columns="trendColumns" :data="trend" :bordered="false" size="small" />
          <n-empty v-else :description="t('common.noData')" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card :bordered="false" :title="t('dashboard.modelDistribution')" size="small" class="glass-card">
          <n-data-table v-if="models.length > 0" :columns="modelColumns" :data="models" :bordered="false" size="small" />
          <n-empty v-else :description="t('common.noData')" />
        </n-card>
      </n-gi>
    </n-grid>
  </n-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCard, NGrid, NGi, NStatistic, NDataTable, NEmpty } from 'naive-ui'
import { statsApi } from '../api/stats'

const { t } = useI18n()

const stats = ref({
  total_requests_today: 0,
  success_rate: 0,
  avg_latency_ms: 0,
  active_keys: 0,
})

const trend = ref<any[]>([])
const models = ref<any[]>([])

const trendColumns = [
  { title: '日期', key: 'date' },
  { title: '总请求', key: 'total_requests' },
  { title: '成功', key: 'success_requests' },
  { title: '失败', key: 'fail_requests' },
]

const modelColumns = [
  { title: '模型', key: 'model_name' },
  { title: '请求数', key: 'total_requests' },
]

onMounted(async () => {
  try {
    const [dashRes, modelRes] = await Promise.all([
      statsApi.dashboard(),
      statsApi.models(),
    ])
    if (dashRes.data?.data) {
      const d = dashRes.data.data
      stats.value = {
        total_requests_today: d.total_requests_today || 0,
        success_rate: d.success_rate || 0,
        avg_latency_ms: d.avg_latency_ms || 0,
        active_keys: d.active_keys || 0,
      }
      trend.value = d.daily_trend || []
    }
    if (modelRes.data?.data) models.value = modelRes.data.data
  } catch { /* ignore */ }
})
</script>

<style scoped>
.stat-card {
  background: rgba(12, 16, 30, 0.6) !important;
  border: 1px solid rgba(255, 255, 255, 0.05) !important;
  border-radius: 12px !important;
  transition: all 0.25s ease;
}
.stat-card:hover {
  border-color: rgba(0, 210, 255, 0.2) !important;
  box-shadow: 0 4px 20px rgba(0, 210, 255, 0.06);
}
:deep(.n-statistic__label) {
  color: #8e94a0 !important;
  font-size: 13px;
}
:deep(.n-statistic__content) {
  color: #e8eaed !important;
  font-size: 28px;
  font-weight: 700;
}
</style>