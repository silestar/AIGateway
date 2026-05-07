<template>
  <div class="system-logs-page">
    <!-- 顶部筛选区 -->
    <div class="filter-bar glass-card">
      <n-space :size="12" align="center" wrap>
        <!-- 日期选择 -->
        <n-select
          v-model:value="selectedDate"
          :options="dateOptions"
          :placeholder="t('systemLogs.selectDate')"
          style="width: 180px"
          @update:value="onDateChange"
        />
        <!-- 级别多选 -->
        <n-select
          v-model:value="selectedLevels"
          :options="levelOptions"
          multiple
          :placeholder="t('systemLogs.selectLevel')"
          style="width: 200px"
          @update:value="onFilterChange"
        />
        <!-- 关键字搜索 -->
        <n-input
          v-model:value="keyword"
          :placeholder="t('systemLogs.keywordPlaceholder')"
          clearable
          style="width: 200px"
          @keyup.enter="onFilterChange"
          @clear="onFilterChange"
        />
        <!-- trace_id 搜索 -->
        <n-input
          v-model:value="traceId"
          :placeholder="t('systemLogs.traceIdPlaceholder')"
          clearable
          style="width: 200px"
          @keyup.enter="onFilterChange"
          @clear="onFilterChange"
        />
        <!-- 搜索按钮 -->
        <n-button type="primary" @click="onFilterChange">
          {{ t('common.search') }}
        </n-button>
        <!-- 重置按钮 -->
        <n-button @click="resetFilters">
          {{ t('systemLogs.reset') }}
        </n-button>
      </n-space>
    </div>

    <!-- 工具栏 -->
    <div class="toolbar glass-card">
      <n-space :size="12" align="center">
        <!-- 分页大小 -->
        <n-select
          v-model:value="pageSize"
          :options="pageSizeOptions"
          style="width: 100px"
          @update:value="onPageSizeChange"
        />
        <!-- 实时跟踪开关 -->
        <n-switch v-model:value="liveMode" @update:value="onLiveModeChange">
          <template #checked>{{ t('systemLogs.liveOn') }}</template>
          <template #unchecked>{{ t('systemLogs.liveOff') }}</template>
        </n-switch>
        <!-- 列选择器 -->
        <n-popover trigger="click" placement="bottom">
          <template #trigger>
            <n-button size="small">
              {{ t('systemLogs.columns') }} 📋
            </n-button>
          </template>
          <n-checkbox-group v-model:value="visibleColumns">
            <n-space vertical>
              <n-checkbox v-for="col in allColumns" :key="col.key" :value="col.key" :label="col.label" />
            </n-space>
          </n-checkbox-group>
        </n-popover>
        <!-- 导出 -->
        <n-button size="small" @click="exportLogs" :loading="exporting">
          {{ t('systemLogs.export') }} ⬇
        </n-button>
      </n-space>
      <span class="log-count">{{ t('systemLogs.total') }}: {{ total }}</span>
    </div>

    <!-- 日志表格 -->
    <div class="log-table glass-card">
      <n-data-table
        :columns="tableColumns"
        :data="logEntries"
        :row-key="(row: SystemLogEntry) => (row.ts || '') + (row.msg || '')"
        :row-props="rowProps"
        :loading="loading"
        :bordered="false"
        size="small"
        flex-height
        style="height: calc(100vh - 340px)"
      />
    </div>

    <!-- 分页器 -->
    <div class="pagination-bar">
      <n-pagination
        v-model:page="currentPage"
        :page-count="totalPages"
        :page-size="pageSize"
        show-quick-jumper
        @update:page="onPageChange"
      />
    </div>

    <!-- 详情抽屉 -->
    <n-drawer v-model:show="showDetail" :width="640" placement="right">
      <n-drawer-content :title="t('systemLogs.detail')" closable>
        <template v-if="selectedLog">
          <!-- 区块1：基本信息 -->
          <div class="detail-section">
            <div class="detail-section-title">{{ t('systemLogs.detailBasicInfo') }}</div>
            <div class="detail-grid">
              <div class="detail-label">{{ t('systemLogs.detailTime') }}</div>
              <div class="detail-value">{{ formatFullTimestamp(selectedLog.ts) }}</div>

              <div class="detail-label">{{ t('systemLogs.detailLevel') }}</div>
              <div class="detail-value">
                <span class="level-badge" :style="levelBadgeStyle(selectedLog.level)">{{ (selectedLog.level || '').toUpperCase() }}</span>
              </div>

              <template v-if="selectedLog.caller">
                <div class="detail-label">{{ t('systemLogs.detailModule') }}</div>
                <div class="detail-value monospace">{{ selectedLog.caller }}</div>
              </template>

              <template v-if="selectedLog.msg">
                <div class="detail-label">{{ t('systemLogs.detailMessage') }}</div>
                <div class="detail-value">{{ selectedLog.msg }}</div>
              </template>

              <template v-if="selectedLog.trace_id">
                <div class="detail-label">{{ t('systemLogs.detailTraceId') }}</div>
                <div class="detail-value monospace clickable" @click="copyTraceId(selectedLog.trace_id!)">
                  {{ selectedLog.trace_id }}
                  <span class="copy-hint">📋</span>
                </div>
              </template>

              <template v-if="selectedLog.status">
                <div class="detail-label">{{ t('systemLogs.detailStatusCode') }}</div>
                <div class="detail-value">
                  <span class="status-badge" :class="is2xx(selectedLog.status) ? 'status-success' : 'status-error'">{{ selectedLog.status }}</span>
                </div>
              </template>
            </div>
          </div>

          <n-divider style="margin: 12px 0" />

          <!-- 区块2：请求详情（仅 HTTP 请求日志） -->
          <template v-if="hasRequestInfo">
            <div class="detail-section">
              <div class="detail-section-title">{{ t('systemLogs.detailRequest') }}</div>
              <div class="detail-grid">
                <template v-if="selectedLog.method">
                  <div class="detail-label">{{ t('systemLogs.detailMethod') }}</div>
                  <div class="detail-value">
                    <span class="method-badge" :class="'method-' + String(selectedLog.method).toLowerCase()">{{ selectedLog.method }}</span>
                  </div>
                </template>
                <template v-if="selectedLog.path">
                  <div class="detail-label">{{ t('systemLogs.detailPath') }}</div>
                  <div class="detail-value monospace">{{ selectedLog.path }}</div>
                </template>
                <template v-if="selectedLog.query">
                  <div class="detail-label">{{ t('systemLogs.detailQuery') }}</div>
                  <div class="detail-value monospace">{{ selectedLog.query }}</div>
                </template>
                <template v-if="selectedLog.ip">
                  <div class="detail-label">{{ t('systemLogs.detailIp') }}</div>
                  <div class="detail-value monospace">{{ selectedLog.ip }}</div>
                </template>
                <template v-if="selectedLog.latency">
                  <div class="detail-label">{{ t('systemLogs.detailLatency') }}</div>
                  <div class="detail-value">{{ formatDuration(selectedLog.latency) }}</div>
                </template>
              </div>
            </div>
            <n-divider style="margin: 12px 0" />
          </template>

          <!-- 区块3：额外信息 -->
          <template v-if="extraFields.length > 0">
            <div class="detail-section">
              <div class="detail-section-title">{{ t('systemLogs.detailExtra') }}</div>
              <div class="detail-grid">
                <template v-for="field in extraFields" :key="field.key">
                  <div class="detail-label monospace">{{ field.key }}</div>
                  <div class="detail-value">
                    <template v-if="typeof field.value === 'boolean'">
                      <span class="bool-badge" :class="field.value ? 'bool-true' : 'bool-false'">{{ field.value }}</span>
                    </template>
                    <template v-else-if="isTimeField(field.key)">
                      {{ formatDuration(field.value) }}
                      <span class="raw-hint">({{ field.value }})</span>
                    </template>
                    <template v-else-if="typeof field.value === 'object' && field.value !== null">
                      <pre class="inline-json">{{ JSON.stringify(field.value, null, 2) }}</pre>
                    </template>
                    <template v-else>
                      {{ field.value }}
                    </template>
                  </div>
                </template>
              </div>
            </div>
            <n-divider style="margin: 12px 0" />
          </template>

          <!-- 区块4：原始数据 -->
          <n-collapse>
            <n-collapse-item :title="t('systemLogs.detailRaw')" name="raw">
              <pre class="raw-json">{{ detailJson }}</pre>
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
  NSpace,
  NSwitch,
  NPopover,
  NCheckboxGroup,
  NCheckbox,
  NDataTable,
  NPagination,
  NDrawer,
  NDrawerContent,
  NDivider,
  NCollapse,
  NCollapseItem,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { systemLogApi, type SystemLogEntry } from '../api/system'

