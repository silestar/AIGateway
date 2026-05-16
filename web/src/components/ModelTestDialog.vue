<template>
  <n-modal v-model:show="visible" preset="card" style="width: 720px" :mask-closable="false">
    <template #header>
      <div style="display: flex; align-items: center; justify-content: space-between; width: 100%; padding-right: 32px">
        <div>
          <span style="font-weight: 600">{{ t('channels.modelTest') }}</span>
          <n-text depth="3" style="font-size: 12px; margin-left: 8px">{{ t('channels.modelTestSubtitle') }} · {{ channelName }}</n-text>
        </div>
      </div>
    </template>

    <n-space vertical size="small">
      <!-- 端点类型 + 流式模式 -->
      <div style="display: flex; gap: 16px; align-items: flex-start">
        <div style="flex: 1; min-width: 0">
          <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px">
            <n-text style="font-size: 13px; white-space: nowrap">{{ t('channels.endpointType') }}</n-text>
            <n-tooltip trigger="hover" placement="top">
              <template #trigger>
                <n-select v-model:value="selectedEndpoint" :options="endpointOptions" size="small" style="min-width: 140px; max-width: 220px" />
              </template>
              {{ selectedEndpointLabel }}
            </n-tooltip>
          </div>
          <n-text depth="3" style="font-size: 11px">{{ t('channels.endpointTypeTip') }}</n-text>
        </div>
        <div style="flex: 1; min-width: 0">
          <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px">
            <n-text style="font-size: 13px; white-space: nowrap">{{ t('channels.streamMode') }}</n-text>
            <n-switch v-model:value="streamEnabled" size="small" />
          </div>
          <n-text depth="3" style="font-size: 11px">{{ t('channels.streamModeTip') }}</n-text>
        </div>
      </div>

      <!-- 渠道模型 + 筛选 -->
      <div style="display: flex; align-items: center; justify-content: space-between; margin-top: 4px">
        <n-text style="font-size: 13px; font-weight: 500">{{ t('channels.channelModels') }}</n-text>
        <n-input v-model:value="modelFilter" :placeholder="t('channels.filterModels')" clearable size="small" style="width: 200px">
          <template #prefix><span style="opacity: 0.5">🔍</span></template>
        </n-input>
      </div>

      <!-- 模型表格 -->
      <div v-if="filteredModels.length > 0" style="border: 1px solid var(--n-border-color, rgba(255,255,255,0.1)); border-radius: 6px; overflow: hidden; max-height: 360px; overflow-y: auto">
        <table style="width: 100%; border-collapse: collapse; font-size: 13px">
          <thead>
            <tr style="background: rgba(255,255,255,0.03); border-bottom: 1px solid var(--n-border-color, rgba(255,255,255,0.06))">
              <th style="padding: 8px 12px; text-align: left; width: 36px">
                <n-checkbox :checked="isAllSelected" :indeterminate="isPartialSelected" @update:checked="toggleAll" />
              </th>
              <th style="padding: 8px 6px; text-align: left">{{ t('channels.modelCol') }}</th>
              <th style="padding: 8px 6px; text-align: left; width: 140px">{{ t('channels.statusCol') }}</th>
              <th style="padding: 8px 6px; text-align: center; width: 70px">{{ t('channels.actionCol') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="m in filteredModels"
              :key="m.name"
              style="border-bottom: 1px solid rgba(255,255,255,0.04); transition: background-color 0.15s"
              :style="{ background: selectedModels.has(m.name) ? 'rgba(0, 210, 255, 0.04)' : '' }"
            >
              <td style="padding: 6px 12px">
                <n-checkbox :checked="selectedModels.has(m.name)" @update:checked="(v: boolean) => toggleModel(m.name, v)" />
              </td>
              <td style="padding: 6px">
                <n-tooltip trigger="hover" placement="top">
                  <template #trigger>
                    <span style="font-family: 'Menlo', 'Consolas', monospace; font-size: 12px; cursor: default">{{ m.name }}</span>
                  </template>
                  {{ m.name }}
                </n-tooltip>
              </td>
              <td style="padding: 6px">
                <div style="display: flex; align-items: center; gap: 6px">
                  <span :style="{ color: statusColor(m.name), fontSize: '10px' }">●</span>
                  <span :style="{ color: statusColor(m.name), fontSize: '12px' }">{{ statusText(m.name) }}</span>
                </div>
              </td>
              <td style="padding: 6px; text-align: center">
                <n-button size="tiny" :loading="testingModel === m.name" :disabled="testingModel !== '' && testingModel !== m.name" @click="handleSingleTest(m.name)">
                  {{ t('channels.testBtn') }}
                </n-button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <n-empty v-else :description="t('channels.noModelsConfigured')" style="padding: 30px 0" />

      <!-- 底部操作栏 -->
      <div v-if="selectedModels.size > 0" style="display: flex; align-items: center; justify-content: center; gap: 12px; padding: 8px 16px; border: 1px solid var(--n-border-color, rgba(255,255,255,0.1)); border-radius: 6px; background: rgba(0, 210, 255, 0.03)">
        <n-tooltip trigger="hover">
          <template #trigger>
            <n-button circle size="small" @click="clearSelection">✖️</n-button>
          </template>
          {{ t('channels.clearSelection') }}
        </n-tooltip>
        <n-divider vertical />
        <n-tag size="small" type="info">{{ selectedModels.size }}</n-tag>
        <n-text depth="3" style="font-size: 12px">{{ t('channels.modelsSelected') }}</n-text>
        <n-divider vertical />
        <n-button type="primary" size="small" :loading="batchTesting" @click="handleBatchTest">{{ t('channels.batchTestBtn') }}</n-button>
      </div>
    </n-space>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NTooltip } from 'naive-ui'
import { channelApi, type TestEndpointInfo, type ChannelModel } from '../api/channel'

const props = defineProps<{
  show: boolean
  channelId: number
  channelName: string
  channelType: string
  models: ChannelModel[]
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'tested'): void
}>()

