<template>
  <div class="request-logs-page">
    <!-- 筛选区 -->
    <div class="filter-bar glass-card">
      <!-- 第一行 -->
      <div class="filter-row">
        <n-date-picker
          v-model:value="dateRange"
          type="datetimerange"
          clearable
          :default-time="['00:00:00', '23:59:59']"
          style="width: 340px"
          @update:value="onFilterChange"
        />
        <n-input
          v-model:value="filterKeysName"
          :placeholder="t('requestLogs.selectKeys')"
          clearable
          style="width: 140px"
          @update:value="onFilterChange"
        />
        <n-input
          v-model:value="filterChannelName"
          :placeholder="t('requestLogs.selectChannel')"
          clearable
          style="width: 140px"
          @update:value="onFilterChange"
        />
        <n-input
          v-model:value="filterModel"
          :placeholder="t('requestLogs.modelPlaceholder')"
          clearable
          style="width: 160px"
          @update:value="onFilterChange"
        />
<n-button type="primary" :loading="loading" :disabled="loading" @click="onFilterChange">{{ loading ? t('requestLogs.searching') : t('requestLogs.search') }}</n-button>
        <n-button @click="resetFilters">{{ t('requestLogs.reset') }}</n-button>
        <div style="flex:1"></div>
        <!-- 实时跟踪开关 -->
        <n-tooltip trigger="hover">
          <template #trigger>
            <n-switch v-model:value="liveMode" size="small" @update:value="onLiveModeChange">
              <template #checked></template>
              <template #unchecked></template>
            </n-switch>
          </template>
          {{ liveMode ? t('requestLogs.liveOffHint') : t('requestLogs.liveOnHint') }}
        </n-tooltip>
        <!-- 脉冲指示灯 -->
        <span v-if="liveMode" class="pulse-dot" :title="t('requestLogs.liveActive')"></span>
        <span class="filter-toggle" @click="filterExpanded = !filterExpanded">
          <template v-if="filterExpanded">
            <UpOutlined style="width:12px;height:12px;vertical-align:middle" />
            <span style="font-size:13px;margin-left:2px">{{ t('requestLogs.collapse') }}</span>
          </template>
          <template v-else>
            <DownOutlined style="width:12px;height:12px;vertical-align:middle" />
            <span style="font-size:13px;margin-left:2px">{{ t('requestLogs.expand') }}</span>
          </template>
        </span>
      </div>
      <!-- 第二行（可收起） -->
      <div v-if="filterExpanded" class="filter-row" style="margin-top:10px">
        <n-select
          v-model:value="filterLogType"
          :options="logTypeOptions"
          clearable
          :placeholder="t('requestLogs.selectLogType')"
          style="width: 240px"
          @update:value="onFilterChange"
        />
        <n-select
          v-model:value="filterStatus"
          :options="statusOptions"
          clearable
          :placeholder="t('requestLogs.selectStatus')"
          style="width: 160px"
          @update:value="onFilterChange"
        />
        <n-input
          v-model:value="filterTraceId"
          :placeholder="t('requestLogs.keywordPlaceholder')"
          clearable
          style="width: 240px"
          @update:value="onFilterChange"
        />
      </div>
    </div>

    <!-- 表格区 -->
    <div class="log-table glass-card">
      <n-data-table
        :columns="tableColumns"
        :data="logEntries"
        :row-key="(row: RequestLog) => row.id"
        :loading="loading && !liveMode"
        :bordered="false"
        size="small"
        flex-height
        style="height: calc(100vh - 230px)"
      />
    </div>

    <!-- 右下角新日志提示框 -->
    <transition name="toast-fade">
      <div v-if="toastVisible" class="live-toast">
        <span>{{ t('requestLogs.liveNewLogs', { n: toastCount }) }}</span>
        <span class="toast-close" @click="dismissToast">×</span>
      </div>
    </transition>

    <!-- 底部分页 -->
    <div class="bottom-bar glass-card">
      <span class="log-count">{{ t('requestLogs.total') }}: {{ total }}</span>
      <div style="flex:1"></div>
      <n-pagination
        v-model:page="currentPage"
        :page-count="totalPages"
        :page-size="pageSize"
        size="small"
        show-quick-jumper
        @update:page="onPageChange"
      />
      <n-select v-model:value="pageSize" :options="pageSizeOptions" size="small" style="width:80px;margin-left:8px" @update:value="onPageSizeChange" />
    </div>

    <!-- 详情抽屉 -->
    <n-drawer v-model:show="showDetail" :width="640" placement="right">
      <n-drawer-content :title="t('requestLogs.detail')" closable>
        <template v-if="detailLog">
          <!-- 区块1：基本信息 -->
          <div class="detail-section">
            <div class="detail-section-title">{{ t('requestLogs.detailBasicInfo') }}</div>
            <div class="detail-grid">
              <div class="detail-label">{{ t('requestLogs.detailTraceId') }}</div>
              <div class="detail-value monospace clickable" @click="copyText(detailLog.trace_id)">
                {{ detailLog.trace_id }}
                <span class="copy-hint">📋</span>
              </div>
              <div class="detail-label">{{ t('requestLogs.detailTime') }}</div>
              <div class="detail-value">{{ formatFullTimestamp(detailLog.timestamp) }}</div>
              <template v-if="detailLog.keys_name">
                <div class="detail-label">{{ t('requestLogs.detailKeys') }}</div>
                <div class="detail-value clickable" @click="copyText(detailLog.keys_name)">🔑 {{ detailLog.keys_name }} <span style="opacity:0.5;font-size:11px">📋</span></div>
              </template>
              <template v-if="detailLog.group_name">
                <div class="detail-label">{{ t('requestLogs.detailGroup') }}</div>
                <div class="detail-value">{{ detailLog.group_name }}</div>
              </template>
              <div class="detail-label">{{ t('requestLogs.detailChannel') }}</div>
              <div class="detail-value clickable" @click="copyText(detailLog.channel_name || `#${detailLog.channel_id}`)">
                <template v-if="detailLog.channel_id && detailLog.channel_id > 0">
                  <span class="channel-id">· #{{ detailLog.channel_id }}</span>
                  <span v-if="detailLog.channel_name"> {{ detailLog.channel_name }}</span>
                  <span style="opacity:0.5;font-size:11px;margin-left:4px">📋</span>
                </template>
                <template v-else>-</template>
              </div>
              <div class="detail-label">{{ t('requestLogs.detailAccount') }}</div>
              <div class="detail-value">
                <template v-if="detailLog.account_note">{{ detailLog.account_note }} <span class="account-id">(#{{ detailLog.account_id }})</span></template>
                <template v-else-if="detailLog.account_id"><span class="account-id">#{{ detailLog.account_id }}</span></template>
                <template v-else>-</template>
              </div>
              <div class="detail-label">{{ t('requestLogs.detailResponseTime') }}</div>
              <div class="detail-value">
                <div style="display:flex; gap:6px; align-items:center">
                  <span class="latency-tag-new latency-tag-total">
                    <span class="latency-dot"></span>
                    {{ formatMs(detailLog.latency_ms) }}
                  </span>
                  <template v-if="detailLog.is_stream && detailLog.first_token_ms > 0">
                    <n-tooltip>
                      <template #trigger>
                        <span class="latency-tag-new latency-tag-frt">
                          <span class="latency-dot latency-dot-frt"></span>
                          {{ formatMs(detailLog.first_token_ms) }}
                        </span>
                      </template>
                      {{ t('requestLogs.latencyFrt') }}
                    </n-tooltip>
                  </template>
                </div>
              </div>
              <div class="detail-label">{{ t('requestLogs.detailStatus') }}</div>
              <div class="detail-value">
                <span class="status-badge" :class="is2xx(detailLog.status_code) ? 'status-success' : 'status-error'">
                  {{ detailLog.status_code }}
                </span>
              </div>
            </div>
          </div>

          <n-divider style="margin: 12px 0" />

          <!-- 区块2：错误信息 -->
          <template v-if="detailLog.error_msg">
            <div class="detail-section">
              <div class="detail-section-title text-error">{{ t('requestLogs.detailErrorInfo') }}</div>
              <div class="error-block">
                <div class="error-row"><span class="error-label">Status:</span> {{ detailLog.status_code }}</div>
                <div class="error-row"><span class="error-label">Error:</span> {{ detailLog.error_msg }}</div>
              </div>
            </div>
            <n-divider style="margin: 12px 0" />
          </template>

          <!-- 区块3：重试链路 -->
          <template v-if="parsedRetryChain && parsedRetryChain.length > 0">
            <div class="detail-section">
              <div class="detail-section-title">{{ t('requestLogs.detailRetryChain') }}</div>
              <template v-if="parsedRetryChain.length === 1 && is2xx(parsedRetryChain[0].status_code || 0)">
                <div class="no-retry">{{ t('requestLogs.noRetry') }}</div>
              </template>
              <template v-else>
                <div class="retry-timeline">
                  <div v-for="(entry, idx) in parsedRetryChain" :key="idx" class="retry-item" :class="is2xx(entry.status_code || 0) ? 'retry-success' : 'retry-failed'">
                    <div class="retry-dot"></div>
                    <div class="retry-content">
                      <div class="retry-header">
                        <n-tooltip trigger="hover">
                          <template #trigger><span class="retry-attempt">#{{ idx + 1 }}</span></template>
                          {{ t('requestLogs.attemptPrefix') }}{{ idx + 1 }}{{ t('requestLogs.attemptSuffix') }}
                        </n-tooltip>
                        <span class="retry-separator">·</span>
                        <span class="retry-channel">{{ t('requestLogs.colChannel') }}: <template v-if="entry.channel_name">{{ entry.channel_name }}</template> (#{{ entry.channel_id }})</span>
                        <template v-if="entry.account_id">
                          <span class="retry-separator">·</span>
                          <span class="retry-account">{{ t('requestLogs.detailAccount') }}: <template v-if="entry.account_note">{{ entry.account_note }}</template> (#{{ entry.account_id }})</span>
                        </template>
                      </div>
                      <div class="retry-meta">
                        <span class="latency-tag" :style="latencyColor(entry.latency_ms || 0, entry.status_code || 0)">{{ formatMs(entry.latency_ms || 0) }}</span>
                        <span class="status-badge small" :class="is2xx(entry.status_code || 0) ? 'status-success' : 'status-error'">{{ entry.status_code }}</span>
                        <template v-if="entry.error"><span class="retry-error">{{ entry.error }}</span></template>
                      </div>
                    </div>
                  </div>
                </div>
              </template>
            </div>
            <n-divider style="margin: 12px 0" />
          </template>

          <!-- 区块4：模型链路 -->
          <div class="detail-section">
            <div class="detail-section-title">{{ t('requestLogs.detailModelChain') }}</div>
            <div class="model-chain">
              <div class="model-row">
                <template v-if="detailLog.mapped_model">
                  <img :src="routeIcon" class="model-icon" />{{ t('requestLogs.modelMapped') }}：
                  <n-tag type="info" size="small" class="model-tag" @click="copyText(detailLog.model_name)">{{ detailLog.model_name }}</n-tag>
                  <span class="model-arrow">→</span>
                  <n-tag type="success" size="small" class="model-tag" @click="copyText(detailLog.mapped_model)">{{ detailLog.mapped_model }}</n-tag>
                </template>
                <template v-else>
                  <span class="model-label">{{ detailLog.model_name }}</span>
                </template>
              </div>
              <template v-if="detailLog.upstream_model && detailLog.upstream_model !== detailLog.model_name && detailLog.upstream_model !== detailLog.mapped_model">
                <div class="model-row" style="margin-top:4px">
                  <span class="model-hint warn">⚠️ {{ t('requestLogs.modelRedirected') }}：</span>
                  <span class="model-label warn">{{ detailLog.upstream_model }}</span>
                </div>
              </template>
            </div>
          </div>

          <n-divider style="margin: 12px 0" />

          <!-- 区块5：Token 明细 -->
          <div class="detail-section">
            <div class="detail-section-title">{{ t('requestLogs.detailTokenDetail') }}</div>
            <div class="detail-grid">
              <div class="detail-label">{{ t('requestLogs.detailPromptTokens') }}</div>
              <div class="detail-value monospace">{{ detailLog.prompt_tokens > 0 ? detailLog.prompt_tokens.toLocaleString('en-US') : '-' }}</div>
              <div class="detail-label">{{ t('requestLogs.detailCompletionTokens') }}</div>
              <div class="detail-value monospace">{{ detailLog.completion_tokens > 0 ? detailLog.completion_tokens.toLocaleString('en-US') : '-' }}</div>
              <template v-if="detailLog.cache_tokens > 0">
                <div class="detail-label">{{ t('requestLogs.detailCacheTokens') }}</div>
                <div class="detail-value"><span class="cache-badge">{{ t('requestLogs.cacheDown') }} {{ detailLog.cache_tokens.toLocaleString('en-US') }}</span></div>
              </template>
            </div>
          </div>

          <n-divider style="margin: 12px 0" />

          <!-- 区块6：请求/响应元数据 -->
          <template v-if="parsedRequestMeta && Object.keys(parsedRequestMeta).length > 0">
            <div class="detail-section">
              <div class="detail-section-title">{{ t('requestLogs.detailRequestInfo') }}</div>
              <div class="detail-grid">
                <template v-for="(val, key) in parsedRequestMeta" :key="key">
                  <div class="detail-label monospace">{{ key }}</div>
                  <div class="detail-value">
                    <template v-if="typeof val === 'object' && val !== null">
                      <pre class="inline-json">{{ JSON.stringify(val, null, 2) }}</pre>
                    </template>
                    <template v-else>{{ val }}</template>
                  </div>
                </template>
              </div>
            </div>
            <n-divider style="margin: 12px 0" />
          </template>

          <template v-if="parsedResponseMeta && Object.keys(parsedResponseMeta).length > 0">
            <div class="detail-section">
              <div class="detail-section-title">{{ t('requestLogs.detailResponseInfo') }}</div>
              <div class="detail-grid">
                <template v-for="(val, key) in parsedResponseMeta" :key="key">
                  <div class="detail-label monospace">{{ key }}</div>
                  <div class="detail-value">
                    <template v-if="typeof val === 'object' && val !== null">
                      <pre class="inline-json">{{ JSON.stringify(val, null, 2) }}</pre>
                    </template>
                    <template v-else>{{ val }}</template>
                  </div>
                </template>
              </div>
            </div>
            <n-divider style="margin: 12px 0" />
          </template>

          <!-- 详细请求/响应内容 -->
          <template v-if="detailLog.has_detail === 1">
            <n-collapse>
              <n-collapse-item :title="t('requestLogs.detailContentTitle')" name="detail-content">
                <div v-if="detailContentLoading" style="text-align:center;padding:16px;">{{ t('requestLogs.loading') }}</div>
                <template v-else-if="detailContent">
                  <div class="detail-section">
                    <div class="detail-section-title">{{ t('requestLogs.request') }}</div>
                    <div class="detail-kv">
                      <span class="detail-key">Method</span>
                      <span class="detail-value">{{ detailContent.request.method }}</span>
                    </div>
                    <div class="detail-kv">
                      <span class="detail-key">Path</span>
                      <span class="detail-value monospace">{{ detailContent.request.path }}</span>
                    </div>
                    <div class="detail-kv" v-if="detailContent.request.headers && Object.keys(detailContent.request.headers).length > 0">
                      <span class="detail-key">Headers</span>
                      <div class="code-block-wrap">
                        <span class="code-copy-btn" @click="copyText(toPrettyJSON(detailContent.request.headers))" title="复制">📋</span>
                        <pre class="raw-json">{{ toPrettyJSON(detailContent.request.headers) }}</pre>
                      </div>
                    </div>
                    <div class="detail-kv" v-if="detailContent.request.body">
                      <span class="detail-key">Body</span>
                      <div class="code-block-wrap">
                        <span class="code-copy-btn" @click="copyText(toPrettyJSON(detailContent.request.body))" title="复制">📋</span>
                        <pre class="raw-json">{{ toPrettyJSON(detailContent.request.body) }}</pre>
                      </div>
                    </div>
                  </div>
                  <n-divider style="margin: 12px 0" />
                  <div class="detail-section">
                    <div class="detail-section-title">{{ t('requestLogs.response') }}</div>
                    <div class="detail-kv">
                      <span class="detail-key">Status</span>
                      <span class="detail-value">{{ detailContent.response.status_code }}</span>
                    </div>
                    <div class="detail-kv" v-if="detailContent.response.headers && Object.keys(detailContent.response.headers).length > 0">
                      <span class="detail-key">Headers</span>
                      <div class="code-block-wrap">
                        <span class="code-copy-btn" @click="copyText(toPrettyJSON(detailContent.response.headers))" title="复制">📋</span>
                        <pre class="raw-json">{{ toPrettyJSON(detailContent.response.headers) }}</pre>
                      </div>
                    </div>
                    <div class="detail-kv" v-if="detailContent.response.body">
                      <span class="detail-key">Body</span>
                      <div class="code-block-wrap">
                        <span class="code-copy-btn" @click="copyText(toPrettyJSON(detailContent.response.body))" title="复制">📋</span>
                        <pre class="raw-json">{{ toPrettyJSON(detailContent.response.body) }}</pre>
                      </div>
                    </div>
                  </div>
                </template>
                <div v-else style="color: var(--text-tertiary);padding:8px;">{{ t('requestLogs.contentLoadFailed') }}</div>
              </n-collapse-item>
            </n-collapse>
          </template>

          <!-- 原始数据 -->
          <n-collapse>
            <n-collapse-item :title="t('requestLogs.detailRawData')" name="raw">
              <pre class="raw-json">{{ JSON.stringify(detailLog, null, 2) }}</pre>
            </n-collapse-item>
          </n-collapse>
        </template>
      </n-drawer-content>
    </n-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NSelect,
  NInput,
  NButton,
  NDataTable,
  NDrawer,
  NDrawerContent,
  NDivider,
  NCollapse,
  NCollapseItem,
  NDatePicker,
  NPagination,
  NPopover,
  NSwitch,
  NTooltip,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { requestLogApi, type RequestLog, type RetryChainEntry, type LogDetailContent } from '../api/logs'
import { systemApi } from '../api/system'
import { UpOutlined, DownOutlined, ExclamationCircleOutlined } from '@vicons/antd'
import routeIcon from '../assets/icons/route.svg'

const { t } = useI18n()
const message = useMessage()

// === 筛选状态 ===
const dateRange = ref<[number, number] | null>(null)
const filterKeysName = ref('')
const filterChannelName = ref('')
const filterModel = ref('')
const filterLogType = ref<string | null>(null)
const filterStatus = ref<string | null>(null)
const filterTraceId = ref('')
const filterExpanded = ref(false)

// === 分页 ===
const currentPage = ref(1)
const pageSize = ref(30)
const total = ref(0)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

// === 数据 ===
const logEntries = ref<RequestLog[]>([])
const loading = ref(false)

// === 实时跟踪 ===
const liveMode = ref(false)
let liveTimer: ReturnType<typeof setInterval> | null = null
let liveIntervalMs = 5000 // 默认 5 秒
let lastFetchTime = Date.now() // 上次刷新时间戳

// Toast 状态
const toastVisible = ref(false)
const toastCount = ref(0)
let toastTimer: ReturnType<typeof setTimeout> | null = null

function showToast(n: number) {
  toastCount.value = n
  toastVisible.value = true
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toastVisible.value = false }, 3000)
}

function dismissToast() {
  toastVisible.value = false
  if (toastTimer) { clearTimeout(toastTimer); toastTimer = null }
}

// === 详情 ===
const showDetail = ref(false)
const detailLog = ref<RequestLog | null>(null)
const detailContentLoading = ref(false)
const detailContent = ref<LogDetailContent | null>(null)

// === 闪烁行 ID 集合（已废弃，改用 DOM 直接操作）===

// === 选项 ===
const pageSizeOptions = [
  { label: '20', value: 20 },
  { label: '30', value: 30 },
  { label: '50', value: 50 },
  { label: '100', value: 100 },
]

const logTypeOptions = [
  { label: t('requestLogs.logTypeConsumption'), value: 'consumption' },
  { label: t('requestLogs.logTypeProbe'), value: 'probe' },
  { label: t('requestLogs.logTypeHealthCheck'), value: 'health_check' },
]

const statusOptions = [
  { label: t('requestLogs.success'), value: 'success' },
  { label: t('requestLogs.failed'), value: 'failed' },
]

// === Computed: 解析 ===
const parsedRetryChain = computed<RetryChainEntry[]>(() => {
  if (!detailLog.value?.retry_chain) return []
  const rc = detailLog.value.retry_chain
  if (typeof rc === 'string') { try { return JSON.parse(rc) } catch { return [] } }
  return Array.isArray(rc) ? rc : []
})

const parsedRequestMeta = computed<Record<string, unknown> | null>(() => {
  if (!detailLog.value?.request_meta) return null
  const rm = detailLog.value.request_meta
  if (typeof rm === 'string') { try { return JSON.parse(rm) } catch { return null } }
  return rm && typeof rm === 'object' ? rm as Record<string, unknown> : null
})

const parsedResponseMeta = computed<Record<string, unknown> | null>(() => {
  if (!detailLog.value?.response_meta) return null
  const rm = detailLog.value.response_meta
  if (typeof rm === 'string') { try { return JSON.parse(rm) } catch { return null } }
  return rm && typeof rm === 'object' ? rm as Record<string, unknown> : null
})

// === 格式化 ===
function formatMs(ms: number): string {
  if (ms <= 0) return '0ms'
  if (ms < 1000) return `${Math.round(ms)}ms`
  const totalSec = ms / 1000
  if (totalSec < 60) {
    const sec = totalSec >= 10 ? Math.round(totalSec) : Math.round(totalSec * 10) / 10
    return `${sec}s`
  }
  const min = Math.floor(totalSec / 60)
  const sec = Math.round(totalSec % 60)
  return sec > 0 ? `${min}m${sec}s` : `${min}m`
}

function formatFullTimestamp(ts?: string): string {
  if (!ts) return ''
  try {
    const d = new Date(ts)
    const Y = d.getFullYear(), M = String(d.getMonth() + 1).padStart(2, '0'), D = String(d.getDate()).padStart(2, '0')
    const h = String(d.getHours()).padStart(2, '0'), m = String(d.getMinutes()).padStart(2, '0'), s = String(d.getSeconds()).padStart(2, '0'), ms = String(d.getMilliseconds()).padStart(3, '0')
    return `${Y}-${M}-${D} ${h}:${m}:${s}.${ms}`
  } catch { return ts }
}

function formatListTime(ts?: string): string {
  if (!ts) return ''
  try {
    const d = new Date(ts)
    const Y = d.getFullYear(), M = String(d.getMonth() + 1).padStart(2, '0'), D = String(d.getDate()).padStart(2, '0')
    const h = String(d.getHours()).padStart(2, '0'), m = String(d.getMinutes()).padStart(2, '0'), s = String(d.getSeconds()).padStart(2, '0')
    return `${Y}-${M}-${D} ${h}:${m}:${s}`
  } catch { return ts }
}

// === 颜色 ===
function is2xx(code: number): boolean { return code >= 200 && code < 300 }

function latencyColor(ms: number, statusCode: number): Record<string, string> {
  if (!is2xx(statusCode)) return { color: '#ff4d4f' }
  if (ms < 500) return { color: '#52c41a' }
  if (ms < 2000) return { color: '#f0c040' }
  return { color: '#ff4d4f' }
}

// === 日志类型颜色 ===
const logTypeColor: Record<string, string> = { consumption: '#61affe', probe: '#fca130', health_check: '#50e3c2' }
const logTypeLabel: Record<string, string> = {
  consumption: t('requestLogs.logTypeConsumption'),
  probe: t('requestLogs.logTypeProbe'),
  health_check: t('requestLogs.logTypeHealthCheck'),
}

// === 类型标签色 ===
const tableColumns = computed<DataTableColumns<RequestLog>>(() => [
  {
    title: t('requestLogs.colTime'),
    key: 'time',
    width: 175,
    render: (row) => {
      const time = formatListTime(row.timestamp)
      const typeColor = logTypeColor[row.log_type] || '#999'
      const typeLvl = logTypeLabel[row.log_type] || row.log_type
      return h('div', { style: { lineHeight: '1.4' } }, [
        h('div', { style: { fontSize: '12px', fontFamily: "'JetBrains Mono', monospace" } }, time),
        h('span', { style: { fontSize: '10px', color: typeColor, border: `1px solid ${typeColor}40`, borderRadius: '3px', padding: '0 4px', background: `${typeColor}15` } }, typeLvl),
      ])
    },
  },
  {
    title: t('requestLogs.colChannel'),
    key: 'channel',
    width: 130,
    render: (row) => {
      if (!row.channel_id || row.channel_id <= 0) return h('span', { style: { color: 'var(--n-text-color-3)' } }, '-')
      const line1 = h('div', { style: { fontSize: '12px', lineHeight: '1.5' } }, [
        h('span', { style: { color: '#ff6b6b', fontFamily: "'JetBrains Mono', monospace" } }, [
          h('span', { style: { display: 'inline-block', width: '6px', height: '6px', borderRadius: '50%', background: 'rgb(255, 107, 107)', flexShrink: '0' } }),
          ` #${row.channel_id}`,
        ]),
        row.channel_name ? h('span', { style: { color: 'var(--n-text-color-1)', marginLeft: '4px' } }, ` ${row.channel_name}`) : null,
      ])
      const children = [line1]
      // 第二行：账号信息
      if (row.account_id && row.account_id > 0) {
        const note = row.account_note || ''
        const truncated = note.length > 15 ? note.slice(0, 15) + '...' : note
        const line2 = h('div', { style: { fontSize: '11px', lineHeight: '1.4', color: 'var(--n-text-color-3)' } }, [
          h('span', { style: { color: '#f0a020', fontFamily: "'JetBrains Mono', monospace" } }, `- #${row.account_id}`),
          note ? h('span', { style: { color: 'var(--n-text-color-3)', marginLeft: '4px' } }, truncated) : null,
        ])
        children.push(line2)
      }
      return h('div', { style: { lineHeight: '1.4', cursor: 'pointer' }, onClick: (e: Event) => { e.stopPropagation(); copyText(row.channel_name || `#${row.channel_id}`) } }, children)
    },
  },
  {
    title: t('requestLogs.colKeys'),
    key: 'keys',
    width: 140,
    render: (row) => {
      if (!row.keys_name && (!row.keys_id || row.keys_id <= 0)) return h('span', { style: { color: 'var(--n-text-color-3)' } }, '-')
      const display = row.keys_name || `#${row.keys_id}`
      return h('span', {
        style: {
          display: 'inline-flex', alignItems: 'center', gap: '4px',
          fontSize: '12px', padding: '1px 8px', borderRadius: '4px',
          background: 'rgba(46, 179, 184, 0.12)', border: '1px solid rgba(46, 179, 184, 0.3)',
          color: 'rgba(46, 179, 184, 0.82)', cursor: 'pointer',
        },
        onClick: (e: Event) => { e.stopPropagation(); copyText(row.keys_name || `#${row.keys_id}`) },
      }, [
        h('span', null, '🔑'),
        h('span', null, display),
      ])
    },
  },
  {
    title: t('requestLogs.colModel'),
    key: 'model',
    width: 140,
    render: (row) => {
      const modelName = row.model_name || '-'
      // 只有当前展示的模型名不等于上游实际请求的模型名时才显示映射图标
      // 即 model_name 是自定义映射别名（display_model_name !== mapped_model）
      const isMapped = !!row.mapped_model && row.model_name !== row.mapped_model
      const children: any[] = [
        h('span', {
          style: {
            display: 'inline-flex', alignItems: 'center', gap: '2px',
            fontSize: '12px', padding: '1px 8px', borderRadius: '4px',
            background: 'lab(36.1758% 69.8525 -80.0381 / 0.12)', border: '1px solid lab(36.1758% 69.8525 -80.0381 / 0.3)',
            color: 'lab(36.1758% 69.8525 -80.0381)', cursor: 'pointer',
          },
          onClick: (e: Event) => { e.stopPropagation(); copyText(modelName) },
        }, [
          h('span', { style: { display: 'inline-block', width: '5px', height: '5px', borderRadius: '50%', background: 'lab(36.1758% 69.8525 -80.0381)', marginRight: '4px', verticalAlign: 'middle' } }),
          h('span', null, modelName),
        ]),
      ]
      if (isMapped) {
        children.push(
          h(NPopover, { trigger: 'click', placement: 'bottom', style: { padding: '10px 14px' } }, {
            trigger: () => h('img', {
              src: routeIcon,
              style: { marginLeft: '5px', cursor: 'pointer', width: '14px', height: '14px', verticalAlign: 'middle', opacity: '0.7' },
            }),
            default: () => h('div', { style: { fontSize: '12px', lineHeight: '2', minWidth: '240px' } }, [
              h('div', { style: { display: 'flex', justifyContent: 'space-between' } }, [
                h('span', { style: { color: 'var(--n-text-color-3)' } }, t('requestLogs.mapRequestModel')),
                h('span', { style: { fontFamily: "'JetBrains Mono', monospace", color: '#f0c040' } }, modelName),
              ]),
              h('div', { style: { display: 'flex', justifyContent: 'space-between' } }, [
                h('span', { style: { color: 'var(--n-text-color-3)' } }, t('requestLogs.mapActualModel')),
                h('span', { style: { fontFamily: "'JetBrains Mono', monospace", color: '#f0c040' } }, row.mapped_model),
              ]),
            ]),
          })
        )
      }
      return h('div', { style: { display: 'inline-flex', alignItems: 'center', position: 'relative' } }, children)
    },
  },
  {
    title: t('requestLogs.colLatency'),
    key: 'latency',
    width: 170,
    render: (row) => {
      const hasFrt = row.is_stream && (row as any).first_token_ms > 0
      const isClientGone = is2xx(row.status_code) && row.error_msg === 'client_gone'
      const tagBase = {
        display: 'inline-flex', alignItems: 'center', gap: '4px',
        borderRadius: '6px', padding: '1px 6px',
        fontFamily: "'JetBrains Mono', monospace", fontSize: '12px', fontWeight: '500', lineHeight: '1.5',
      }
      const tags: any[] = [
        h('span', {
          style: {
            ...tagBase,
            border: '1px solid rgba(245, 158, 11, 0.45)',
            background: 'rgba(245, 158, 11, 0.08)',
            color: 'rgba(251, 191, 36, 0.85)',
          }
        }, [
          h('span', {
            style: {
              display: 'inline-block', width: '6px', height: '6px',
              borderRadius: '50%', background: 'rgba(245, 158, 11, 0.8)', flexShrink: '0',
            }
          }),
          formatMs(row.latency_ms),
        ]),
      ]
      if (hasFrt) {
        tags.push(
          h(NTooltip, null, {
            trigger: () => h('span', {
              style: {
                ...tagBase,
                border: '1px solid rgba(34, 197, 94, 0.5)',
                background: 'rgba(34, 197, 94, 0.08)',
                color: 'rgba(74, 222, 128, 0.85)',
              }
            }, [
              h('span', {
                style: {
                  display: 'inline-block', width: '6px', height: '6px',
                  borderRadius: '50%', background: 'rgba(34, 197, 94, 0.8)', flexShrink: '0',
                }
              }),
              formatMs((row as any).first_token_ms),
            ]),
            default: () => t('requestLogs.latencyFrt')
          })
        )
      }
      return h('div', { style: { lineHeight: '1.6' } }, [
        h('div', { style: { display: 'flex', gap: '6px', alignItems: 'center' } }, tags),
        h('div', { style: { display: 'flex', alignItems: 'center', gap: '4px', fontSize: '11px', color: isClientGone ? '#ff4d4f' : 'var(--n-text-color-3)' } }, [
          row.is_stream ? t('requestLogs.stream') : t('requestLogs.nonStream'),
          isClientGone ? h(NTooltip, { placement: 'bottom' }, {
            trigger: () => h(ExclamationCircleOutlined, {
              style: {
                color: '#ff4d4f',
                width: '11px',
                height: '11px',
                cursor: 'pointer',
                verticalAlign: 'middle',
              },
            }),
            default: () => h('div', { style: { fontSize: '12px', lineHeight: '1.6' } }, [
              h('div', { style: { fontWeight: '600', color: '#ff4d4f' } }, t('requestLogs.streamStatusError')),
              h('div', { style: { color: 'var(--n-text-color-3)' } }, t('requestLogs.clientGone')),
            ]),
          }) : null,
        ]),
      ])
    },
  },
  {
    title: t('requestLogs.colTokens'),
    key: 'tokens',
    width: 120,
render: (row) => {
      const fmtNum = (n: number) => n > 0 ? n.toLocaleString('en-US') : null
      const hasPrompt = fmtNum(row.prompt_tokens)
      const hasCompletion = fmtNum(row.completion_tokens)
      let tokenText = '-'
      if (hasPrompt && hasCompletion) tokenText = hasPrompt + ' / ' + hasCompletion
      else if (hasPrompt) tokenText = hasPrompt
      else if (hasCompletion) tokenText = hasCompletion
      const tokenFont = { fontSize: '12px', fontFamily: "'JetBrains Mono', monospace" }
      const children: any[] = [
        h('div', { style: tokenFont }, tokenText),
      ]
      if (row.cache_tokens > 0) {
        children.push(h('div', { style: { fontSize: '10px', color: '#52c41a' } }, `缓存↓ ${row.cache_tokens.toLocaleString('en-US')}`))
      }
      return h('div', { style: { lineHeight: '1.4' } }, children)
    },
  },
  {
    title: t('requestLogs.colStatus'),
    key: 'status',
    width: 70,
    render: (row) => {
      const ok = is2xx(row.status_code)
      const color = ok ? '#52c41a' : '#ff4d4f'
      return h('span', {
        style: {
          display: 'inline-block',
          padding: '1px 8px',
          borderRadius: '4px',
          fontSize: '12px',
          fontWeight: '600',
          fontFamily: "'JetBrains Mono', monospace",
          color,
          background: ok ? 'rgba(82, 196, 26, 0.1)' : 'rgba(255, 77, 79, 0.1)',
          border: ok ? '1px solid rgba(82, 196, 26, 0.3)' : '1px solid rgba(255, 77, 79, 0.3)',
        },
      }, String(row.status_code))
    },
  },
  {
    title: '',
    key: 'has_detail',
    width: 36,
    render: (row) => {
      if (row.has_detail !== 1) return null
      return h('span', {
        style: { fontSize: '14px', opacity: '0.7', cursor: 'help' },
        title: '含详细请求内容',
      }, '📄')
    },
  },
  {
    title: '',
    key: 'detail',
    width: 40,
    render: (row) => {
      return h('span', {
        style: {
          cursor: 'pointer',
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          width: '28px',
          height: '28px',
          borderRadius: '6px',
          background: 'rgba(0, 210, 255, 0.08)',
          border: '1px solid rgba(0, 210, 255, 0.2)',
          color: '#00d2ff',
          fontSize: '16px',
          transition: 'all 0.2s',
        },
        onClick: (e: Event) => { e.stopPropagation(); openDetail(row) },
        onMouseenter: (e: MouseEvent) => {
          (e.target as HTMLElement).style.background = 'rgba(0, 210, 255, 0.15)'
          ;(e.target as HTMLElement).style.borderColor = '#00d2ff'
        },
        onMouseleave: (e: MouseEvent) => {
          (e.target as HTMLElement).style.background = 'rgba(0, 210, 255, 0.08)'
          ;(e.target as HTMLElement).style.borderColor = 'rgba(0, 210, 255, 0.2)'
        },
      }, '▶')
    },
  },
])

// === 映射弹出层状态 ===
// === 操作 ===
function openDetail(row: RequestLog) {
  detailLog.value = row
  detailContent.value = null
  showDetail.value = true
  // 如果有详细内容，异步加载
  if (row.has_detail === 1) {
    detailContentLoading.value = true
    requestLogApi.getDetail(row.id).then(res => {
      const data = (res.data as any)?.data
      detailContent.value = data || null
    }).catch(() => {
      detailContent.value = null
    }).finally(() => {
      detailContentLoading.value = false
    })
  }
}

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    message.success(t('common.copied'))
  }).catch(() => {
    message.error(t('common.copyFailed'))
  })
}

