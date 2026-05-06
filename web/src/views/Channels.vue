<template>
  <div>
    <!-- 列表视图 -->
    <n-card v-if="!selectedChannel" :bordered="false" class="glass-card">
      <template #header>
        <h2 class="page-title" style="margin:0">{{ t('channels.title') }}</h2>
      </template>
      <template #header-extra>
        <n-button type="primary" @click="showCreateModal = true">+ {{ t('common.create') }}</n-button>
      </template>

      <n-space vertical size="large">
        <n-space>
          <n-select v-model:value="filterType" :options="channelTypeOptions" :placeholder="t('channels.type')" clearable style="width: 140px" />
          <n-button @click="loadChannels">{{ t('common.search') }}</n-button>
        </n-space>
        <n-data-table :columns="columns" :data="channels" :loading="loading" :pagination="pagination" remote @update:page="handlePageChange" />
      </n-space>
    </n-card>

    <!-- 详情视图 -->
    <n-card v-else :title="selectedChannel.name">

      <n-tabs type="line" animated>
        <!-- 基本信息 -->
        <n-tab-pane name="info" :tab="t('channels.basicInfo')">
          <n-form :model="editForm" label-placement="left" label-width="100">
            <n-form-item :label="t('common.name')"><n-input v-model:value="editForm.name" /></n-form-item>
            <n-form-item :label="t('channels.baseUrl')">
              <n-input v-model:value="editForm.base_url" />
              <template #feedback>
                <n-text v-if="editForm.base_url.match(/\/v\d+\/?$/)" type="warning" style="font-size: 12px">{{ t('channels.baseUrlSuffixTip') }}</n-text>
              </template>
            </n-form-item>
            <n-form-item :label="t('common.type')"><n-input :value="selectedChannel.type" disabled /></n-form-item>
            <n-form-item :label="t('common.weight')"><n-input-number v-model:value="editForm.weight" :min="0" /></n-form-item>
            <n-form-item :label="t('channels.maxRPM')">
              <n-input-number v-model:value="editForm.max_rpm" :min="0" :placeholder="t('channels.noLimit')" />
            </n-form-item>
            <n-form-item :label="t('channels.maxTPM')">
              <n-input-number v-model:value="editForm.max_tpm" :min="0" :placeholder="t('channels.noLimit')" />
            </n-form-item>
            <n-form-item :label="t('channels.maxDailyRequests')">
              <n-input-number v-model:value="editForm.max_daily_requests" :min="0" :placeholder="t('channels.noLimit')" />
            </n-form-item>
            <n-form-item>
              <n-space>
                <n-button type="primary" @click="handleUpdateChannel">{{ t('common.save') }}</n-button>
                <n-button @click="selectedChannel = null">{{ t('common.back') }}</n-button>
              </n-space>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 模型配置 -->
        <n-tab-pane name="models" :tab="t('channels.models')">
          <n-space vertical>
            <n-button type="primary" @click="showModelModal = true">{{ t('channels.fetchModels') }}</n-button>
            <!-- 已选上游模型标签区 -->
            <div v-if="upstreamModels.length > 0" class="model-tag-area">
              <n-text depth="3" style="font-size: 13px; margin-bottom: 8px; display: block">{{ t('channels.upstreamModels') }}（{{ upstreamModels.length }}）</n-text>
              <n-space size="small">
                <n-tag
                  v-for="name in upstreamModels"
                  :key="name"
                  size="small"
                  @click="copyModelName(name)"
                  style="cursor: pointer; font-family: 'Menlo', 'Consolas', monospace"
                  :title="t('channels.clickToCopyModel')"
                >
                  {{ name }}
                </n-tag>
              </n-space>
            </div>
            <n-empty v-else :description="t('channels.noModelsConfigured')" style="padding: 20px 0" />
            <!-- 映射列表 -->
            <div v-if="modelMappings.length > 0" style="margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--n-border-color, rgba(255,255,255,0.1))">
              <n-text depth="3" style="font-size: 13px; margin-bottom: 8px; display: block">{{ t('channels.modelMapping') }}（{{ modelMappings.length }}）</n-text>
              <div v-for="m in modelMappings" :key="m.display_model_name" style="display: flex; align-items: center; gap: 8px; margin-bottom: 6px; padding: 4px 8px; background: rgba(255,255,255,0.03); border-radius: 4px">
                <span style="font-family: 'Menlo', 'Consolas', monospace; font-size: 13px; color: #00d2ff">{{ m.display_model_name }}</span>
                <span style="color: var(--text-tertiary)">→</span>
                <span style="font-family: 'Menlo', 'Consolas', monospace; font-size: 13px">{{ m.actual_model_name }}</span>
              </div>
            </div>
          </n-space>
        </n-tab-pane>

        <!-- 账号管理 -->
        <n-tab-pane name="accounts" :tab="t('channels.accounts')">
          <n-space vertical>
            <n-button type="primary" @click="showAddAccount = true">{{ t('channels.addAccount') }}</n-button>
            <n-data-table :columns="accountColumns" :data="accounts" />
          </n-space>
        </n-tab-pane>
      </n-tabs>
    </n-card>

    <!-- 创建渠道弹窗 -->
    <n-modal v-model:show="showCreateModal" preset="dialog" :title="t('channels.create')" :positive-text="t('common.confirm')" :negative-text="t('common.cancel')" @positive-click="handleCreateChannel">
      <n-form :model="createForm">
        <n-form-item :label="t('common.name')"><n-input v-model:value="createForm.name" /></n-form-item>
        <n-form-item :label="t('common.type')">
          <n-select v-model:value="createForm.type" :options="channelTypeOptions" />
        </n-form-item>
        <n-form-item :label="t('channels.baseUrl')">
          <n-input v-model:value="createForm.base_url" />
          <template #feedback>
            <n-text v-if="createForm.base_url.match(/\/v\d+\/?$/)" type="warning" style="font-size: 12px">{{ t('channels.baseUrlSuffixTip') }}</n-text>
          </template>
        </n-form-item>
        <n-form-item :label="t('channels.apiKeyRequired')">
          <n-input v-model:value="createForm.api_key" type="password" show-password-on="click" :placeholder="t('channels.apiKeyPlaceholder')" />
        </n-form-item>
        <n-button :loading="testingConnection" @click="handleTestConnection" style="margin-bottom: 12px">{{ t('channels.testConnection') }}</n-button>
        <n-alert v-if="testConnectionResult !== null" :type="testConnectionResult ? 'success' : 'error'" style="margin-bottom: 12px">
          {{ testConnectionResult ? t('channels.testConnectionSuccess') : testConnectionError }}
        </n-alert>
      </n-form>
    </n-modal>

    <!-- 添加账号弹窗 -->
    <n-modal v-model:show="showAddAccount" preset="dialog" :title="t('channels.addAccount')" :positive-text="t('common.confirm')" :negative-text="t('common.cancel')" @positive-click="handleAddAccount">
      <n-form :model="accountForm">
        <n-form-item :label="t('channels.keyLabel')"><n-input v-model:value="accountForm.api_key" type="password" show-password-on="click" /></n-form-item>
        <n-form-item :label="t('channels.remark')"><n-input v-model:value="accountForm.remark" :placeholder="t('channels.remarkPlaceholder')" /></n-form-item>
      </n-form>
    </n-modal>

    <!-- 模型选择弹窗 -->
    <ModelSelectModal
      v-model:show="showModelModal"
      :channel-id="selectedChannel?.id ?? 0"
      :channel-name="selectedChannel?.name ?? ''"
      :existing-models="channelModels"
      @save="handleModelSave"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, h, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, useDialog, NButton, NSpace, NTag, NInput, NAlert, NInputNumber } from 'naive-ui'