const { t } = useI18n()
const message = useMessage()

// === 筛选状态 ===
const selectedDate = ref('')
const selectedLevels = ref<string[]>([])
const keyword = ref('')
const traceId = ref('')
const currentPage = ref(1)
const pageSize = ref(100)
const total = ref(0)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

// === 数据 ===
const logEntries = ref<SystemLogEntry[]>([])
const dateOptions = ref<{ label: string; value: string }[]>([])
const loading = ref(false)
const exporting = ref(false)

// === 实时跟踪 ===
const liveMode = ref(false)
let liveTimer: ReturnType<typeof setInterval> | null = null

// === 详情 ===
const showDetail = ref(false)
const selectedLog = ref<SystemLogEntry | null>(null)
const detailJson = computed(() => selectedLog.value ? JSON.stringify(selectedLog.value, null, 2) : '')

// === 列选择 ===
const allColumns = computed(() => [
  { key: 'time', label: t('systemLogs.colTime') },
  { key: 'level', label: t('systemLogs.colLevel') },
  { key: 'caller', label: t('systemLogs.colModule') },
  { key: 'msg', label: t('systemLogs.colMessage') },
  { key: 'trace_id', label: 'Trace ID' },
])
const visibleColumns = ref<string[]>(['time', 'level', 'caller', 'msg'])

