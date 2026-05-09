<template>
  <n-card :bordered="false" class="glass-card">
    <template #header>
      <div style="display:flex;align-items:center;justify-content:space-between">
        <h2 class="page-title" style="margin:0">{{ t('plugins.title') }}</h2>
        <n-space>
          <n-button tertiary :disabled="true">
            <template #icon><n-icon><StoreIcon /></n-icon></template>
            {{ t('plugins.market') }}
          </n-button>
          <n-upload
            :show-file-list="false"
            accept=".zip"
            :custom-request="handleUpload"
          >
            <n-button type="primary">
              <template #icon><n-icon><UploadIcon /></n-icon></template>
              {{ t('plugins.upload') }}
            </n-button>
          </n-upload>
        </n-space>
      </div>
    </template>

    <!-- 空状态 -->
    <n-empty v-if="!loading && plugins.length === 0" :description="t('plugins.emptyDesc')" style="padding:60px 0" />

    <!-- 插件卡片网格 -->
    <n-grid v-else :cols="2" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
      <n-grid-item v-for="plugin in plugins" :key="plugin.id" span="2 m:1">
        <n-card :bordered="true" size="small" hoverable>
          <template #header>
            <div style="display:flex;align-items:center;gap:8px">
              <span style="font-weight:600">{{ plugin.name }}</span>
              <n-tag size="small" :type="versionTagType" bordered>{{ plugin.version }}</n-tag>
            </div>
          </template>
          <template #header-extra>
            <n-tag :type="statusTagType(plugin.status)" size="small" round>
              {{ t('plugins.status' + capitalize(plugin.status)) }}
            </n-tag>
          </template>

          <n-descriptions :column="1" label-placement="left" size="small" :label-style="{width:'60px'}">
            <n-descriptions-item :label="t('plugins.description')">
              {{ plugin.description || '-' }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('plugins.author')">
              {{ plugin.author || '-' }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('plugins.port')">
              {{ plugin.port }}
            </n-descriptions-item>
            <n-descriptions-item v-if="plugin.status === 'running'" :label="t('plugins.pid')">
              {{ plugin.pid }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('plugins.hooks')">
              <n-space size="small">
                <n-tag v-for="hook in parseHooks(plugin.hooks)" :key="hook" size="tiny" type="info">{{ hook }}</n-tag>
              </n-space>
            </n-descriptions-item>
          </n-descriptions>

          <template #action>
            <n-space justify="end">
              <n-button size="small" @click="openConfig(plugin)">
                {{ t('plugins.config') }}
              </n-button>
              <n-button
                v-if="plugin.status !== 'running'"
                size="small"
                type="primary"
                :loading="actionLoading[plugin.id]"
                @click="handleStart(plugin)"
              >
                {{ t('plugins.start') }}
              </n-button>
              <n-button
                v-else
                size="small"
                type="warning"
                :loading="actionLoading[plugin.id]"
                @click="handleStop(plugin)"
              >
                {{ t('plugins.stop') }}
              </n-button>
              <n-button
                size="small"
                type="error"
                ghost
                @click="handleUninstall(plugin)"
              >
                {{ t('plugins.uninstall') }}
              </n-button>
            </n-space>
          </template>
        </n-card>
      </n-grid-item>
    </n-grid>

    <!-- 配置弹窗 -->
    <n-modal v-model:show="configModalShow" preset="card" :title="t('plugins.configTitle')" style="width:500px">
      <n-input
        v-model:value="configText"
        type="textarea"
        :rows="12"
        placeholder="JSON"
        font="monospace"
      />
      <template #action>
        <n-space justify="end">
          <n-button @click="configModalShow = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="configSaving" @click="saveConfig">{{ t('plugins.saveConfig') }}</n-button>
        </n-space>
      </template>
    </n-modal>
  </n-card>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, useDialog } from 'naive-ui'
import {
  NCard, NGrid, NGridItem, NTag, NButton, NSpace, NUpload,
  NDescriptions, NDescriptionsItem, NEmpty, NModal, NInput, NIcon,
} from 'naive-ui'
import { CloudUploadOutline as UploadIcon, AppsOutline as StoreIcon } from '@vicons/ionicons5'
import { pluginApi, type PluginItem } from '../api/plugin'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const plugins = ref<PluginItem[]>([])
const loading = ref(false)
const actionLoading = reactive<Record<number, boolean>>({})
const configModalShow = ref(false)
const configText = ref('')
const configSaving = ref(false)
const configPluginId = ref<number>(0)

const versionTagType = 'default' as const

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1)
}