import { channelApi, type Channel, type ChannelModel } from '../api/channel'
import { accountApi, type Account } from '../api/account'
import ModelSelectModal from '../components/ModelSelectModal.vue'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const channels = ref<Channel[]>([])
const selectedChannel = ref<Channel | null>(null)
const channelModels = ref<ChannelModel[]>([])

// 已选上游模型：所有 unique 的 actual_model_name（去重）
const upstreamModels = computed(() => {
  const names = new Set<string>()
  channelModels.value
    .filter(m => m.status === 'enabled')
    .forEach(m => names.add(m.actual_model_name))
  return Array.from(names)
})

// 映射关系：display !== actual（有自定义别名）
const modelMappings = computed(() =>
  channelModels.value.filter(m => m.display_model_name !== m.actual_model_name && m.status === 'enabled')
)

const accounts = ref<Account[]>([])
const showCreateModal = ref(false)
const showAddAccount = ref(false)
const showModelModal = ref(false)

// 双击编辑备注相关
const editingRemarkId = ref<number | null>(null)
const editingRemark = ref('')

// 行内编辑权重/优先级的临时值（不直接改响应式数据，避免重渲染导致焦点丢失）
const editingWeightMap = reactive<Record<number, number>>({})
const editingPriorityMap = reactive<Record<number, number>>({})