// === 级别选项 ===
const levelOptions = [
  { label: 'DEBUG', value: 'debug' },
  { label: 'INFO', value: 'info' },
  { label: 'WARN', value: 'warn' },
  { label: 'ERROR', value: 'error' },
]

// === 分页大小选项 ===
const pageSizeOptions = [
  { label: '50', value: 50 },
  { label: '100', value: 100 },
  { label: '200', value: 200 },
  { label: '500', value: 500 },
]

// === 级别颜色 ===
const levelColorMap: Record<string, string> = {
  debug: '#9e9e9e',
  info: '',
  warn: '#f0c040',
  error: '#ff4d4f',
}

// === 已知字段集合（排除这些字段后就是额外字段） ===
const knownFields = new Set(['ts', 'level', 'msg', 'caller', 'trace_id', 'status', 'method', 'path', 'query', 'ip', 'latency'])

// === 时间相关字段名 ===
const timeFieldNames = new Set(['latency', 'duration', 'elapsed', 'timeout', 'response_time', 'processing_time'])

// === Computed: 是否包含请求详情 ===
const hasRequestInfo = computed(() => {
  if (!selectedLog.value) return false
  return !!(selectedLog.value.method || selectedLog.value.path || selectedLog.value.query || selectedLog.value.ip || selectedLog.value.latency)
})

// === Computed: 额外字段 ===
const extraFields = computed(() => {
  if (!selectedLog.value) return []
  return Object.entries(selectedLog.value)
    .filter(([key]) => !knownFields.has(key))
    .map(([key, value]) => ({ key, value }))
})

// === 格式化：列表时间 HH:mm:ss.ms ===
function formatTime(ts?: string): string {
  if (!ts) return ''
  try {
    const d = new Date(ts)
    const h = String(d.getHours()).padStart(2, '0')
    const m = String(d.getMinutes()).padStart(2, '0')
    const s = String(d.getSeconds()).padStart(2, '0')
    const ms = String(d.getMilliseconds()).padStart(3, '0')
    return `${h}:${m}:${s}.${ms}`
  } catch {
    return ts
  }
}

