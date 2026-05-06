<template>
  <n-card :bordered="false" class="glass-card">
    <template #header>
      <h2 class="page-title" style="margin:0">{{ t('stats.title') }}</h2>
    </template>
    <n-tabs type="line" animated>
      <n-tab-pane name="requests" :tab="t('stats.requests')">
        <n-data-table :columns="requestColumns" :data="requestData" :pagination="requestPagination" :loading="loading" />
      </n-tab-pane>
      <n-tab-pane name="models" :tab="t('stats.models')">
        <n-data-table :columns="modelColumns" :data="modelsData" :loading="loading" />
      </n-tab-pane>
      <n-tab-pane name="channels" :tab="t('stats.channels')">
        <n-data-table :columns="channelColumns" :data="channelsData" :loading="loading" />
      </n-tab-pane>
    </n-tabs>
  </n-card>
</template>

<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NTag } from 'naive-ui'
import { statsApi } from '../api/stats'

const { t } = useI18n()
const loading = ref(false)

// 请求统计
const requestData = ref<any[]>([])
const requestPagination = ref({ page: 1, pageSize: 20 })

const requestColumns = [
  { title: t('stats.date'), key: 'date' },
  { title: t('stats.total'), key: 'total_requests' },
  { title: t('stats.success'), key: 'success_requests' },
  { title: t('stats.failed'), key: 'fail_requests' },
]

// 模型分布
const modelsData = ref<any[]>([])

const modelColumns = [
  { title: t('stats.model'), key: 'model_name' },
  { title: t('stats.total'), key: 'total_requests' },
]

// 渠道负载
const channelsData = ref<any[]>([])

const channelColumns = [
  { title: t('stats.channelId'), key: 'channel_id' },
  { title: t('stats.total'), key: 'total_requests' },
  { title: t('stats.successRate'), key: 'success_rate', render: (row: any) => h(NTag, { type: row.success_rate > 95 ? 'success' : row.success_rate > 80 ? 'warning' : 'error', size: 'small' }, { default: () => `${row.success_rate?.toFixed(1)}%` }) },
  { title: t('stats.avgLatency'), key: 'avg_latency_ms', render: (row: any) => `${row.avg_latency_ms}ms` },
]

onMounted(async () => {
  loading.value = true
  try {
    const [reqRes, modelRes, chRes] = await Promise.allSettled([
      statsApi.requests(),
      statsApi.models(),
      statsApi.channels(),
    ])

    if (reqRes.status === 'fulfilled') {
      requestData.value = reqRes.value.data.data || []
    }
    if (modelRes.status === 'fulfilled') {
      modelsData.value = modelRes.value.data.data || []
    }
    if (chRes.status === 'fulfilled') {
      channelsData.value = chRes.value.data.data || []
    }
  } finally {
    loading.value = false
  }
})
</script>
