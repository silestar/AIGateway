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

    <!-- 配置弹窗：全局配置 + 渠道级配置 Tab -->
    <n-modal v-model:show="configModalShow" preset="card" :title="t('plugins.configTitle')" style="width:600px">
      <n-tabs type="line" size="small">
        <!-- 全局配置 Tab -->
        <n-tab-pane :name="'global'" :tab="t('plugins.globalConfig')">
          <!-- 有 schema 时显示动态表单 -->
          <template v-if="configSchemaFields.length > 0">
            <n-form label-placement="left" label-width="120" size="small">
              <n-form-item v-for="field in configSchemaFields" :key="field.key" :label="field.title || field.key">
                <n-select
                  v-if="field.type === 'string' && field.enum && field.enum.length > 0"
                  v-model:value="configFormValues[field.key]"
                  :options="field.enum.map((v: string) => ({ label: v, value: v }))"
                />
                <n-input
                  v-else-if="field.type === 'string'"
                  v-model:value="configFormValues[field.key]"
                  :placeholder="field.description || ''"
                />
                <n-input-number
                  v-else-if="field.type === 'number' || field.type === 'integer'"
                  v-model:value="configFormValues[field.key]"
                  :min="field.minimum"
                  :max="field.maximum"
                  style="width:100%"
                />
                <n-switch
                  v-else-if="field.type === 'boolean'"
                  v-model:value="configFormValues[field.key]"
                />
                <n-input
                  v-else-if="field.type === 'array'"
                  v-model:value="configFormValues[field.key]"
                  type="textarea"
                  :rows="2"
                  :placeholder="field.description || '[]'"
                />
                <n-input
                  v-else
                  v-model:value="configFormValues[field.key]"
                  type="textarea"
                  :rows="2"
                  :placeholder="field.description || ''"
                />
                <template v-if="field.description" #feedback>
                  <span style="color:#999;font-size:12px">{{ field.description }}</span>
                </template>
              </n-form-item>
            </n-form>
          </template>
          <template v-else>
            <n-input v-model:value="configText" type="textarea" :rows="10" placeholder="JSON" font="monospace" />
          </template>
          <n-button text size="tiny" style="margin-top:8px" @click="toggleAdvancedMode">
            {{ advancedMode ? t('plugins.simpleMode') : t('plugins.advancedMode') }}
          </n-button>
          <n-input v-if="advancedMode" v-model:value="configText" type="textarea" :rows="8" placeholder="JSON" font="monospace" style="margin-top:8px" />
          <n-space justify="end" style="margin-top:12px">
            <n-button type="primary" :loading="configSaving" @click="saveConfig">{{ t('plugins.saveConfig') }}</n-button>
          </n-space>
        </n-tab-pane>

        <!-- 渠道级配置 Tab -->
        <n-tab-pane :name="'channel'" :tab="t('plugins.channelConfig')">
          <n-empty v-if="channelConfigs.length === 0" :description="t('plugins.noChannelConfig')" style="padding:24px 0" />
          <n-card v-for="cc in channelConfigs" :key="cc.id" size="small" style="margin-bottom:8px">
            <template #header>
              {{ t('plugins.channelId') }}: {{ cc.channel_id }}
            </template>
            <n-input v-model:value="cc.config" type="textarea" :rows="3" font="monospace" placeholder="JSON" />
            <template #action>
              <n-space justify="end">
                <n-button size="small" type="primary" @click="saveChannelConfig(cc)">{{ t('common.save') }}</n-button>
                <n-button size="small" type="error" ghost @click="deleteChannelConfig(cc)">{{ t('common.delete') }}</n-button>
              </n-space>
            </template>
          </n-card>
          <!-- 添加渠道配置 -->
          <n-space style="margin-top:12px" align="center">
            <n-input-number v-model:value="newChannelId" :min="1" size="small" :placeholder="t('plugins.channelId')" style="width:120px" />
            <n-button size="small" @click="addChannelConfig">{{ t('plugins.addChannelConfig') }}</n-button>
          </n-space>
        </n-tab-pane>
      </n-tabs>
      <template #action>
        <n-space justify="end">
          <n-button @click="configModalShow = false">{{ t('common.cancel') }}</n-button>
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
  NForm, NFormItem, NSelect, NInputNumber, NSwitch, NTabs, NTabPane,
} from 'naive-ui'
import { CloudUploadOutline as UploadIcon, AppsOutline as StoreIcon } from '@vicons/ionicons5'
import { pluginApi, type PluginItem, type ChannelPluginConfig } from '../api/plugin'

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
const advancedMode = ref(false)
const channelConfigs = ref<ChannelPluginConfig[]>([])
const newChannelId = ref<number | null>(null)