function statusTagType(status: string): 'default' | 'success' | 'warning' | 'error' | 'info' {
  const map: Record<string, 'default' | 'success' | 'warning' | 'error' | 'info'> = {
    installed: 'default',
    running: 'success',
    stopped: 'warning',
    unhealthy: 'error',
    error: 'error',
  }
  return map[status] || 'default'
}

function parseHooks(hooksStr: string): string[] {
  try {
    return JSON.parse(hooksStr || '[]')
  } catch {
    return []
  }
}

async function fetchPlugins() {
  loading.value = true
  try {
    const { data } = await pluginApi.list()
    plugins.value = data?.data || []
  } finally {
    loading.value = false
  }
}

async function handleUpload({ file, onFinish, onError }: any) {
  const formData = new FormData()
  formData.append('file', file.file)
  try {
    await pluginApi.upload(formData)
    message.success(t('plugins.uploadSuccess'))
    await fetchPlugins()
    onFinish()
  } catch (e: any) {
    message.error(t('plugins.installFailed') + ': ' + (e?.response?.data?.error?.message || e.message))
    onError()
  }
}

async function handleStart(plugin: PluginItem) {
  dialog.warning({
    title: t('plugins.start'),
    content: t('plugins.startConfirm'),
    positiveText: t('plugins.start'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      actionLoading[plugin.id] = true
      try {
        await pluginApi.updateStatus(plugin.id, 'start')
        message.success(t('plugins.startSuccess'))
        await fetchPlugins()
      } catch (e: any) {
        message.error(t('plugins.startFailed') + ': ' + (e?.response?.data?.error?.message || e.message))
      } finally {
        actionLoading[plugin.id] = false
      }
    },
  })
}

async function handleStop(plugin: PluginItem) {
  dialog.warning({
    title: t('plugins.stop'),
    content: t('plugins.stopConfirm'),
    positiveText: t('plugins.stop'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      actionLoading[plugin.id] = true
      try {
        await pluginApi.updateStatus(plugin.id, 'stop')
        message.success(t('plugins.stopSuccess'))
        await fetchPlugins()
      } catch (e: any) {
        message.error(t('plugins.stopFailed') + ': ' + (e?.response?.data?.error?.message || e.message))
      } finally {
        actionLoading[plugin.id] = false
      }
    },
  })
}

function handleUninstall(plugin: PluginItem) {
  dialog.error({
    title: t('plugins.uninstall'),
    content: t('plugins.uninstallConfirm'),
    positiveText: t('plugins.uninstall'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await pluginApi.delete(plugin.id)
        message.success(t('plugins.uninstallSuccess'))
        await fetchPlugins()
      } catch (e: any) {
        message.error(e?.response?.data?.error?.message || 'Error')
      }
    },
  })
}

function openConfig(plugin: PluginItem) {
  configPluginId.value = plugin.id
  try {
    const parsed = JSON.parse(plugin.config || '{}')
    configText.value = JSON.stringify(parsed, null, 2)
  } catch {
    configText.value = plugin.config || '{}'
  }
  configModalShow.value = true
}

async function saveConfig() {
  configSaving.value = true
  try {
    JSON.parse(configText.value) // validate
    await pluginApi.updateConfig(configPluginId.value, configText.value)
    message.success(t('common.saveSuccess'))
    configModalShow.value = false
    await fetchPlugins()
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      message.error('Invalid JSON')
    } else {
      message.error(e?.response?.data?.error?.message || 'Error')
    }
  } finally {
    configSaving.value = false
  }
}

onMounted(fetchPlugins)
</script>
