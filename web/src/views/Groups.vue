<template>
  <div>
    <n-tabs type="line" animated>
      <!-- 渠道分组 -->
      <n-tab-pane name="channel" :tab="t('groups.channelGroups')">
        <n-card :title="t('groups.channelGroups')">
          <template #header-extra>
            <n-button type="primary" @click="showCreateCG = true">{{ t('common.create') }}</n-button>
          </template>
          <n-data-table :columns="cgColumns" :data="channelGroups" :loading="cgLoading" />
        </n-card>
      </n-tab-pane>

      <!-- 消费者分组 -->
      <n-tab-pane name="consumer" :tab="t('groups.consumerGroups')">
        <n-card :title="t('groups.consumerGroups')">
          <template #header-extra>
            <n-button type="primary" @click="showCreateConG = true">{{ t('common.create') }}</n-button>
          </template>
          <n-data-table :columns="conGColumns" :data="consumerGroups" :loading="conGLoading" />
        </n-card>
      </n-tab-pane>
    </n-tabs>

    <!-- 创建渠道分组 -->
    <n-modal v-model:show="showCreateCG" preset="dialog" :title="t('groups.createChannelGroup')" positive-text="OK" negative-text="Cancel" @positive-click="handleCreateCG">
      <n-form :model="cgForm">
        <n-form-item label="Name"><n-input v-model:value="cgForm.name" /></n-form-item>
        <n-form-item label="Description"><n-input v-model:value="cgForm.description" type="textarea" /></n-form-item>
        <n-form-item label="Weight"><n-input-number v-model:value="cgForm.weight" :min="0" /></n-form-item>
      </n-form>
    </n-modal>

    <!-- 创建消费者分组 -->
    <n-modal v-model:show="showCreateConG" preset="dialog" :title="t('groups.createConsumerGroup')" positive-text="OK" negative-text="Cancel" @positive-click="handleCreateConG">
      <n-form :model="conGForm">
        <n-form-item label="Name"><n-input v-model:value="conGForm.name" /></n-form-item>
        <n-form-item label="Description"><n-input v-model:value="conGForm.description" type="textarea" /></n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NButton } from 'naive-ui'
import { groupApi, type ChannelGroup, type ConsumerGroup } from '../api/group'

const { t } = useI18n()
const message = useMessage()

const channelGroups = ref<ChannelGroup[]>([])
const consumerGroups = ref<ConsumerGroup[]>([])
const cgLoading = ref(false)
const conGLoading = ref(false)
const showCreateCG = ref(false)
const showCreateConG = ref(false)

const cgForm = reactive({ name: '', description: '', weight: 0 })
const conGForm = reactive({ name: '', description: '' })

const cgColumns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('groups.name'), key: 'name' },
  { title: t('groups.description'), key: 'description', ellipsis: true },
  { title: t('groups.weight'), key: 'weight', width: 100 },
  {
    title: t('common.actions'), key: 'actions', width: 120,
    render: (row: ChannelGroup) => h(NButton, { size: 'small', type: 'error', ghost: true, onClick: () => handleDeleteCG(row.id) }, () => t('common.delete')),
  },
])

const conGColumns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('groups.name'), key: 'name' },
  { title: t('groups.description'), key: 'description', ellipsis: true },
  {
    title: t('common.actions'), key: 'actions', width: 120,
    render: (row: ConsumerGroup) => h(NButton, { size: 'small', type: 'error', ghost: true, onClick: () => handleDeleteConG(row.id) }, () => t('common.delete')),
  },
])

async function loadChannelGroups() {
  cgLoading.value = true
  try { const res = await groupApi.listChannelGroups(); channelGroups.value = res.data.data || [] }
  finally { cgLoading.value = false }
}

async function loadConsumerGroups() {
  conGLoading.value = true
  try { const res = await groupApi.listConsumerGroups(); consumerGroups.value = res.data.data || [] }
  finally { conGLoading.value = false }
}

async function handleCreateCG() {
  try { await groupApi.createChannelGroup(cgForm); message.success(t('common.success')); showCreateCG.value = false; loadChannelGroups() }
  catch { message.error(t('common.createFailed')) }
}

async function handleCreateConG() {
  try { await groupApi.createConsumerGroup(conGForm); message.success(t('common.success')); showCreateConG.value = false; loadConsumerGroups() }
  catch { message.error(t('common.createFailed')) }
}

async function handleDeleteCG(id: number) {
  try { await groupApi.deleteChannelGroup(id); message.success(t('common.deleted')); loadChannelGroups() }
  catch { message.error(t('common.operationFailed')) }
}

async function handleDeleteConG(id: number) {
  try { await groupApi.deleteConsumerGroup(id); message.success(t('common.deleted')); loadConsumerGroups() }
  catch { message.error(t('common.operationFailed')) }
}

onMounted(() => { loadChannelGroups(); loadConsumerGroups() })
</script>
