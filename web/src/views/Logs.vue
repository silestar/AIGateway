<template>
  <n-card :title="t('logs.title')">
    <n-space vertical>
      <!-- 筛选区 -->
      <n-space align="center">
        <n-input v-model:value="filter.model_name" :placeholder="t('logs.modelPlaceholder')" clearable style="width: 180px" />
        <n-select v-model:value="filter.status" :options="statusOptions" :placeholder="t('logs.statusPlaceholder')" clearable style="width: 140px" />
        <n-date-picker v-model:value="dateRange" type="daterange" clearable />
        <n-button type="primary" @click="fetchLogs">{{ t('common.search') }}</n-button>
      </n-space>

      <!-- 日志表格 -->
      <n-data-table
        :columns="columns"
        :data="logs"
        :pagination="pagination"
        :loading="loading"
        :row-props="(row: any) => ({ style: 'cursor: pointer', onClick: () => showDetail(row) })"
      />

      <!-- 日志详情弹窗 -->
      <n-modal v-model:show="detailVisible" preset="card" :title="t('logs.detail')" style="width: 600px">
        <n-descriptions v-if="detailLog" bordered :column="2" size="small">
          <n-descriptions-item :label="t('logs.timestamp')">{{ detailLog.timestamp }}</n-descriptions-item>
          <n-descriptions-item :label="t('logs.consumer')">{{ detailLog.consumer_id }}</n-descriptions-item>
          <n-descriptions-item :label="t('logs.model')">{{ detailLog.model_name }}</n-descriptions-item>
          <n-descriptions-item :label="t('logs.channel')">{{ detailLog.channel_id }}</n-descriptions-item>
          <n-descriptions-item :label="t('logs.account')">{{ detailLog.account_id }}</n-descriptions-item>
          <n-descriptions-item :label="t('logs.status')">
            <n-tag :type="detailLog.status_code >= 200 && detailLog.status_code < 300 ? 'success' : 'error'" size="small">
              {{ detailLog.status_code }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item :label="t('logs.latency')">{{ detailLog.latency_ms }}ms</n-descriptions-item>
          <n-descriptions-item :label="t('logs.stream')">
            <n-tag :type="detailLog.is_stream ? 'info' : 'default'" size="small">
              {{ detailLog.is_stream ? 'Stream' : 'Sync' }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item v-if="detailLog.error_msg" :label="t('logs.error')" :span="2">
            <n-text type="error">{{ detailLog.error_msg }}</n-text>
          </n-descriptions-item>
          <n-descriptions-item v-if="detailLog.prompt_tokens || detailLog.completion_tokens" :label="t('logs.tokens')">
            {{ detailLog.prompt_tokens }} + {{ detailLog.completion_tokens }} = {{ detailLog.prompt_tokens + detailLog.completion_tokens }}
          </n-descriptions-item>
        </n-descriptions>

        <!-- 重试链 -->
        <template v-if="detailLog?.retry_chain && detailLog.retry_chain.length > 1">
          <n-divider>{{ t('logs.retryChain') }}</n-divider>
          <n-timeline>
            <n-timeline-item
              v-for="(entry, idx) in detailLog.retry_chain"
              :key="idx"
              :type="entry.result === 'success' ? 'success' : 'error'"
              :title="`${t('logs.channel')}: ${entry.channel_id}, ${t('logs.account')}: ${entry.account_id}`"
              :content="entry.error || entry.result"
            />
          </n-timeline>
        </template>
      </n-modal>
    </n-space>
  </n-card>
</template>

<script setup lang="ts">
import { ref, reactive, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NTag } from 'naive-ui'
import { logsApi } from '../api/logs'

const { t } = useI18n()

const loading = ref(false)
const logs = ref<any[]>([])
const total = ref(0)
const detailVisible = ref(false)
const detailLog = ref<any>(null)
const dateRange = ref<[number, number] | null>(null)

const filter = reactive({
  model_name: '',
  status: '' as string,
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  itemCount: 0,
  onChange: (page: number) => {
    pagination.page = page
    fetchLogs()
  },
})

const statusOptions = [
  { label: t('logs.success'), value: 'success' },
  { label: t('logs.failed'), value: 'failed' },
]

const columns = [
  { title: t('logs.timestamp'), key: 'timestamp', width: 180, render: (row: any) => row.timestamp?.substring(0, 19).replace('T', ' ') },
  { title: t('logs.consumer'), key: 'consumer_id', width: 80 },
  { title: t('logs.model'), key: 'model_name', width: 140 },
  { title: t('logs.channel'), key: 'channel_id', width: 80 },
  { title: t('logs.status'), key: 'status_code', width: 80, render: (row: any) => h(NTag, { type: row.status_code >= 200 && row.status_code < 300 ? 'success' : 'error', size: 'small' }, { default: () => row.status_code }) },
  { title: t('logs.latency'), key: 'latency_ms', width: 80, render: (row: any) => `${row.latency_ms}ms` },
  { title: t('logs.stream'), key: 'is_stream', width: 70, render: (row: any) => h(NTag, { type: row.is_stream ? 'info' : 'default', size: 'small' }, { default: () => row.is_stream ? 'S' : 'F' }) },
]

function showDetail(row: any) {
  detailLog.value = row
  // 如果有 retry_chain 且是 JSON 字符串，解析
  if (typeof row.retry_chain === 'string') {
    try { detailLog.value = { ...row, retry_chain: JSON.parse(row.retry_chain) } } catch { /* ignore */ }
  }
  detailVisible.value = true
}

async function fetchLogs() {
  loading.value = true
  try {
    const params: any = {
      page: pagination.page,
      page_size: pagination.pageSize,
    }
    if (filter.model_name) params.model_name = filter.model_name
    if (filter.status) params.status = filter.status
    if (dateRange.value) {
      params.start = new Date(dateRange.value[0]).toISOString().substring(0, 10)
      params.end = new Date(dateRange.value[1]).toISOString().substring(0, 10)
    }

    const res = await logsApi.list(params)
    logs.value = res.data.data || []
    total.value = res.data.total || 0
    pagination.itemCount = total.value
  } catch { /* ignore */ } finally {
    loading.value = false
  }
}

onMounted(() => fetchLogs())
</script>