// 动态表单相关
interface SchemaField {
  key: string
  type: string
  title?: string
  description?: string
  enum?: string[]
  minimum?: number
  maximum?: number
  default?: any
}
const configSchemaFields = ref<SchemaField[]>([])
const configFormValues = reactive<Record<string, any>>({})

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

/** 解析 JSON Schema，提取顶层字段定义 */
function parseConfigSchema(schemaStr: string): SchemaField[] {
  if (!schemaStr) return []
  try {
    const schema = JSON.parse(schemaStr)
    if (schema.type !== 'object' || !schema.properties) return []
    const fields: SchemaField[] = []
    for (const [key, prop] of Object.entries(schema.properties as Record<string, any>)) {
      fields.push({
        key,
        type: prop.type || 'string',
        title: prop.title || key,
        description: prop.description || '',
        enum: prop.enum || undefined,
        minimum: prop.minimum,
        maximum: prop.maximum,
        default: prop.default,
      })
    }
    return fields
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
  advancedMode.value = false

  // 解析当前配置值
  let currentConfig: Record<string, any> = {}
  try {
    currentConfig = JSON.parse(plugin.config || '{}')
  } catch {}
  configText.value = JSON.stringify(currentConfig, null, 2)

  // 解析 config_schema，构建动态表单
  const fields = parseConfigSchema(plugin.config_schema || '')
  configSchemaFields.value = fields

  // 填充表单值：优先用当前配置，其次用 schema 默认值
  for (const key of Object.keys(configFormValues)) {
    delete configFormValues[key]
  }
  for (const field of fields) {
    if (field.key in currentConfig) {
      configFormValues[field.key] = currentConfig[field.key]
    } else if (field.default !== undefined) {
      configFormValues[field.key] = field.default
    } else {
      configFormValues[field.key] = field.type === 'boolean' ? false : field.type === 'number' || field.type === 'integer' ? null : ''
    }
  }

  configModalShow.value = true

  // 加载渠道级配置
  fetchChannelConfigs(plugin.id)
}

function toggleAdvancedMode() {
  advancedMode.value = !advancedMode.value
  if (advancedMode.value) {
    // 从表单值同步到 JSON 文本
    configText.value = JSON.stringify(configFormValues, null, 2)
  }
}

async function saveConfig() {
  configSaving.value = true
  try {
    let finalConfig: string
    if (configSchemaFields.value.length > 0 && !advancedMode.value) {
      // 从动态表单收集值
      const values: Record<string, any> = {}
      for (const field of configSchemaFields.value) {
        values[field.key] = configFormValues[field.key]
      }
      finalConfig = JSON.stringify(values)
    } else {
      // 从 JSON 文本
      JSON.parse(configText.value) // validate
      finalConfig = configText.value
    }
    await pluginApi.updateConfig(configPluginId.value, finalConfig)
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

async function fetchChannelConfigs(pluginId: number) {
  try {
    const { data } = await pluginApi.listChannelConfigs(pluginId)
    channelConfigs.value = data?.data || []
  } catch {
    channelConfigs.value = []
  }
}

async function saveChannelConfig(cc: ChannelPluginConfig) {
  try {
    JSON.parse(cc.config) // validate
    await pluginApi.setChannelConfig(cc.plugin_id, cc.channel_id, cc.config)
    message.success(t('common.saveSuccess'))
    await fetchChannelConfigs(cc.plugin_id)
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      message.error('Invalid JSON')
    } else {
      message.error(e?.response?.data?.error?.message || 'Error')
    }
  }
}

async function deleteChannelConfig(cc: ChannelPluginConfig) {
  dialog.error({
    title: t('common.delete'),
    content: t('plugins.deleteChannelConfigConfirm'),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await pluginApi.deleteChannelConfig(cc.plugin_id, cc.channel_id)
        message.success(t('common.deleteSuccess'))
        await fetchChannelConfigs(cc.plugin_id)
      } catch (e: any) {
        message.error(e?.response?.data?.error?.message || 'Error')
      }
    },
  })
}

function addChannelConfig() {
  if (!newChannelId.value) {
    message.warning(t('plugins.channelIdRequired'))
    return
  }
  // 检查是否已存在
  if (channelConfigs.value.some(cc => cc.channel_id === newChannelId.value)) {
    message.warning(t('plugins.channelConfigExists'))
    return
  }
  channelConfigs.value.push({
    id: 0,
    channel_id: newChannelId.value!,
    plugin_id: configPluginId.value,
    config: '{}',
    created_at: '',
    updated_at: '',
  })
  newChannelId.value = null
}

onMounted(fetchPlugins)
</script>