const { t } = useI18n()
const message = useMessage()

const visible = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v),
})

// 端点
const selectedEndpoint = ref('auto')
const selectedEndpointLabel = computed(() => {
  const opt = endpointOptions.value.find(o => o.value === selectedEndpoint.value)
  return opt?.label || selectedEndpoint.value
})
const streamEnabled = ref(false)
const endpointOptions = ref<{ label: string; value: string }[]>([])

// 模型筛选
const modelFilter = ref('')

// 选中模型
const selectedModels = ref<Set<string>>(new Set())

// 测试状态
interface ModelStatus {
  status: 'untested' | 'testing' | 'success' | 'failure'
  latency?: number
  error?: string
}
const modelStatusMap = ref<Map<string, ModelStatus>>(new Map())
const testingModel = ref('')
const batchTesting = ref(false)

// 模型列表（上游优先，自定义排后）
interface ModelItem {
  name: string
  isUpstream: boolean
}
const sortedModels = computed<ModelItem[]>(() => {
  const items: ModelItem[] = []
  const seen = new Set<string>()
  for (const m of props.models) {
    if (m.status !== 'enabled') continue
    if (!seen.has(m.display_model_name)) {
      seen.add(m.display_model_name)
      items.push({
        name: m.display_model_name,
        isUpstream: m.display_model_name === m.actual_model_name,
      })
    }
  }
  // 上游排前面，自定义排后面
  items.sort((a, b) => {
    if (a.isUpstream !== b.isUpstream) return a.isUpstream ? -1 : 1
    return a.name.localeCompare(b.name)
  })
  return items
})

const filteredModels = computed(() => {
  if (!modelFilter.value) return sortedModels.value
  const q = modelFilter.value.toLowerCase()
  return sortedModels.value.filter(m => m.name.toLowerCase().includes(q))
})

// 全选逻辑
const isAllSelected = computed(() => filteredModels.value.length > 0 && filteredModels.value.every(m => selectedModels.value.has(m.name)))
const isPartialSelected = computed(() => !isAllSelected.value && filteredModels.value.some(m => selectedModels.value.has(m.name)))