// === 格式化：完整时间戳 YYYY-MM-DD HH:mm:ss.ms ===
function formatFullTimestamp(ts?: string): string {
  if (!ts) return ''
  try {
    const d = new Date(ts)
    const Y = d.getFullYear()
    const M = String(d.getMonth() + 1).padStart(2, '0')
    const D = String(d.getDate()).padStart(2, '0')
    const h = String(d.getHours()).padStart(2, '0')
    const m = String(d.getMinutes()).padStart(2, '0')
    const s = String(d.getSeconds()).padStart(2, '0')
    const ms = String(d.getMilliseconds()).padStart(3, '0')
    return `${Y}-${M}-${D} ${h}:${m}:${s}.${ms}`
  } catch {
    return ts
  }
}

// === 格式化：持续时间 ===
function formatDuration(value: unknown): string {
  if (value == null) return ''

  // 数字 → 当作毫秒
  if (typeof value === 'number') {
    return formatMilliseconds(value)
  }

  if (typeof value !== 'string') return String(value)

  const str = value.trim()

  // 纯数字
  const num = parseFloat(str)
  if (!isNaN(num) && str === String(num)) {
    return formatMilliseconds(num)
  }

  // "X.XXXs" 格式
  const secMatch = str.match(/^([\d.]+)\s*s$/i)
  if (secMatch) {
    return formatMilliseconds(parseFloat(secMatch[1]) * 1000)
  }

  // "Xms" 格式
  const msMatch = str.match(/^([\d.]+)\s*ms$/i)
  if (msMatch) {
    return formatMilliseconds(parseFloat(msMatch[1]))
  }

  // "XmYs" 格式
  const minMatch = str.match(/^(\d+)m(\d+)s$/)
  if (minMatch) {
    return formatMilliseconds((parseInt(minMatch[1]) * 60 + parseInt(minMatch[2])) * 1000)
  }

  // 无法解析，原样返回
  return str
}