// 测试连接相关
const testingConnection = ref(false)
const testConnectionResult = ref<boolean | null>(null)
const testConnectionError = ref('')

const pagination = reactive({ page: 1, pageSize: 20, itemCount: 0 })
const filterType = ref<string | null>(null)

const createForm = reactive({ name: '', type: 'openai', base_url: '', api_key: '' })
const editForm = reactive({ name: '', base_url: '', weight: 0, max_rpm: 0, max_tpm: 0, max_daily_requests: 0 })
const accountForm = reactive({ api_key: '', remark: '' })

const channelTypeOptions = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'OpenAI Response', value: 'openai-response' },
  { label: 'Anthropic', value: 'anthropic' },
  { label: 'Google Gemini', value: 'gemini' },
]

// 各渠道类型对应的官方默认 Base URL
const defaultBaseURLs: Record<string, string> = {
  openai: 'https://api.openai.com',
  'openai-response': 'https://api.openai.com',
  anthropic: 'https://api.anthropic.com',
  gemini: 'https://generativelanguage.googleapis.com',
}

// 选择渠道类型时自动填充默认 Base URL（仅当用户未手动修改时）
watch(() => createForm.type, (newType) => {
  const defaultURL = defaultBaseURLs[newType]
  if (defaultURL && (!createForm.base_url || Object.values(defaultBaseURLs).includes(createForm.base_url))) {
    createForm.base_url = defaultURL
  }
})

const columns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('channels.name'), key: 'name' },
  { title: t('channels.type'), key: 'type', width: 120 },
  { title: t('channels.baseUrl'), key: 'base_url', ellipsis: true },
  {
    title: t('common.weight'), key: 'weight', width: 130, sorter: (a: Channel, b: Channel) => a.weight - b.weight, defaultSortOrder: 'descend',
    render: (row: Channel) => h(NInputNumber, {
      value: editingWeightMap[row.id] ?? row.weight, size: 'small', min: 0, style: { width: '90px' },
      'onUpdate:value': (val: number | null) => { if (val !== null) editingWeightMap[row.id] = val },
      onBlur: () => handleUpdateWeight(row),
    }),
  },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (row: Channel) => h(NTag, { type: row.status === 'active' ? 'success' : 'error', size: 'small' }, () => row.status === 'active' ? t('common.active') : t('common.disabled')),
  },
  {
    title: t('common.actions'), key: 'actions', width: 280,
    render: (row: Channel) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', onClick: () => selectChannel(row) }, () => t('common.detail')),
      h(NButton, { size: 'small', type: row.status === 'active' ? 'error' : 'success', onClick: () => handleToggleChannel(row) }, () => row.status === 'active' ? t('common.disable') : t('common.enable')),
      h(NButton, { size: 'small', type: 'error', ghost: true, onClick: () => handleDeleteChannel(row) }, () => t('common.delete')),
    ]),
  },
])

