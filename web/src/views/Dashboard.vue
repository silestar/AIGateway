<template>
  <div>
    <n-grid :cols="4" :x-gap="16" :y-gap="16">
      <n-gi>
        <n-card>
          <n-statistic :label="t('dashboard.todayRequests')" :value="stats.total_requests_today" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card>
          <n-statistic :label="t('dashboard.successRate')" :value="stats.success_rate.toFixed(1)">
            <template #suffix>%</template>
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card>
          <n-statistic :label="t('dashboard.avgLatency')">
            <template #default>{{ stats.avg_latency_ms }}</template>
            <template #suffix>ms</template>
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card>
          <n-statistic :label="t('dashboard.activeConsumers')" :value="stats.active_consumers" />
        </n-card>
      </n-gi>
    </n-grid>

    <n-grid :cols="2" :x-gap="16" :y-gap="16" style="margin-top: 16px">
      <n-gi>
        <n-card :title="t('dashboard.requestTrend')">
          <div v-if="trend.length > 0">
            <n-table :bordered="false" :single-line="false" size="small">
              <thead>
                <tr>
                  <th>{{ t('stats.date') }}</th>
                  <th>{{ t('stats.total') }}</th>
                  <th>{{ t('stats.success') }}</th>
                  <th>{{ t('stats.failed') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in trend" :key="item.date">
                  <td>{{ item.date }}</td>
                  <td>{{ item.total_requests }}</td>
                  <td>{{ item.success_requests }}</td>
                  <td>{{ item.fail_requests }}</td>
                </tr>
              </tbody>
            </n-table>
          </div>
          <n-empty v-else :description="t('dashboard.noData')" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card :title="t('dashboard.modelDistribution')">
          <div v-if="models.length > 0">
            <n-table :bordered="false" :single-line="false" size="small">
              <thead>
                <tr>
                  <th>{{ t('stats.model') }}</th>
                  <th>{{ t('stats.total') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in models" :key="item.model_name">
                  <td>{{ item.model_name }}</td>
                  <td>{{ item.total_requests }}</td>
                </tr>
              </tbody>
            </n-table>
          </div>
          <n-empty v-else :description="t('dashboard.noData')" />
        </n-card>
      </n-gi>
    </n-grid>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { statsApi } from '../api/stats'

const { t } = useI18n()

const stats = ref({
  total_requests_today: 0,
  success_rate: 0,
  avg_latency_ms: 0,
  total_tokens: 0,
  active_consumers: 0,
  active_channels: 0,
})
const trend = ref<any[]>([])
const models = ref<any[]>([])

onMounted(async () => {
  try {
    const res = await statsApi.dashboard()
    const data = res.data.data
    stats.value = {
      total_requests_today: data.total_requests_today || 0,
      success_rate: data.success_rate || 0,
      avg_latency_ms: data.avg_latency_ms || 0,
      total_tokens: data.total_tokens || 0,
      active_consumers: data.active_consumers || 0,
      active_channels: data.active_channels || 0,
    }
    trend.value = data.last_7_days || []
  } catch { /* 使用默认值 */ }

  try {
    const res = await statsApi.models()
    models.value = res.data.data || []
  } catch { /* ignore */ }
})
</script>
