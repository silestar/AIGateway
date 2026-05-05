<template>
  <n-card :title="t('settings.title')">
    <n-descriptions bordered :column="1">
      <n-descriptions-item label="Version">0.1.0</n-descriptions-item>
      <n-descriptions-item label="Port">{{ systemInfo.port || '-' }}</n-descriptions-item>
      <n-descriptions-item label="DB Type">{{ systemInfo.db_type || '-' }}</n-descriptions-item>
    </n-descriptions>

    <n-divider />

    <n-space>
      <n-button @click="loadConfig">{{ t('settings.loadConfig') }}</n-button>
      <n-button type="primary" @click="handleDownloadLogs">{{ t('settings.downloadLogs') }}</n-button>
    </n-space>
  </n-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { systemApi } from '../api/system'

const { t } = useI18n()
const message = useMessage()

const systemInfo = ref<Record<string, unknown>>({})

async function loadConfig() {
  try {
    const res = await systemApi.info()
    systemInfo.value = res.data.data
  } catch { message.error('Failed to load config') }
}

async function handleDownloadLogs() {
  try {
    const res = await systemApi.downloadLogs()
    const url = URL.createObjectURL(new Blob([res.data]))
    const a = document.createElement('a')
    a.href = url; a.download = 'agw.log'; a.click()
    URL.revokeObjectURL(url)
  } catch { message.error('Failed to download logs') }
}

onMounted(() => loadConfig())
</script>