const accountColumns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('channels.keyMask'), key: 'api_key_mask' },
  {
    title: t('channels.remark'), key: 'remark', width: 180,
    render: (row: Account) => {
      if (editingRemarkId.value === row.id) {
        return h(NInput, {
          value: editingRemark.value,
          size: 'small',
          autofocus: true,
          style: { width: '150px' },
          'onUpdate:value': (val: string) => { editingRemark.value = val },
          onKeyup: (e: KeyboardEvent) => {
            if (e.key === 'Enter') handleSaveRemark(row.id)
            if (e.key === 'Escape') { editingRemarkId.value = null }
          },
          onBlur: () => handleSaveRemark(row.id),
        })
      }
      return h('span', {
        style: { cursor: 'pointer', borderBottom: '1px dashed var(--text-tertiary)' },
        onDblclick: () => {
          editingRemarkId.value = row.id
          editingRemark.value = row.remark || ''
        },
      }, row.remark || '-')
    },
  },
  { title: t('common.priority'), key: 'priority', width: 130, sorter: (a: Account, b: Account) => a.priority - b.priority, defaultSortOrder: 'descend',
    render: (row: Account) => h(NInputNumber, {
      value: editingPriorityMap[row.id] ?? row.priority, size: 'small', min: 0, style: { width: '90px' },
      'onUpdate:value': (val: number | null) => { if (val !== null) editingPriorityMap[row.id] = val },
      onBlur: () => handleUpdatePriority(row),
    }),
  },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (row: Account) => h(NTag, { type: row.status === 'active' ? 'success' : 'error', size: 'small' }, () => row.status === 'active' ? t('common.active') : t('common.disabled')),
  },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row: Account) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', type: row.status === 'active' ? 'error' : 'success', onClick: () => handleToggleAccount(row) }, () => row.status === 'active' ? t('common.disable') : t('common.enable')),
      h(NButton, { size: 'small', type: 'error', ghost: true, onClick: () => handleDeleteAccount(row.id) }, () => t('common.delete')),
    ]),
  },
])

async function loadChannels() {
  loading.value = true
  try {
    const res = await channelApi.list({ page: pagination.page, page_size: pagination.pageSize, type: filterType.value || undefined })
    channels.value = res.data.data.sort((a: Channel, b: Channel) => {
      if (b.weight !== a.weight) return b.weight - a.weight
      return b.id - a.id
    })
    total.value = res.data.total
    pagination.itemCount = res.data.total
  } finally {
    loading.value = false
  }
}
const total = ref(0)

function handlePageChange(page: number) {
  pagination.page = page
  loadChannels()
}

async function loadAccounts(channelId: number) {
  const accRes = await accountApi.listByChannel(channelId)
  accounts.value = accRes.data.data.sort((a: Account, b: Account) => {
    if (b.priority !== a.priority) return b.priority - a.priority
    return b.id - a.id
  })
}

async function selectChannel(ch: Channel) {
  selectedChannel.value = ch
  editForm.name = ch.name
  editForm.base_url = ch.base_url
  editForm.weight = ch.weight
  editForm.max_rpm = ch.max_rpm ?? 0
  editForm.max_tpm = ch.max_tpm ?? 0
  editForm.max_daily_requests = ch.max_daily_requests ?? 0
  // 加载账号
  await loadAccounts(ch.id)
  // 加载已配置模型
  try {
    const res = await channelApi.getModelsByChannel(ch.id)
    channelModels.value = res.data.data || []
  } catch {
    channelModels.value = []
  }
}

async function copyModelName(name: string) {
  try {
    await navigator.clipboard.writeText(name)
    message.success(t('common.copied'))
  } catch {
    message.error(t('common.copyFailed'))
  }
}

async function handleUpdateWeight(row: Channel) {
  const newWeight = editingWeightMap[row.id]
  if (newWeight === undefined || newWeight === row.weight) {
    delete editingWeightMap[row.id]
    return
  }
  try {
    await channelApi.updateWeight(row.id, newWeight)
    row.weight = newWeight
    delete editingWeightMap[row.id]
    // 提交成功后重新排序
    channels.value = [...channels.value].sort((a, b) => {
      if (b.weight !== a.weight) return b.weight - a.weight
      return b.id - a.id
    })
    message.success(t('common.success'))
  } catch {
    message.error(t('common.operationFailed'))
    delete editingWeightMap[row.id]
  }
}

async function handleUpdatePriority(row: Account) {
  const newPriority = editingPriorityMap[row.id]
  if (newPriority === undefined || newPriority === row.priority) {
    delete editingPriorityMap[row.id]
    return
  }
  try {
    await accountApi.updatePriority(row.id, newPriority)
    row.priority = newPriority
    delete editingPriorityMap[row.id]
    // 提交成功后重新排序
    accounts.value = [...accounts.value].sort((a, b) => {
      if (b.priority !== a.priority) return b.priority - a.priority
      return b.id - a.id
    })
    message.success(t('common.success'))
  } catch {
    message.error(t('common.operationFailed'))
    delete editingPriorityMap[row.id]
  }
}

