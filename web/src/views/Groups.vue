<template>
  <div>
    <n-tabs type="line" animated>
      <!-- 渠道分组 -->
      <n-tab-pane name="channel" :tab="t('groups.channelGroups')">
        <n-card :bordered="false" class="glass-card">
          <template #header>
            <h2 class="page-title" style="margin:0">{{ t('groups.channelGroups') }}</h2>
          </template>
          <template #header-extra>
            <n-button type="primary" @click="showCreateCG = true">+ {{ t('common.create') }}</n-button>
          </template>
          <n-data-table :columns="cgColumns" :data="channelGroups" :loading="cgLoading" />
        </n-card>
      </n-tab-pane>

      <!-- 密钥分组 -->
      <n-tab-pane name="keys" :tab="t('groups.keysGroups')">
        <n-card :bordered="false" class="glass-card">
          <template #header>
            <h2 class="page-title" style="margin:0">{{ t('groups.keysGroups') }}</h2>
          </template>
          <template #header-extra>
            <n-button type="primary" @click="showCreateKG = true">+ {{ t('common.create') }}</n-button>
          </template>
          <n-data-table :columns="kgColumns" :data="keysGroups" :loading="kgLoading" />
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

    <!-- 创建密钥分组 -->
    <n-modal v-model:show="showCreateKG" preset="dialog" :title="t('groups.createKeysGroup')" positive-text="OK" negative-text="Cancel" @positive-click="handleCreateKG">
      <n-form :model="kgForm">
        <n-form-item label="Name"><n-input v-model:value="kgForm.name" /></n-form-item>
        <n-form-item label="Description"><n-input v-model:value="kgForm.description" type="textarea" /></n-form-item>
        <n-form-item label="Quota RPM"><n-input-number v-model:value="kgForm.quota_rpm" :min="0" /></n-form-item>
        <n-form-item label="Quota TPM"><n-input-number v-model:value="kgForm.quota_tpm" :min="0" /></n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NButton } from 'naive-ui'
import { groupApi, type ChannelGroup, type KeysGroup } from '../api/group'

const { t } = useI18n()
const message = useMessage()

const channelGroups = ref<ChannelGroup[]>([])
const keysGroups = ref<KeysGroup[]>([])
const cgLoading = ref(false)
const kgLoading = ref(false)
const showCreateCG = ref(false)
const showCreateKG = ref(false)

const cgForm = reactive({ name: '', description: '', weight: 0 })
const kgForm = reactive({ name: '', description: '', quota_rpm: 0, quota_tpm: 0 })

const cgColumns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('groups.name'), key: 'name' },
  { title: t('groups.description'), key: 'description', ellipsis: true },
  { title: t('groups.weight'), key: 'weight', width: 100 },
  { title: 'Channels', key: 'channel_count', width: 100 },
  {
    title: t('common.actions'), key: 'actions', width: 120,
    render: (row: ChannelGroup) => h(NButton, { size: 'small', type: 'error', ghost: true, onClick: () => handleDeleteCG(row.id) }, () => t('common.delete')),
  },
])

const kgColumns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('groups.name'), key: 'name' },
  { title: t('groups.description'), key: 'description', ellipsis: true },
  { title: 'RPM', key: 'quota_rpm', width: 90 },
  { title: 'TPM', key: 'quota_tpm', width: 90 },
  { title: '绑定渠道组', key: 'channel_count', width: 100 },
  {
    title: t('common.actions'), key: 'actions', width: 120,
    render: (row: KeysGroup) => h(NButton, { size: 'small', type: 'error', ghost: true, onClick: () => handleDeleteKG(row.id) }, () => t('common.delete')),
  },
])

async function loadChannelGroups() {
  cgLoading.value = true
  try { const res = await groupApi.listChannelGroups(); channelGroups.value = res.data.data || [] }
  finally { cgLoading.value = false }
}

async function loadKeysGroups() {
  kgLoading.value = true
  try { const res = await groupApi.listKeysGroups(); keysGroups.value = res.data.data || [] }
  finally { kgLoading.value = false }
}

async function handleCreateCG() {
  try { await groupApi.createChannelGroup(cgForm); message.success(t('common.success')); showCreateCG.value = false; loadChannelGroups() }
  catch { message.error(t('common.createFailed')) }
}

async function handleCreateKG() {
  try { await groupApi.createKeysGroup(kgForm); message.success(t('common.success')); showCreateKG.value = false; loadKeysGroups() }
  catch { message.error(t('common.createFailed')) }
}

async function handleDeleteCG(id: number) {
  try { await groupApi.deleteChannelGroup(id); message.success(t('common.deleted')); loadChannelGroups() }
  catch { message.error(t('common.operationFailed')) }
}

async function handleDeleteKG(id: number) {
  try { await groupApi.deleteKeysGroup(id); message.success(t('common.deleted')); loadKeysGroups() }
  catch { message.error(t('common.operationFailed')) }
}

onMounted(() => { loadChannelGroups(); loadKeysGroups() })
</script>