function toggleAll(checked: boolean) {
  if (checked) {
    for (const m of filteredModels.value) {
      selectedModels.value.add(m.name)
    }
  } else {
    for (const m of filteredModels.value) {
      selectedModels.value.delete(m.name)
    }
  }
  selectedModels.value = new Set(selectedModels.value)
}

function toggleModel(name: string, checked: boolean) {
  if (checked) {
    selectedModels.value.add(name)
  } else {
    selectedModels.value.delete(name)
  }
  selectedModels.value = new Set(selectedModels.value)
}

function clearSelection() {
  selectedModels.value = new Set()
}

// 状态显示
function statusColor(name: string): string {
  const s = modelStatusMap.value.get(name)
  if (!s) return 'var(--n-text-color-3)'
  switch (s.status) {
    case 'testing': return '#2080f0'
    case 'success': return '#18a058'
    case 'failure': return '#d03050'
    default: return 'var(--n-text-color-3)'
  }
}

function statusText(name: string): string {
  const s = modelStatusMap.value.get(name)
  if (!s) return t('channels.statusUntested')
  switch (s.status) {
    case 'testing': return t('channels.statusTesting')
    case 'success': return s.latency !== undefined ? `${t('channels.statusSuccess')} (${s.latency}ms)` : t('channels.statusSuccess')
    case 'failure': return t('channels.statusFailed')
    default: return t('channels.statusUntested')
  }
}

// 单模型测试
async function handleSingleTest(modelName: string) {
  testingModel.value = modelName
  modelStatusMap.value.set(modelName, { status: 'testing' })
  try {
    const res = await channelApi.testSingleModel(props.channelId, {
      model: modelName,
      endpoint: selectedEndpoint.value,
      stream: streamEnabled.value,
    })
    const data = res.data?.data
    if (data?.success) {
      modelStatusMap.value.set(modelName, { status: 'success', latency: data.latency })
    } else {
      modelStatusMap.value.set(modelName, { status: 'failure', error: data?.error || 'Unknown error' })
    }
  } catch (e: any) {
    modelStatusMap.value.set(modelName, { status: 'failure', error: e?.response?.data?.error || e.message })
  } finally {
    testingModel.value = ''
    emit('tested')
  }
}

// 批量测试
async function handleBatchTest() {
  const models = Array.from(selectedModels.value)
  if (models.length === 0) return

  batchTesting.value = true
  // 标记为 testing
  for (const m of models) {
    modelStatusMap.value.set(m, { status: 'testing' })
  }

  try {
    const res = await channelApi.batchTestModels(props.channelId, models, selectedEndpoint.value, streamEnabled.value)
    const results = res.data?.data || []
    for (const r of results) {
      if (r.success) {
        modelStatusMap.value.set(r.model, { status: 'success', latency: r.latency })
      } else {
        modelStatusMap.value.set(r.model, { status: 'failure', error: r.error })
      }
    }
  } catch (e: any) {
    message.error(t('channels.batchTestFailed'))
    for (const m of models) {
      if (modelStatusMap.value.get(m)?.status === 'testing') {
        modelStatusMap.value.set(m, { status: 'failure', error: e.message })
      }
    }
  } finally {
    batchTesting.value = false
    emit('tested')
  }
}

// 加载端点列表
async function loadEndpoints() {
  try {
    const res = await channelApi.getTestEndpoints(props.channelId)
    const data = res.data?.data || []
    endpointOptions.value = data.map((e: TestEndpointInfo) => ({ label: e.label, value: e.id }))
    if (data.length > 0) {
      selectedEndpoint.value = 'auto'
    }
  } catch {
    endpointOptions.value = [{ label: t('channels.autoDetect'), value: 'auto' }]
  }
}

// 弹窗打开时初始化
watch(() => props.show, (val) => {
  if (val) {
    selectedModels.value = new Set()
    modelStatusMap.value = new Map()
    modelFilter.value = ''
    testingModel.value = ''
    selectedEndpoint.value = 'auto'
    streamEnabled.value = false
    loadEndpoints()
  }
})
</script>
