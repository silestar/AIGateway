<template>
  <n-card :bordered="false" class="glass-card">
    <template #header>
      <h2 class="page-title" style="margin:0">{{ t('keys.title') }}</h2>
    </template>
    <template #header-extra>
      <n-button type="primary" @click="showCreateModal = true">+ {{ t('common.create') }}</n-button>
    </template>

    <n-space vertical size="large">
      <!-- 筛选栏 -->
      <n-space>
        <n-input v-model:value="filterName" :placeholder="t('keys.searchName')" clearable style="width: 200px" @keyup.enter="loadData" />
        <n-select v-model:value="filterStatus" :options="statusOptions" :placeholder="t('common.status')" clearable style="width: 120px" />
        <n-button @click="loadData">{{ t('common.search') }}</n-button>
      </n-space>

      <!-- 表格 -->
      <n-data-table :columns="columns" :data="keys" :loading="loading" :pagination="pagination" remote @update:page="handlePageChange" />
    </n-space>

    <!-- 创建密钥弹窗 -->
    <n-modal v-model:show="showCreateModal" preset="dialog" :title="t('keys.create')" positive-text="OK" negative-text="Cancel" @positive-click="handleCreate">
      <n-form :model="createForm">
        <n-form-item :label="t('keys.name')" path="name">
          <n-input v-model:value="createForm.name" :placeholder="t('keys.namePlaceholder')" />
        </n-form-item>
      </n-form>
    </n-modal>

    <!-- 新建成功显示 Key -->
    <n-modal v-model:show="showKeyModal" preset="dialog" :title="t('keys.keyCreated')" :show-icon="false" :mask-closable="false">
      <n-alert type="warning" :title="t('keys.keyWarning')" style="margin-bottom: 12px" />
      <n-input :value="createdKey" readonly type="textarea" :rows="2" />
      <template #action>
        <n-button type="primary" @click="copyCreatedKey">{{ t('common.copy') }}</n-button>
        <n-button @click="showKeyModal = false">{{ t('common.close') }}</n-button>
      </template>
    </n-modal>
  </n-card>
</template>

<script setup lang="ts">
import { ref, reactive, computed, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NButton, NSpace, NTag, NIcon, NInput } from 'naive-ui'
import { KeyOutlined } from '@vicons/antd'
import { keysApi, type Keys } from '../api/keys'

const { t } = useI18n()
const message = useMessage()

const loading = ref(false)
const keys = ref<Keys[]>([])
const total = ref(0)
const filterName = ref('')
const filterStatus = ref<string | null>(null)
const showCreateModal = ref(false)
const showKeyModal = ref(false)
const createdKey = ref('')

// 双击编辑名称相关
const editingId = ref<number | null>(null)
const editingName = ref('')

const createForm = reactive({ name: '' })

const pagination = reactive({ page: 1, pageSize: 20, itemCount: 0 })

const statusOptions = computed(() => [
  { label: t('common.active'), value: 'active' },
  { label: t('common.disabled'), value: 'disabled' },
])

const columns = computed(() => [
  { title: 'ID', key: 'id', width: 60 },
  {
    title: t('keys.name'), key: 'name', width: 140,
    render: (row: Keys) => {
      if (editingId.value === row.id) {
        return h(NInput, {
          value: editingName.value,
          size: 'small',
          autofocus: true,
          style: { width: '120px' },
          'onUpdate:value': (val: string) => { editingName.value = val },
          onKeyup: (e: KeyboardEvent) => {
            if (e.key === 'Enter') handleSaveName(row.id)
            if (e.key === 'Escape') { editingId.value = null }
          },
          onBlur: () => handleSaveName(row.id),
        })
      }
      return h('span', {
        style: { cursor: 'pointer', borderBottom: '1px dashed var(--text-tertiary)' },
        onDblclick: () => {
          editingId.value = row.id
          editingName.value = row.name
        },
      }, row.name)
    },
  },
  {
    title: t('keys.keyLabel'), key: 'api_key_prefix', width: 260,
    render: (row: Keys) => {
      const prefix = row.api_key_prefix || 'sk-...'
      return h(NSpace, { size: 'small', align: 'center' }, () => [
        h('span', { style: { fontFamily: 'monospace', fontSize: '13px' } }, prefix + '****'),
        h(NButton, {
          size: 'tiny',
          quaternary: true,
          title: t('keys.clickToCopy'),
          onClick: () => handleCopyKey(row.id),
        }, () => h(NIcon, { component: KeyOutlined, style: { color: '#00d2ff', cursor: 'pointer' } })),
      ])
    },
  },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row: Keys) => h(NTag, { type: row.status === 'active' ? 'success' : 'error', size: 'small' }, () => row.status === 'active' ? t('common.active') : t('common.disabled')),
  },
  {
    title: t('common.createdAt'), key: 'created_at', width: 180,
    render: (row: Keys) => {
      if (!row.created_at) return '-'
      const d = new Date(row.created_at)
      const pad = (n: number) => String(n).padStart(2, '0')
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
    },
  },
  {
    title: t('common.actions'), key: 'actions', width: 260,
    render: (row: Keys) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', onClick: () => handleResetKey(row.id) }, () => t('keys.resetKey')),
      h(NButton, { size: 'small', type: row.status === 'active' ? 'error' : 'success', onClick: () => handleToggleStatus(row) }, () => row.status === 'active' ? t('common.disable') : t('common.enable')),
      h(NButton, { size: 'small', type: 'error', ghost: true, onClick: () => handleDelete(row.id) }, () => t('common.delete')),
    ]),
  },
])

async function loadData() {
  loading.value = true
  try {
    const res = await keysApi.list({
      page: pagination.page,
      page_size: pagination.pageSize,
      name: filterName.value || undefined,
      status: filterStatus.value || undefined,
    })
    keys.value = res.data.data
    total.value = res.data.total
    pagination.itemCount = res.data.total
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  loadData()
}

async function handleCreate() {
  if (!createForm.name) return
  try {
    const res = await keysApi.create({ name: createForm.name })
    createdKey.value = res.data.data.api_key
    showKeyModal.value = true
    createForm.name = ''
    loadData()
  } catch {
    message.error(t('common.createFailed'))
  }
}

async function copyCreatedKey() {
  try {
    await navigator.clipboard.writeText(createdKey.value)
    message.success(t('common.copied'))
  } catch {
    message.error(t('common.copyFailed'))
  }
}

async function handleCopyKey(id: number) {
  try {
    const res = await keysApi.revealKey(id)
    const apiKey = res.data.data.api_key
    if (apiKey) {
      await navigator.clipboard.writeText(apiKey)
      message.success(t('common.copied'))
    } else {
      message.warning(t('common.keyNotAvailable'))
    }
  } catch {
    message.error(t('common.copyFailed'))
  }
}

async function handleResetKey(id: number) {
  try {
    const res = await keysApi.resetKey(id)
    createdKey.value = res.data.data.api_key
    showKeyModal.value = true
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleToggleStatus(row: Keys) {
  const newStatus = row.status === 'active' ? 'disabled' : 'active'
  try {
    await keysApi.updateStatus(row.id, newStatus)
    message.success(t('common.success'))
    loadData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleSaveName(id: number) {
  if (editingId.value === null) return // 防止 Enter + onBlur 重复提交
  const newName = editingName.value.trim()
  editingId.value = null
  if (!newName) return
  try {
    await keysApi.update(id, { name: newName })
    message.success(t('common.success'))
    loadData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleDelete(id: number) {
  try {
    await keysApi.delete(id)
    message.success(t('common.deleted'))
    loadData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

onMounted(() => loadData())
</script>