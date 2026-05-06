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
      <template #header-extra>
        <n-button @click="selectedChannel = null">{{ t('common.back') }}</n-button>
      </template>

      <n-tabs type="line" animated>
        <!-- 基本信息 -->
        <n-tab-pane name="info" :tab="t('channels.basicInfo')">
          <n-form :model="editForm" label-placement="left" label-width="100">
            <n-form-item label="Name"><n-input v-model:value="editForm.name" /></n-form-item>
            <n-form-item label="Base URL"><n-input v-model:value="editForm.base_url" /></n-form-item>
            <n-form-item label="Type"><n-input :value="selectedChannel.type" disabled /></n-form-item>
            <n-form-item label="Weight"><n-input-number v-model:value="editForm.weight" :min="0" /></n-form-item>
            <n-form-item><n-button type="primary" @click="handleUpdateChannel">{{ t('common.save') }}</n-button></n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 模型配置 -->
        <n-tab-pane name="models" :tab="t('channels.models')">
          <n-space vertical>
            <n-space>
              <n-button type="primary" :loading="fetchingModels" @click="handleFetchModels">{{ t('channels.fetchModels') }}</n-button>
              <n-input v-model:value="testKey" :placeholder="t('channels.testKeyPlaceholder')" style="width: 300px" size="small" />
            </n-space>
            <n-data-table :columns="modelColumns" :data="channelModels" :row-key="(r: any) => r.display_model_name" />
            <n-button type="primary" @click="handleSaveModels">{{ t('common.save') }}</n-button>
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
    <n-modal v-model:show="showCreateModal" preset="dialog" :title="t('channels.create')" positive-text="OK" negative-text="Cancel" @positive-click="handleCreateChannel">
      <n-form :model="createForm">
        <n-form-item label="Name"><n-input v-model:value="createForm.name" /></n-form-item>
        <n-form-item label="Type">
          <n-select v-model:value="createForm.type" :options="channelTypeOptions" />
        </n-form-item>
        <n-form-item label="Base URL"><n-input v-model:value="createForm.base_url" /></n-form-item>
      </n-form>
    </n-modal>

    <!-- 添加账号弹窗 -->
    <n-modal v-model:show="showAddAccount" preset="dialog" :title="t('channels.addAccount')" positive-text="OK" negative-text="Cancel" @positive-click="handleAddAccount">
      <n-form :model="accountForm">
        <n-form-item label="API Key"><n-input v-model:value="accountForm.api_key" type="password" show-password-on="click" /></n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NButton, NSpace, NTag, NSwitch } from 'naive-ui'
import { channelApi, type Channel, type ChannelModel, type ModelInfo } from '../api/channel'
import { accountApi, type Account } from '../api/account'

const { t } = useI18n()
const message = useMessage()

const loading = ref(false)
const channels = ref<Channel[]>([])
const selectedChannel = ref<Channel | null>(null)
const channelModels = ref<ChannelModel[]>([])
const accounts = ref<Account[]>([])
const fetchingModels = ref(false)
const testKey = ref('')
const showCreateModal = ref(false)
const showAddAccount = ref(false)

const pagination = reactive({ page: 1, pageSize: 20, itemCount: 0 })
const filterType = ref<string | null>(null)

const createForm = reactive({ name: '', type: 'openai', base_url: '' })
const editForm = reactive({ name: '', base_url: '', weight: 0 })
const accountForm = reactive({ api_key: '' })

const channelTypeOptions = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'OpenAI Response', value: 'openai-response' },
  { label: 'Anthropic', value: 'anthropic' },
  { label: 'Google Gemini', value: 'gemini' },
]

const columns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('channels.name'), key: 'name' },
  { title: t('channels.type'), key: 'type', width: 120 },
  { title: t('channels.baseUrl'), key: 'base_url', ellipsis: true },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (row: Channel) => h(NTag, { type: row.status === 'active' ? 'success' : 'error', size: 'small' }, () => row.status),
  },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row: Channel) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', onClick: () => selectChannel(row) }, () => t('common.detail')),
      h(NButton, { size: 'small', type: row.status === 'active' ? 'error' : 'success', onClick: () => handleToggleChannel(row) }, () => row.status === 'active' ? t('common.disable') : t('common.enable')),
    ]),
  },
])

const modelColumns = computed(() => [
  { title: t('channels.displayName'), key: 'display_model_name' },
  { title: t('channels.actualName'), key: 'actual_model_name' },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (row: ChannelModel) => h(NSwitch, {
      value: row.status === 'enabled',
      onUpdateValue: (v: boolean) => { row.status = v ? 'enabled' : 'disabled' },
    }),
  },
])

const accountColumns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('channels.keyMask'), key: 'api_key_mask' },
  { title: t('channels.priority'), key: 'priority', width: 100 },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (row: Account) => h(NTag, { type: row.status === 'active' ? 'success' : 'error', size: 'small' }, () => row.status),
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
    channels.value = res.data.data
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

async function selectChannel(ch: Channel) {
  selectedChannel.value = ch
  editForm.name = ch.name
  editForm.base_url = ch.base_url
  editForm.weight = ch.weight
  // 加载账号
  const accRes = await accountApi.listByChannel(ch.id)
  accounts.value = accRes.data.data
}

async function handleCreateChannel() {
  try {
    await channelApi.create(createForm)
    message.success(t('common.success'))
    showCreateModal.value = false
    createForm.name = ''
    createForm.base_url = ''
    loadChannels()
  } catch { message.error(t('common.createFailed')) }
}

async function handleUpdateChannel() {
  if (!selectedChannel.value) return
  try {
    await channelApi.update(selectedChannel.value.id, editForm)
    message.success(t('common.success'))
    loadChannels()
  } catch { message.error(t('common.operationFailed')) }
}

async function handleFetchModels() {
  if (!selectedChannel.value) return
  fetchingModels.value = true
  try {
    const res = await channelApi.fetchModels(selectedChannel.value.id, testKey.value)
    const models: ModelInfo[] = res.data.data
    // 转换为 ChannelModel 格式
    channelModels.value = models.map((m) => ({
      channel_id: selectedChannel.value!.id,
      display_model_name: m.id,
      actual_model_name: m.id,
      status: 'enabled' as const,
    }))
  } catch { message.error(t('common.operationFailed')) }
  finally { fetchingModels.value = false }
}

async function handleSaveModels() {
  if (!selectedChannel.value) return
  try {
    await channelApi.saveModels(selectedChannel.value.id, channelModels.value)
    message.success(t('common.success'))
  } catch { message.error(t('common.operationFailed')) }
}

async function handleToggleChannel(row: Channel) {
  try {
    await channelApi.update(row.id, { weight: row.weight })
    message.success(t('common.success'))
    loadChannels()
  } catch { message.error(t('common.operationFailed')) }
}

async function handleAddAccount() {
  if (!selectedChannel.value) return
  try {
    await accountApi.create({ channel_id: selectedChannel.value.id, api_key: accountForm.api_key })
    message.success(t('common.success'))
    showAddAccount.value = false
    accountForm.api_key = ''
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

onMounted(() => loadChannels())
</script>