function toPrettyJSON(obj: unknown): string {
  try {
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(obj)
  }
}

// === 加载列表 ===
async function fetchLogs(silent: boolean = false) {
  if (!silent) loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      page_size: pageSize.value,
    }
    if (dateRange.value) {
      params.start = new Date(dateRange.value[0]).toISOString()
      params.end = new Date(dateRange.value[1]).toISOString()
    }
    if (filterKeysName.value) params.keys_name = filterKeysName.value
    if (filterChannelName.value) params.channel_name = filterChannelName.value
    if (filterModel.value) params.model_name = filterModel.value
    if (filterLogType.value) params.log_types = filterLogType.value
    if (filterStatus.value) params.status = filterStatus.value
    if (filterTraceId.value) params.trace_id = filterTraceId.value

    const res = await requestLogApi.list(params as any)
    const data = res.data as any

    if (liveMode.value && silent && logEntries.value.length > 0) {
      // 实时跟踪：新日志插入顶部
      const newLogs = (data?.data || []) as RequestLog[]
      const existIds = new Set(logEntries.value.map(e => e.id))
      const fresh = newLogs.filter(e => !existIds.has(e.id))
      if (fresh.length > 0) {
        logEntries.value = [...fresh, ...logEntries.value]
        total.value += fresh.length
        // DOM 层级：直接操作新行 td 的 transition 实现绿色渐变
        const freshIds = new Set(fresh.map(f => f.id))
        fresh.forEach((_, idx) => {
          const delay = idx * 300
          setTimeout(() => {
            const trs = document.querySelectorAll('.log-table .n-data-table-tr')
            trs.forEach(tr => {
              const rowId = (tr as any).__vnode?.key
              if (rowId != null && freshIds.has(Number(rowId))) {
                const tds = tr.querySelectorAll('td')
                tds.forEach(td => {
                  const el = td as HTMLElement
                  el.style.transition = 'background-color 1.5s ease-out'
                  el.style.backgroundColor = 'rgba(54, 179, 126, 0.08)'
                  requestAnimationFrame(() => {
                    el.style.backgroundColor = 'transparent'
                  })
                })
              }
            })
          }, delay)
        })
        // 右下角 Toast
        showToast(fresh.length)
      }
    } else {
      logEntries.value = data?.data || []
      total.value = data?.total || 0
    }
  } catch (e: any) {
    if (!silent) message.error(e?.message || 'Failed to fetch logs')
  } finally {
    loading.value = false
    lastFetchTime = Date.now()
  }
}

