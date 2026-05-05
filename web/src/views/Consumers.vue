<template>
  <div>
    <n-card :title="t('consumers.title')">
      <template #header-extra>
        <n-button type="primary" @click="showCreateModal = true">{{ t('common.create') }}</n-button>
      </template>

      <n-space vertical size="large">
        <!-- 筛选栏 -->
        <n-space>
          <n-input v-model:value="filterName" :placeholder="t('consumers.searchName')" clearable style="width: 200px" @keyup.enter="loadData" />
          <n-select v-model:value="filterStatus" :options="statusOptions" :placeholder="t('common.status')" clearable style="width: 120px" />
          <n-button @click="loadData">{{ t('common.search') }}</n-button>
        </n-space>

        <!-- 表格 -->
        <n-data-table :columns="columns" :data="consumers" :loading="loading" :pagination="pagination" remote @update:page="handlePageChange" />
      </n-space>
    </n-card>

    <!-- 创建消费者弹窗 -->
    <n-modal v-model:show="showCreateModal" preset="dialog" :title="t('consumers.create')" positive-text="OK" negative-text="Cancel" @positive-click="handleCreate">
      <n-form :model="createForm">
        <n-form-item :label="t('consumers.name')" path="name">
          <n-input v-model:value="createForm.name" :placeholder="t('consumers.namePlaceholder')" />
        </n-form-item>
      </n-form>
    </n-modal>

    <!-- 新建成功显示 Key -->
    <n-modal v-model:show="showKeyModal" preset="dialog" :title="t('consumers.keyCreated')" :show-icon="false" :mask-closable="false">
      <n-alert type="warning" :title="t('consumers.keyWarning')" style="margin-bottom: 12px" />
      <n-input :value="createdKey" readonly type="textarea" :rows="2" />
      <template #action>
        <n-button type="primary" @click="copyCreatedKey">{{ t('common.copy') }}</n-button>
        <n-button @click="showKeyModal = false">{{ t('common.close') }}</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NButton, NSpace, NTag } from 'naive-ui'
import { consumerApi, type Consumer } from '../api/consumer'

const { t } = useI18n()
const message = useMessage()

const loading = ref(false)
const consumers = ref<Consumer[]>([])
const total = ref(0)
const filterName = ref('')
const filterStatus = ref<string | null>(null)
const showCreateModal = ref(false)
const showKeyModal = ref(false)
const createdKey = ref('')

const createForm = reactive({ name: '' })

const pagination = reactive({ page: 1, pageSize: 20, itemCount: 0 })

const statusOptions = computed(() => [
  { label: t('common.active'), value: 'active' },
  { label: t('common.disabled'), value: 'disabled' },
])

const columns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('consumers.name'), key: 'name' },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (row: Consumer) => h(NTag, { type: row.status === 'active' ? 'success' : 'error', size: 'small' }, () => row.status),
  },
  { title: t('common.createdAt'), key: 'created_at', width: 180 },
  {
    title: t('common.actions'), key: 'actions', width: 280,
    render: (row: Consumer) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', onClick: () => handleResetKey(row.id) }, () => t('consumers.resetKey')),
      h(NButton, { size: 'small', type: row.status === 'active' ? 'error' : 'success', onClick: () => handleToggleStatus(row) }, () => row.status === 'active' ? t('common.disable') : t('common.enable')),
      h(NButton, { size: 'small', type: 'error', ghost: true, onClick: () => handleDelete(row.id) }, () => t('common.delete')),
    ]),
  },
])

async function loadData() {
  loading.value = true
  try {
    const res = await consumerApi.list({
      page: pagination.page,
      page_size: pagination.pageSize,
      name: filterName.value || undefined,
      status: filterStatus.value || undefined,
    })
    consumers.value = res.data.data
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
    const res = await consumerApi.create({ name: createForm.name })
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

async function handleResetKey(id: number) {
  try {
    const res = await consumerApi.resetKey(id)
    createdKey.value = res.data.data.api_key
    showKeyModal.value = true
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleToggleStatus(row: Consumer) {
  const newStatus = row.status === 'active' ? 'disabled' : 'active'
  try {
    await consumerApi.updateStatus(row.id, newStatus)
    message.success(t('common.success'))
    loadData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleDelete(id: number) {
  try {
    await consumerApi.delete(id)
    message.success(t('common.deleted'))
    loadData()
  } catch {
    message.error(t('common.operationFailed'))
  }
}

onMounted(() => loadData())
</script>