function formatMilliseconds(ms: number): string {
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

// === 判断字段是否为时间相关 ===
function isTimeField(key: string): boolean {
  return timeFieldNames.has(key)
}

// === 判断状态码是否 2xx ===
function is2xx(status: unknown): boolean {
  const code = Number(status)
  return code >= 200 && code < 300
}

// === 级别 badge 样式 ===
function levelBadgeStyle(level?: string): Record<string, string> {
  const lvl = (level || '').toLowerCase()
  const color = levelColorMap[lvl]
  const style: Record<string, string> = {
    padding: '2px 8px',
    borderRadius: '4px',
    fontSize: '12px',
    fontWeight: '600',
    letterSpacing: '0.5px',
  }
  if (color) {
    style.color = color
    style.border = `1px solid ${color}40`
    style.background = `${color}15`
  } else {
    style.color = 'var(--n-text-color)'
    style.border = '1px solid var(--n-border-color)'
    style.background = 'var(--n-color-embedded)'
  }
  return style
}

// === 截取 caller ===
function shortCaller(caller?: string): string {
  if (!caller) return ''
  const parts = caller.split('/')
  return parts.length > 1 ? parts[parts.length - 1] : caller
}

// === 高亮关键字 ===
function highlightText(text: string, kw: string): string {
  if (!kw) return text
  const regex = new RegExp(`(${kw.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi')
  return text.replace(regex, '<mark style="background:#ffeb3b;color:#333;padding:0 2px;border-radius:2px">$1</mark>')
}

// === 表格列定义 ===
const tableColumns = computed<DataTableColumns<SystemLogEntry>>(() => {
  const cols: DataTableColumns<SystemLogEntry> = []
  if (visibleColumns.value.includes('time')) {
    cols.push({
      title: t('systemLogs.colTime'),
      key: 'time',
      width: 120,
      render: (row) => formatTime(row.ts),
    })
  }
  if (visibleColumns.value.includes('level')) {
    cols.push({
      title: t('systemLogs.colLevel'),
      key: 'level',
      width: 80,
      render: (row) => {
        const lvl = (row.level || '').toLowerCase()
        const color = levelColorMap[lvl]
        return h('span', {
          style: {
            color: color || undefined,
            fontWeight: '600',
            textTransform: 'uppercase',
          },
        }, lvl || '-')
      },
    })
  }
  if (visibleColumns.value.includes('caller')) {
    cols.push({
      title: t('systemLogs.colModule'),
      key: 'caller',
      width: 200,
      ellipsis: { tooltip: true },
      render: (row) => shortCaller(row.caller),
    })
  }
  if (visibleColumns.value.includes('msg')) {
    cols.push({
      title: t('systemLogs.colMessage'),
      key: 'msg',
      ellipsis: { tooltip: true },
      render: (row) => {
        const msg = row.msg || ''
        if (keyword.value) {
          return h('span', { innerHTML: highlightText(msg, keyword.value) })
        }
        return msg
      },
    })
  }
  if (visibleColumns.value.includes('trace_id')) {
    cols.push({
      title: 'Trace ID',
      key: 'trace_id',
      width: 200,
      render: (row) => {
        const tid = row.trace_id || ''
        if (!tid) return '-'
        return h('span', {
          style: { cursor: 'pointer', color: '#00d2ff', fontFamily: 'monospace', fontSize: '12px' },
          onClick: (e: Event) => {
            e.stopPropagation()
            copyTraceId(tid)
          },
        }, tid)
      },
    })
  }
  return cols
})

// === 复制 trace_id ===
function copyTraceId(tid: string) {
  navigator.clipboard.writeText(tid).then(() => {
    message.success(t('common.copied'))
  }).catch(() => {
    message.error(t('common.copyFailed'))
  })
}

// === 行点击 → 详情 ===
function rowProps(row: SystemLogEntry) {
  return {
    style: 'cursor: pointer',
    onClick: () => {
      selectedLog.value = row
      showDetail.value = true
    },
  }
}

// === 加载日志 ===
async function fetchLogs(since?: string) {
  if (!selectedDate.value) return
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      date: selectedDate.value,
      page: currentPage.value,
      page_size: pageSize.value,
    }
    if (selectedLevels.value.length > 0) params.level = selectedLevels.value.join(',')
    if (keyword.value) params.keyword = keyword.value
    if (traceId.value) params.trace_id = traceId.value
    if (since) params.since = since

    const res = await systemLogApi.list(params as any)
    const data = (res.data as { data: SystemLogEntry[]; total: number; page: number; page_size: number })

    if (since && liveMode.value) {
      // 实时跟踪：新日志插入顶部
      const newLogs = data.data || []
      if (newLogs.length > 0) {
        // 去重
        const existSet = new Set(logEntries.value.map(e => (e.ts || '') + (e.msg || '')))
        const unique = newLogs.filter(e => !existSet.has((e.ts || '') + (e.msg || '')))
        if (unique.length > 0) {
          logEntries.value = [...unique, ...logEntries.value]
          total.value += unique.length
        }
      }
    } else {
      logEntries.value = data.data || []
      total.value = data.total || 0
    }
  } catch (e: any) {
    message.error(e?.message || 'Failed to fetch logs')
  } finally {
    loading.value = false
  }
}

// === 加载日期列表 ===
async function fetchDates() {
  try {
    const res = await systemLogApi.dates()
    const data = (res.data as { data: string[] }).data || []
    dateOptions.value = data.map(d => ({ label: d, value: d }))
    // 默认选中今天
    if (!selectedDate.value && data.length > 0) {
      selectedDate.value = data[0]
      fetchLogs()
    }
  } catch {
    // ignore
  }
}

// === 筛选变更 ===
function onFilterChange() {
  // 修改筛选条件时关闭实时跟踪
  if (liveMode.value) {
    liveMode.value = false
    stopLiveMode()
  }
  currentPage.value = 1
  fetchLogs()
}

function onDateChange() {
  currentPage.value = 1
  if (liveMode.value) {
    liveMode.value = false
    stopLiveMode()
  }
  fetchLogs()
}

function resetFilters() {
  selectedLevels.value = []
  keyword.value = ''
  traceId.value = ''
  currentPage.value = 1
  if (liveMode.value) {
    liveMode.value = false
    stopLiveMode()
  }
  fetchLogs()
}

// === 分页 ===
function onPageChange(page: number) {
  currentPage.value = page
  fetchLogs()
}

function onPageSizeChange(size: number) {
  pageSize.value = size
  currentPage.value = 1
  fetchLogs()
}

// === 实时跟踪 ===
function onLiveModeChange(val: boolean) {
  if (val) {
    startLiveMode()
  } else {
    stopLiveMode()
  }
}

function startLiveMode() {
  if (liveTimer) clearInterval(liveTimer)
  liveTimer = setInterval(() => {
    if (logEntries.value.length > 0 && logEntries.value[0]?.ts) {
      fetchLogs(logEntries.value[0].ts)
    } else {
      fetchLogs()
    }
  }, 2000)
}

function stopLiveMode() {
  if (liveTimer) {
    clearInterval(liveTimer)
    liveTimer = null
  }
}

// === 导出 ===
async function exportLogs() {
  if (!selectedDate.value) {
    message.warning(t('systemLogs.selectDate'))
    return
  }
  exporting.value = true
  try {
    const res = await systemLogApi.download(selectedDate.value)
    const blob = new Blob([res.data as any], { type: 'application/octet-stream' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${selectedDate.value}.log`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e: any) {
    message.error(e?.message || 'Export failed')
  } finally {
    exporting.value = false
  }
}

// === 生命周期 ===
onMounted(() => {
  fetchDates()
})

onUnmounted(() => {
  stopLiveMode()
})
</script>

<style scoped>
.system-logs-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
}