// === 筛选 ===
function onFilterChange() {
  if (liveMode.value) { liveMode.value = false; stopLiveMode() }
  currentPage.value = 1
  fetchLogs()
}

function resetFilters() {
  // 重置为当天时间区间
  const todayStart = new Date()
  todayStart.setHours(0, 0, 0, 0)
  const todayEnd = new Date()
  todayEnd.setHours(23, 59, 59, 999)
  dateRange.value = [todayStart.getTime(), todayEnd.getTime()]
  filterKeysName.value = ''
  filterChannelName.value = ''
  filterModel.value = ''
  filterLogType.value = null
  filterStatus.value = null
  filterTraceId.value = ''
  currentPage.value = 1
  if (liveMode.value) { liveMode.value = false; stopLiveMode() }
  fetchLogs()
}

// === 分页 ===
function onPageChange(page: number) { currentPage.value = page; fetchLogs() }
function onPageSizeChange(size: number) { pageSize.value = size; currentPage.value = 1; fetchLogs() }

// === 实时跟踪 ===
function onLiveModeChange(val: boolean) {
  if (val) {
    // 每次开启实时追踪时重新获取刷新间隔
    loadRefreshInterval().then(() => startLiveMode())
  } else {
    stopLiveMode()
  }
}

function startLiveMode() {
  if (liveTimer) clearInterval(liveTimer)
  liveTimer = setInterval(() => fetchLogs(true), liveIntervalMs)
}