async function handleCreateChannel() {
  const missing: string[] = []
  if (!createForm.name) missing.push(t('common.name'))
  if (!createForm.base_url) missing.push(t('channels.baseUrl'))
  if (!createForm.api_key) missing.push(t('channels.apiKeyRequired'))
  if (missing.length > 0) {
    message.warning(t('channels.missingFields') + missing.join('、'))
    return false
  }
  try {
    await channelApi.create(createForm)
    message.success(t('common.success'))
    showCreateModal.value = false
    createForm.name = ''
    createForm.base_url = ''
    createForm.api_key = ''
    testConnectionResult.value = null
    loadChannels()
  } catch { message.error(t('common.createFailed')) }
  return false
}

async function handleTestConnection() {
  if (!createForm.base_url || !createForm.api_key) {
    message.warning(t('common.operationFailed'))
    return
  }
  testingConnection.value = true
  testConnectionResult.value = null
  try {
    const res = await channelApi.testConnection({
      type: createForm.type,
      base_url: createForm.base_url,
      api_key: createForm.api_key,
    })
    testConnectionResult.value = res.data.success
    if (!res.data.success) {
      testConnectionError.value = res.data.error || t('channels.testConnectionFailed')
    }
  } catch {
    testConnectionResult.value = false
    testConnectionError.value = t('channels.testConnectionFailed')
  } finally {
    testingConnection.value = false
  }
}

async function handleUpdateChannel() {
  if (!selectedChannel.value) return
  try {
    await channelApi.update(selectedChannel.value.id, editForm)
    message.success(t('common.success'))
    loadChannels()
  } catch { message.error(t('common.operationFailed')) }
}

async function handleModelSave(models: ChannelModel[]) {
  if (!selectedChannel.value) return
  try {
    await channelApi.saveModels(selectedChannel.value.id, models)
    message.success(t('common.success'))
    // 重新从后端加载，确保数据一致
    const res = await channelApi.getModelsByChannel(selectedChannel.value.id)
    channelModels.value = res.data.data || []
  } catch { message.error(t('common.operationFailed')) }
}

async function handleToggleChannel(row: Channel) {
  const newStatus = row.status === 'active' ? 'disabled' : 'active'
  try {
    await channelApi.updateStatus(row.id, newStatus)
    message.success(t('common.success'))
    loadChannels()
  } catch { message.error(t('common.operationFailed')) }
}

function handleDeleteChannel(row: Channel) {
  dialog.warning({
    title: t('common.confirm'),
    content: t('channels.deleteConfirm', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await channelApi.delete(row.id)
        message.success(t('common.success'))
        if (selectedChannel.value?.id === row.id) selectedChannel.value = null
        loadChannels()
      } catch { message.error(t('common.operationFailed')) }
    },
  })
}

async function handleAddAccount() {
  if (!selectedChannel.value) return
  try {
    await accountApi.create({ channel_id: selectedChannel.value.id, api_key: accountForm.api_key, remark: accountForm.remark })
    message.success(t('common.success'))
    showAddAccount.value = false
    accountForm.api_key = ''
    accountForm.remark = ''
    selectChannel(selectedChannel.value)
  } catch { message.error(t('common.createFailed')) }
}

async function handleToggleAccount(row: Account) {
  const newStatus = row.status === 'active' ? 'disabled' : 'active'
  try {
    await accountApi.updateStatus(row.id, newStatus)
    message.success(t('common.success'))
    if (selectedChannel.value) selectChannel(selectedChannel.value)
  } catch { message.error(t('common.operationFailed')) }
}

async function handleDeleteAccount(id: number) {
  try {
    await accountApi.delete(id)
    message.success(t('common.deleted'))
    if (selectedChannel.value) selectChannel(selectedChannel.value)
  } catch { message.error(t('common.operationFailed')) }
}

async function handleSaveRemark(id: number) {
  if (editingRemarkId.value === null) return // 防止 Enter + onBlur 重复提交
  const newRemark = editingRemark.value.trim()
  editingRemarkId.value = null
  try {
    await accountApi.updateRemark(id, newRemark)
    message.success(t('common.success'))
    if (selectedChannel.value) selectChannel(selectedChannel.value)
  } catch { message.error(t('common.operationFailed')) }
}

onMounted(() => loadChannels())
</script>