.filter-bar {
  padding: 16px;
}

.toolbar {
  padding: 10px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.log-count {
  font-size: 13px;
  color: var(--text-secondary);
}

.log-table {
  flex: 1;
  min-height: 0;
}

.log-table :deep(.n-data-table) {
  --n-td-color: transparent;
  --n-th-color: rgba(16, 22, 42, 0.5);
}

.log-table :deep(.n-data-table-td) {
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace;
  font-size: 12px;
  padding: 4px 8px !important;
}

.log-table :deep(.n-data-table-th) {
  font-size: 12px;
  padding: 6px 8px !important;
}

.pagination-bar {
  display: flex;
  justify-content: center;
  padding: 12px 0;
}

/* === 详情抽屉样式 === */
.detail-section {
  margin-bottom: 4px;
}

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

.detail-value.monospace {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
}

.detail-value.clickable {
  cursor: pointer;
  color: #00d2ff;
  transition: opacity 0.2s;
}

.detail-value.clickable:hover {
  opacity: 0.8;
}

.copy-hint {
  margin-left: 4px;
  opacity: 0.5;
  font-size: 11px;
}

.raw-hint {
  margin-left: 6px;
  font-size: 11px;
  color: var(--n-text-color-3, #999);
}

/* 级别 badge */
.level-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

/* 状态码 badge */
.status-badge {
  display: inline-block;
  padding: 2px 8px;
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

/* HTTP 方法 badge */
.method-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.5px;
}

.method-get { color: #61affe; background: rgba(97, 175, 254, 0.1); border: 1px solid rgba(97, 175, 254, 0.3); }
.method-post { color: #49cc90; background: rgba(73, 204, 144, 0.1); border: 1px solid rgba(73, 204, 144, 0.3); }
.method-put { color: #fca130; background: rgba(252, 161, 48, 0.1); border: 1px solid rgba(252, 161, 48, 0.3); }
.method-delete { color: #f93e3e; background: rgba(249, 62, 62, 0.1); border: 1px solid rgba(249, 62, 62, 0.3); }
.method-patch { color: #50e3c2; background: rgba(80, 227, 194, 0.1); border: 1px solid rgba(80, 227, 194, 0.3); }

/* 布尔值 badge */
.bool-badge {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: 600;
  font-family: 'JetBrains Mono', monospace;
}

.bool-true {
  color: #52c41a;
  background: rgba(82, 196, 26, 0.1);
  border: 1px solid rgba(82, 196, 26, 0.3);
}

.bool-false {
  color: #999;
  background: rgba(153, 153, 153, 0.1);
  border: 1px solid rgba(153, 153, 153, 0.3);
}

/* 内联 JSON */
.inline-json {
  background: rgba(0, 0, 0, 0.3);
  color: #e8eaed;
  padding: 8px;
  border-radius: 6px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 11px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
  max-height: 200px;
  overflow: auto;
}

/* 原始 JSON 折叠面板 */
.raw-json {
  background: rgba(0, 0, 0, 0.3);
  color: #e8eaed;
  padding: 16px;
  border-radius: 8px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  overflow: auto;
  max-height: calc(100vh - 400px);
  margin: 0;
}
</style>