function stopLiveMode() {
  if (liveTimer) { clearInterval(liveTimer); liveTimer = null }
  dismissToast()
}

// 获取刷新间隔配置
async function loadRefreshInterval() {
  try {
    const res = await systemApi.getConfig()
    const data = (res.data as any)?.data
    const interval = data?.log?.refresh_interval
    if (interval && interval >= 3 && interval <= 60) {
      liveIntervalMs = interval * 1000
    }
  } catch { /* use default */ }
}

// === 生命周期 ===
onMounted(() => {
  const now = new Date()
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0)
  const end = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 23, 59, 59)
  dateRange.value = [start.getTime(), end.getTime()]
  loadRefreshInterval()
  fetchLogs()

  // 标签页切回时自动刷新
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onUnmounted(() => {
  stopLiveMode()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})

// 标签页可见性变化：切回页面时若距上次刷新>15秒且未开启实时追踪则自动刷新
function onVisibilityChange() {
  if (document.visibilityState === 'visible' && !liveMode.value) {
    if (Date.now() - lastFetchTime > 15000) {
      fetchLogs()
    }
  }
}
</script>

<style scoped>
.request-logs-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
}

/* === 筛选区 === */
.filter-bar {
  padding: 14px 16px;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-toggle {
  cursor: pointer;
  font-size: 14px;
  color: var(--n-text-color-3);
  padding: 4px 8px;
  border-radius: 4px;
  transition: background 0.2s;
  user-select: none;
}

.filter-toggle:hover {
  background: rgba(255,255,255,0.08);
}

/* === 表格 === */
.log-table {
  flex: 1;
  min-height: 0;
}

.log-table :deep(.n-data-table) {
  --n-td-color: transparent;
  --n-th-color: rgba(16, 22, 42, 0.5);
}

.log-table :deep(.n-data-table-td) {
  font-size: 12px;
  padding: 4px 8px !important;
}

.log-table :deep(.n-data-table-th) {
  font-size: 14px;
  padding: 6px 8px !important;
}

/* 新行淡入动画：通过 JS 直接操作 DOM td 实现 */

/* === 底部栏 === */
.bottom-bar {
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.live-label {
  font-size: 13px;
  cursor: help;
}

.log-count {
  font-size: 12px;
  color: var(--n-text-color-3);
  margin-left: 4px;
}

/* === 耗时颜色 === */
.latency-tag { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
.latency-ok { color: #52c41a; }
.latency-warn { color: #f0c040; }
.latency-error { color: #ff4d4f; }

/* === 新延迟 Tag（new-api 风格） === */
.latency-tag-new {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: 6px;
  padding: 1px 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  font-weight: 500;
  line-height: 1.5;
}

.latency-tag-total {
  border: 1px solid rgba(245, 158, 11, 0.45);
  background: rgba(245, 158, 11, 0.08);
  color: rgba(251, 191, 36, 0.85);
}

.latency-tag-upstream {
  border: 1px solid rgba(244, 63, 94, 0.5);
  background: rgba(244, 63, 94, 0.08);
  color: rgba(251, 113, 133, 0.85);
}

.latency-tag-frt {
  border: 1px solid rgba(34, 197, 94, 0.5);
  background: rgba(34, 197, 94, 0.08);
  color: rgba(74, 222, 128, 0.85);
}

.latency-dot-frt {
  background: rgba(34, 197, 94, 0.8);
}

.latency-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgba(245, 158, 11, 0.8);
  flex-shrink: 0;
}

/* === 状态标签 === */
.status-badge {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  font-family: 'JetBrains Mono', monospace;
}

.status-success {
  color: #52c41a;
  background: rgba(82, 196, 26, 0.1);
  border: 1px solid rgba(82, 196, 26, 0.3);
}

.status-error {
  color: #ff4d4f;
  background: rgba(255, 77, 79, 0.1);
  border: 1px solid rgba(255, 77, 79, 0.3);
}

.text-success { color: #52c41a; margin-left: 6px; font-size: 12px; }
.text-error { color: #ff4d4f; margin-left: 6px; font-size: 12px; }

/* === 详情抽屉 === */
.detail-section { margin-bottom: 4px; }

.detail-section-title {
  font-size: 13px;
  font-weight: 600;
  color: #00d2ff;
  margin-bottom: 10px;
  letter-spacing: 0.5px;
}

.detail-grid {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 6px 16px;
  align-items: baseline;
}

.detail-label {
  font-size: 12px;
  color: var(--n-text-color-3, #999);
  white-space: nowrap;
  min-width: 80px;
}

.detail-value {
  font-size: 13px;
  color: var(--n-text-color);
  word-break: break-all;
}

.detail-value.monospace { font-family: 'JetBrains Mono', monospace; font-size: 12px; }

.detail-value.clickable {
  cursor: pointer;
  color: #00d2ff;
  transition: opacity 0.2s;
}

.detail-value.clickable:hover { opacity: 0.8; }

.copy-hint { margin-left: 4px; opacity: 0.5; font-size: 11px; }

.channel-id { color: #ff6b6b; font-family: 'JetBrains Mono', monospace; font-size: 12px; }
.account-id { color: #999; font-family: 'JetBrains Mono', monospace; font-size: 12px; }

.error-block {
  background: rgba(255, 77, 79, 0.08);
  border: 1px solid rgba(255, 77, 79, 0.2);
  border-radius: 6px;
  padding: 12px;
  font-size: 12px;
}

.error-row { margin-bottom: 4px; }
.error-row:last-child { margin-bottom: 0; }
.error-label { color: #ff4d4f; font-weight: 600; margin-right: 8px; }

.retry-timeline { border-left: 2px solid var(--n-border-color, #333); margin-left: 8px; padding-left: 16px; }
.retry-item { position: relative; margin-bottom: 12px; }
.retry-item:last-child { margin-bottom: 0; }
.retry-dot { position: absolute; left: -21px; top: 4px; width: 10px; height: 10px; border-radius: 50%; border: 2px solid; }
.retry-success .retry-dot { border-color: #52c41a; background: rgba(82, 196, 26, 0.2); }
.retry-failed .retry-dot { border-color: #ff4d4f; background: rgba(255, 77, 79, 0.2); }
.retry-content { font-size: 12px; }
.retry-header { margin-bottom: 4px; }
.retry-attempt { font-weight: 600; color: var(--n-text-color); }
.retry-channel { color: #ff6b6b; font-family: 'JetBrains Mono', monospace; margin-left: 6px; }
.retry-account { color: #999; margin-left: 6px; }
.retry-meta { display: flex; align-items: center; gap: 8px; }
.retry-error { color: #ff4d4f; font-size: 11px; }
.no-retry { font-size: 12px; color: #52c41a; padding: 4px 0; }

.model-chain { display: flex; align-items: center; flex-wrap: wrap; gap: 4px; }
.model-row { display: flex; align-items: center; gap: 4px; font-size: 12px; }
.model-label { font-family: 'JetBrains Mono', monospace; font-weight: 600; }
.model-label.warn { color: #f0c040; }
.model-tag { cursor: pointer; font-family: 'JetBrains Mono', monospace; }
.model-hint { font-size: 11px; color: #999; }
.model-hint.warn { color: #f0c040; }
.model-icon { width: 14px; height: 14px; vertical-align: middle; margin-right: 3px; opacity: 0.7; }
.model-arrow { color: #999; font-size: 14px; }

.cache-badge {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 11px;
  color: #52c41a;
  background: rgba(82, 196, 26, 0.1);
  border: 1px solid rgba(82, 196, 26, 0.3);
}

.inline-json {
  background: rgba(255, 255, 255, 0.06);
  color: #e0e0e0;
  padding: 8px;
  border-radius: 6px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
  max-height: 200px;
  overflow: auto;
}

.raw-json {
  background: #000;
  color: #fff;
  padding: 16px;
  border-radius: 8px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  overflow: auto;
  max-height: calc(100vh - 400px);
  margin: 0;
}

.code-block-wrap {
  position: relative;
}

.code-copy-btn {
  position: absolute;
  top: 6px;
  right: 8px;
  z-index: 2;
  cursor: pointer;
  font-size: 14px;
  opacity: 0.5;
  transition: opacity 0.2s;
  user-select: none;
}
.code-copy-btn:hover {
  opacity: 1;
}

.code-block-wrap .raw-json {
  max-height: 150px;
  overflow-y: auto;
  /* 滚动条样式 */
  scrollbar-width: thin;
  scrollbar-color: rgba(255,255,255,0.15) transparent;
}
.code-block-wrap .raw-json::-webkit-scrollbar {
  width: 5px;
}
.code-block-wrap .raw-json::-webkit-scrollbar-track {
  background: transparent;
}
.code-block-wrap .raw-json::-webkit-scrollbar-thumb {
  background: rgba(255,255,255,0.15);
  border-radius: 3px;
}
.code-block-wrap .raw-json::-webkit-scrollbar-thumb:hover {
  background: rgba(255,255,255,0.3);
}

/* === 脉冲指示灯 === */
.pulse-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #36b37e;
  animation: pulse-blink 2s ease-in-out infinite;
  margin-left: 4px;
  vertical-align: middle;
}

@keyframes pulse-blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.15; }
}

/* === 右下角 Toast === */
.live-toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 1000;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 20px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.75);
  color: #fff;
  font-size: 14px;
  backdrop-filter: blur(8px);
  box-shadow: 0 4px 16px rgba(0,0,0,0.4);
}

.live-toast .toast-close {
  cursor: pointer;
  font-size: 18px;
  line-height: 1;
  opacity: 0.6;
  transition: opacity 0.2s;
  pointer-events: auto;
}
.live-toast .toast-close:hover { opacity: 1; }

.toast-fade-enter-active,
.toast-fade-leave-active {
  transition: opacity 0.3s ease;
}
.toast-fade-enter-from,
.toast-fade-leave-to {
  opacity: 0;
}
</style>
