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
        <!-- 顶部操作栏 -->
        <n-space align="center">
          <n-input v-model:value="searchText" :placeholder="t('channels.searchPlaceholder')" clearable style="width: 280px" @keyup.enter="loadChannels">
            <template #prefix>🔍</template>
          </n-input>
          <n-button @click="loadChannels">{{ t('common.search') }}</n-button>
          <n-select v-model:value="filterType" :options="channelTypeOptions" :placeholder="t('channels.type')" clearable style="width: 150px" @update:value="loadChannels" />
          <n-select v-model:value="sortBy" :options="sortOptions" :placeholder="t('channels.sortBy')" style="width: 140px" @update:value="loadChannels" />
        </n-space>

        <n-data-table
          :columns="columns"
          :data="channels"
          :loading="loading"
          :pagination="pagination"
          remote
          :row-props="rowProps"
          @update:page="handlePageChange"
        />
      </n-space>
    </n-card>

    <!-- 详情视图 -->
    <n-card v-else :title="selectedChannel.name">
      <template #header-extra>
        <n-button @click="selectedChannel = null">{{ t('common.back') }}</n-button>
      </template>
      <n-tabs v-model:value="activeDetailTab" type="line" animated>
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
            <div style="display: flex; align-items: center; justify-content: space-between">
              <n-button type="primary" @click="showModelModal = true">{{ t('channels.fetchModels') }}</n-button>
              <n-space align="center" size="small">
                <n-text depth="3" style="font-size: 12px; white-space: nowrap">{{ t('channels.testModel') }}:</n-text>
                <n-input v-model:value="editForm.test_model" :placeholder="t('channels.testModelPlaceholder')" size="small" style="width: 200px" />
                <n-button size="small" @click="handleSaveTestModel">{{ t('common.save') }}</n-button>
              </n-space>
            </div>
            <div v-if="upstreamModels.length > 0" class="model-tag-area">
              <n-text depth="3" style="font-size: 13px; margin-bottom: 8px; display: block">{{ t('channels.upstreamModels') }}（{{ upstreamModels.length }}）</n-text>
              <n-space size="small">
                <n-tag v-for="name in upstreamModels" :key="name" size="small" @click="copyModelName(name)" style="cursor: pointer; font-family: 'Menlo', 'Consolas', monospace" :title="t('channels.clickToCopyModel')">{{ name }}</n-tag>
              </n-space>
            </div>
            <n-empty v-else :description="t('channels.noModelsConfigured')" style="padding: 20px 0" />
            <div v-if="modelMappings.length > 0" style="margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--n-border-color, rgba(255,255,255,0.1))">
              <n-text depth="3" style="font-size: 13px; margin-bottom: 8px; display: block">{{ t('channels.modelMapping') }}（{{ modelMappings.length }}）</n-text>
              <div v-for="m in modelMappings" :key="m.display_model_name" style="display: flex; align-items: center; gap: 8px; margin-bottom: 6px">
                <n-tag size="small" type="info" @click="copyText(m.display_model_name)" style="cursor: pointer" :title="t('channels.clickToCopyModel')">{{ m.display_model_name }}</n-tag>
                <span style="color: var(--text-tertiary)">→</span>
                <n-tag size="small" @click="copyText(m.actual_model_name)" style="cursor: pointer" :title="t('channels.clickToCopyModel')">{{ m.actual_model_name }}</n-tag>
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
          <n-select v-model:value="createForm.type" :options="channelTypeOptions" @update:value="onChannelTypeChange" />
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
    <ModelSelectModal v-model:show="showModelModal" :channel-id="selectedChannel?.id ?? 0" :channel-name="selectedChannel?.name ?? ''" :existing-models="channelModels" @save="handleModelSave" />

    <!-- 批量测试弹窗 -->
    <n-modal v-model:show="showBatchTest" preset="card" style="width: 700px">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between; width: 100%; padding-right: 32px">
          <span>{{ t('channels.batchTest') }} - {{ batchTestChannelName }}</span>
          <n-button type="primary" size="small" :loading="batchTesting" @click="handleBatchTest" :disabled="batchTestModels.length === 0">{{ t('channels.startBatchTest') }}</n-button>
        </div>
      </template>
      <n-space vertical size="small">
        <template v-if="batchTestEnabledModels.length > 0">
          <n-space align="center">
            <n-checkbox :checked="batchTestModels.length === batchTestEnabledModels.length && batchTestEnabledModels.length > 0" :indeterminate="batchTestModels.length > 0 && batchTestModels.length < batchTestEnabledModels.length" @update:checked="toggleAllBatchTest" />
            <n-text depth="3" style="font-size: 12px">{{ t('channels.selectAll') }} ({{ batchTestModels.length }}/{{ batchTestEnabledModels.length }})</n-text>
          </n-space>
          <div style="max-height: 200px; overflow-y: auto; border: 1px solid var(--n-border-color, rgba(255,255,255,0.1)); border-radius: 6px; padding: 8px">
            <n-checkbox-group v-model:value="batchTestModels">
              <n-space vertical :size="4">
                <n-checkbox v-for="m in batchTestPagedModels" :key="m.actual_model_name" :value="m.actual_model_name" :label="m.display_model_name === m.actual_model_name ? m.display_model_name : `${m.display_model_name} → ${m.actual_model_name}`" />
              </n-space>
            </n-checkbox-group>
          </div>
          <n-space v-if="batchTestEnabledModels.length > batchTestPageSize" justify="center" style="margin-top: 4px">
            <n-button size="tiny" :disabled="batchTestPage <= 1" @click="batchTestPage--">‹</n-button>
            <n-text depth="3" style="font-size: 12px; line-height: 24px">{{ batchTestPage }} / {{ Math.ceil(batchTestEnabledModels.length / batchTestPageSize) }}</n-text>
            <n-button size="tiny" :disabled="batchTestPage >= Math.ceil(batchTestEnabledModels.length / batchTestPageSize)" @click="batchTestPage++">›</n-button>
          </n-space>
        </template>
        <n-empty v-else :description="t('channels.noModelsConfigured')" style="padding: 20px 0" />
        <div v-if="batchTestResults.length > 0" style="margin-top: 8px">
          <n-data-table :columns="batchTestResultColumns" :data="batchTestResults" size="small" :pagination="false" />
        </div>
      </n-space>
    </n-modal>

    <!-- 上游更新弹窗 -->
    <n-modal v-model:show="showUpstreamUpdate" preset="card" style="width: 600px" :title="t('channels.upstreamUpdate')">
      <n-space vertical>
        <div v-if="fetchingUpstream" style="display: flex; align-items: center; gap: 8px; padding: 20px 0; justify-content: center">
          <n-spin :size="20" />
          <n-text depth="3">{{ t('channels.checkingUpstream') }}</n-text>
        </div>
        <template v-else>
          <div v-if="upstreamRemovedModels.length > 0">
            <n-alert type="warning" style="margin-bottom: 12px">{{ t('channels.upstreamRemovedTip', { count: upstreamRemovedModels.length }) }}</n-alert>
            <n-space vertical>
              <div v-for="m in upstreamRemovedModels" :key="m.display_model_name" style="display: flex; align-items: center; gap: 8px; padding: 6px 10px; background: rgba(255,80,80,0.08); border-radius: 4px">
                <n-tag type="error" size="small" style="text-decoration: line-through">{{ m.display_model_name }}</n-tag>
                <span style="color: var(--text-tertiary); font-size: 12px">→ {{ m.actual_model_name }}</span>
              </div>
            </n-space>
            <n-space style="margin-top: 12px">
              <n-button type="error" @click="handleRemoveUpstreamRemoved">{{ t('channels.removeRemovedModels') }}</n-button>
              <n-button @click="goToModelConfig">{{ t('channels.goToModelConfig') }}</n-button>
            </n-space>
          </div>
          <n-text v-else-if="upstreamChecked" depth="3">{{ t('channels.noRemovedModels') }}</n-text>
        </template>
      </n-space>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, h, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, useDialog, NButton, NSpace, NTag, NInput, NAlert, NInputNumber, NTooltip, NDropdown, NCheckbox, NCheckboxGroup, NSpin } from 'naive-ui'
import { channelApi, type ChannelListItem, type ChannelModel, type BatchTestResultItem } from '../api/channel'
import { accountApi, type Account } from '../api/account'
import ModelSelectModal from '../components/ModelSelectModal.vue'

// 渠道类型图标（SVG 文件引用）
import iconOpenai from '../assets/icons/channel/openai.svg'
import iconOpenaiResponse from '../assets/icons/channel/openai-response.svg'
import iconAnthropic from '../assets/icons/channel/anthropic.svg'
import iconGemini from '../assets/icons/channel/gemini.svg'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

// ========== 列表相关 ==========
const loading = ref(false)
const channels = ref<ChannelListItem[]>([])
const selectedChannel = ref<ChannelListItem | null>(null)
const channelModels = ref<ChannelModel[]>([])
const searchText = ref('')
const filterType = ref<string | null>(null)
const sortBy = ref('weight')
const activeDetailTab = ref('info')

const pagination = reactive({ page: 1, pageSize: 20, itemCount: 0 })

const channelTypeOptions = ref<{ label: string; value: string }[]>([])
// 保存渠道类型完整信息，用于自动填充 base_url
const channelTypeMap = ref<Record<string, { base_url?: string; is_plugin?: boolean }>>({})

async function loadChannelTypes() {
  try {
    const { data } = await channelApi.listChannelTypes()
    const types = data?.data || []
    channelTypeOptions.value = types.map((t: any) => ({
      label: t.is_plugin ? `${t.name} (Plugin)` : t.name,
      value: t.type,
    }))
    // 保存 base_url 映射
    const map: Record<string, { base_url?: string; is_plugin?: boolean }> = {}
    for (const t of types) {
      map[t.type] = { base_url: t.base_url || '', is_plugin: t.is_plugin }
    }
    channelTypeMap.value = map
  } catch {
    // fallback 到内置类型
    channelTypeOptions.value = [
      { label: 'OpenAI', value: 'openai' },
      { label: 'Anthropic', value: 'anthropic' },
      { label: 'Gemini', value: 'gemini' },
    ]
  }
}

// 选择渠道类型时，自动填充 base_url（仅当 base_url 为空时）
function onChannelTypeChange(type: string) {
  if (!createForm.base_url && channelTypeMap.value[type]?.base_url) {
    createForm.base_url = channelTypeMap.value[type].base_url!
  }
}

const sortOptions = [
  { label: t('channels.sortByWeight'), value: 'weight' },
  { label: 'ID', value: 'id' },
  { label: t('channels.sortByLatency'), value: 'latency' },
]

const defaultBaseURLs: Record<string, string> = {
  openai: 'https://api.openai.com',
  'openai-response': 'https://api.openai.com',
  anthropic: 'https://api.anthropic.com',
  gemini: 'https://generativelanguage.googleapis.com',
}

// 类型图标映射（SVG 文件路径）
const typeIcons: Record<string, string> = {
  openai: iconOpenai,
  'openai-response': iconOpenaiResponse,
  anthropic: iconAnthropic,
  gemini: iconGemini,
}

const typeLabels: Record<string, string> = {
  openai: 'OpenAI',
  'openai-response': 'OpenAI Response',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
}

// ========== 表格列定义 ==========
const columns = computed(() => [
  { title: 'ID', key: 'id', width: 70 },
  {
    title: t('channels.name'), key: 'name', width: 180,
    render: (row: ChannelListItem) => {
      const children = [h('span', row.name)]
      if (row.total_account_count > 1) {
        children.push(
          h(NTooltip, null, {
            trigger: () => h('span', { style: 'margin-left: 6px; cursor: help; font-size: 14px; opacity: 0.6' }, '👥'),
            default: () => t('channels.multiAccount'),
          })
        )
      }
      return h('div', { style: 'display: flex; align-items: center' }, children)
    },
  },
  {
    title: t('channels.type'), key: 'type', width: 200,
    render: (row: ChannelListItem) => h('div', { style: 'display: flex; align-items: center; gap: 6px' }, [
      h('img', { src: typeIcons[row.type] || '', style: 'width: 18px; height: 18px; flex-shrink: 0', alt: row.type }),
      h('span', { style: 'width: 8px; height: 8px; border-radius: 50%; background: #f0c040; display: inline-block; flex-shrink: 0' }),
      h('span', { style: 'color: #f0c040' }, typeLabels[row.type] || row.type),
    ]),
  },
  {
    title: t('common.status'), key: 'status', width: 160,
    render: (row: ChannelListItem) => {
      const statusTag = h(NTag, {
        type: row.status === 'active' ? 'success' : 'error',
        size: 'small',
      }, () => row.status === 'active' ? t('common.active') : t('common.disabled'))

      const active = row.active_account_count
      const total = row.total_account_count
      let ratioType: 'success' | 'warning' | 'error' = 'success'
      if (active === 0) ratioType = 'error'
      else if (active < total) ratioType = 'warning'
      const ratioTag = h(NTag, { type: ratioType, size: 'small', bordered: false }, () => `(${active}/${total})`)

      return h(NSpace, { size: 4, align: 'center' }, () => [statusTag, ratioTag])
    },
  },
  {
    title: t('channels.group'), key: 'groups', width: 150,
    render: (row: ChannelListItem) => {
      if (!row.groups || row.groups.length === 0) return h('span', { style: 'color: var(--text-tertiary)' }, '-')
      return h(NSpace, { size: 4 }, () => row.groups.map(g => h(NTag, { size: 'small', round: true, type: 'info', bordered: false }, () => g.name)))
    },
  },
  {
    title: t('common.weight'), key: 'weight', width: 120,
    render: (row: ChannelListItem) => {
      const currentWeight = editingWeightMap[row.id] ?? row.weight
      // 编辑模式：点击数字时显示输入框（带左右加减按钮）
      if (editingWeightRowId.value === row.id) {
        return h('div', {
          class: 'agw-weight-spinner',
          style: 'display: inline-flex; align-items: center; height: 28px; border-radius: 6px; background: rgba(255,255,255,0.06)',
        }, [
          h('button', {
            class: 'agw-weight-btn',
            style: 'display: flex; align-items: center; justify-content: center; width: 24px; height: 28px; border: none; background: transparent; cursor: pointer; opacity: 0.6; transition: opacity 0.15s, color 0.15s; color: rgba(255,255,255,0.5); border-radius: 6px 0 0 6px; font-size: 16px; line-height: 1; padding: 0',
            onClick: (e: Event) => { e.stopPropagation(); adjustWeight(row, -1) },
            onMouseenter: (e: MouseEvent) => { (e.target as HTMLElement).style.opacity = '1'; (e.target as HTMLElement).style.color = 'var(--n-text-color, #fff)'; (e.target as HTMLElement).style.background = 'rgba(255,255,255,0.1)' },
            onMouseleave: (e: MouseEvent) => { (e.target as HTMLElement).style.opacity = '0.6'; (e.target as HTMLElement).style.color = 'rgba(255,255,255,0.5)'; (e.target as HTMLElement).style.background = 'transparent' },
          }, '−'),
          h('input', {
            value: currentWeight,
            type: 'text',
            inputmode: 'numeric',
            pattern: '[0-9]*',
            autofocus: true,
            style: 'width: 44px; height: 28px; text-align: center; font-family: monospace; font-size: 13px; background: transparent; color: var(--n-text-color); border: none; border-bottom: 1px solid var(--n-primary-color, #00d2ff); outline: none; padding: 0; -moz-appearance: textfield',
            onFocus: (e: FocusEvent) => { (e.target as HTMLInputElement).select() },
            onBlur: (e: FocusEvent) => { finishEditWeight(row, (e.target as HTMLInputElement).value) },
            onKeyup: (e: KeyboardEvent) => {
              if (e.key === 'Enter') (e.target as HTMLInputElement).blur()
              if (e.key === 'Escape') { editingWeightRowId.value = null; delete editingWeightMap[row.id] }
            },
          }),
          h('button', {
            class: 'agw-weight-btn',
            style: 'display: flex; align-items: center; justify-content: center; width: 24px; height: 28px; border: none; background: transparent; cursor: pointer; opacity: 0.6; transition: opacity 0.15s, color 0.15s; color: rgba(255,255,255,0.5); border-radius: 0 6px 6px 0; font-size: 16px; line-height: 1; padding: 0',
            onClick: (e: Event) => { e.stopPropagation(); adjustWeight(row, 1) },
            onMouseenter: (e: MouseEvent) => { (e.target as HTMLElement).style.opacity = '1'; (e.target as HTMLElement).style.color = 'var(--n-text-color, #fff)'; (e.target as HTMLElement).style.background = 'rgba(255,255,255,0.1)' },
            onMouseleave: (e: MouseEvent) => { (e.target as HTMLElement).style.opacity = '0.6'; (e.target as HTMLElement).style.color = 'rgba(255,255,255,0.5)'; (e.target as HTMLElement).style.background = 'transparent' },
          }, '+'),
        ])
      }
      // 正常模式：悬停显示左右加减按钮
      return h('div', {
        class: 'agw-weight-spinner',
        style: 'display: inline-flex; align-items: center; height: 28px; border-radius: 6px; transition: background 0.15s; cursor: default',
        onMouseenter: (e: MouseEvent) => {
          const el = e.currentTarget as HTMLElement
          el.style.background = 'rgba(255,255,255,0.06)'
          el.querySelectorAll('.agw-weight-btn').forEach(b => { (b as HTMLElement).style.opacity = '0.6' })
        },
        onMouseleave: (e: MouseEvent) => {
          const el = e.currentTarget as HTMLElement
          el.style.background = 'transparent'
          el.querySelectorAll('.agw-weight-btn').forEach(b => { (b as HTMLElement).style.opacity = '0' })
        },
      }, [
        h('button', {
          class: 'agw-weight-btn',
          style: 'display: flex; align-items: center; justify-content: center; width: 24px; height: 28px; border: none; background: transparent; cursor: pointer; opacity: 0; transition: opacity 0.15s, color 0.15s, background 0.15s; color: rgba(255,255,255,0.5); border-radius: 6px 0 0 6px; font-size: 16px; line-height: 1; padding: 0',
          onClick: (e: Event) => { e.stopPropagation(); adjustWeight(row, -1) },
          onMouseenter: (e: MouseEvent) => { (e.target as HTMLElement).style.opacity = '1'; (e.target as HTMLElement).style.color = 'var(--n-text-color, #fff)'; (e.target as HTMLElement).style.background = 'rgba(255,255,255,0.1)' },
          onMouseleave: (e: MouseEvent) => { (e.target as HTMLElement).style.opacity = '0.6'; (e.target as HTMLElement).style.color = 'rgba(255,255,255,0.5)'; (e.target as HTMLElement).style.background = 'transparent' },
        }, '−'),
        h('span', {
          class: 'agw-weight-value',
          style: 'display: flex; align-items: center; justify-content: center; min-width: 28px; height: 28px; font-family: monospace; font-size: 13px; padding: 0 2px; user-select: none; cursor: pointer; transition: color 0.15s',
          onClick: (e: Event) => { e.stopPropagation(); startEditWeight(row) },
          onMouseenter: (e: MouseEvent) => { (e.target as HTMLElement).style.color = 'var(--n-primary-color, #00d2ff)' },
          onMouseleave: (e: MouseEvent) => { (e.target as HTMLElement).style.color = '' },
        }, String(currentWeight)),
        h('button', {
          class: 'agw-weight-btn',
          style: 'display: flex; align-items: center; justify-content: center; width: 24px; height: 28px; border: none; background: transparent; cursor: pointer; opacity: 0; transition: opacity 0.15s, color 0.15s, background 0.15s; color: rgba(255,255,255,0.5); border-radius: 0 6px 6px 0; font-size: 16px; line-height: 1; padding: 0',
          onClick: (e: Event) => { e.stopPropagation(); adjustWeight(row, 1) },
          onMouseenter: (e: MouseEvent) => { (e.target as HTMLElement).style.opacity = '1'; (e.target as HTMLElement).style.color = 'var(--n-text-color, #fff)'; (e.target as HTMLElement).style.background = 'rgba(255,255,255,0.1)' },
          onMouseleave: (e: MouseEvent) => { (e.target as HTMLElement).style.opacity = '0.6'; (e.target as HTMLElement).style.color = 'rgba(255,255,255,0.5)'; (e.target as HTMLElement).style.background = 'transparent' },
        }, '+'),
      ])
    },
  },
  {
    title: t('channels.responseTime'), key: 'latency', width: 120,
    render: (row: ChannelListItem) => {
      if (!row.last_test_latency || row.last_test_latency === 0) return h('span', { style: 'color: var(--text-tertiary)' }, t('channels.notTested'))
      const ms = row.last_test_latency
      const display = ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms}ms`
      return h('span', { style: { color: latencyColor(ms) } }, display)
    },
  },
  {
    title: t('channels.lastTested'), key: 'last_tested', width: 120,
    render: (row: ChannelListItem) => {
      if (!row.last_tested_at) return h('span', { style: 'color: var(--text-tertiary)' }, t('channels.notTested'))
      return h('span', null, formatTimeAgo(row.last_tested_at))
    },
  },
  {
    title: t('common.actions'), key: 'actions', width: 150, fixed: 'right',
    render: (row: ChannelListItem) => h(NSpace, { size: 4, align: 'center' }, () => [
      h(NTooltip, null, {
        trigger: () => h(NButton, { size: 'small', quaternary: true,      loading: testingChannelId.value !== null && testingChannelId.value === row.id, onClick: () => handleTestChannel(row)}, { icon: () => h('span', '⚡') }),
        default: () => t('channels.testAvailability'),
      }),
      h(NTooltip, null, {
        trigger: () => h(NButton, {
          size: 'small', quaternary: true,
          type: row.status === 'active' ? 'error' : 'success',
          onClick: () => handleToggleChannel(row),
        }, { icon: () => h('span', row.status === 'active' ? '⏸' : '▶') }),
        default: () => row.status === 'active' ? t('common.disable') : t('common.enable'),
      }),
      h(NDropdown, {
        options: getMoreOptions(row),
        onSelect: (key: string) => handleMoreAction(key, row),
      }, {
        default: () => h(NButton, { size: 'small', quaternary: true }, { icon: () => h('span', '⋯') }),
      }),
    ]),
  },
])

// ========== 更多操作 ==========
function getMoreOptions(_row: ChannelListItem) {
  return [
    { label: t('channels.editChannel'), key: 'edit' },
    { label: t('channels.batchTest'), key: 'batch-test' },
    { label: t('channels.fetchModels'), key: 'fetch-models' },
    { label: t('channels.upstreamUpdate'), key: 'upstream-update' },
    { label: t('channels.copyChannel'), key: 'copy' },
    { label: t('channels.manageKeys'), key: 'manage-keys' },
    { type: 'divider', key: 'd1' },
    { label: t('common.delete'), key: 'delete', props: { style: 'color: #ff6b6b' } },
  ]
}

function handleMoreAction(key: string, row: ChannelListItem) {
  switch (key) {
    case 'edit':
      selectChannel(row, 'info')
      break
    case 'batch-test':
      openBatchTest(row)
      break
    case 'fetch-models':
      selectChannel(row, 'models').then(() => { showModelModal.value = true })
      break
    case 'upstream-update':
      openUpstreamUpdate(row)
      break
    case 'copy':
      handleCopyChannel(row)
      break
    case 'manage-keys':
      selectChannel(row, 'accounts')
      break
    case 'delete':
      handleDeleteChannel(row)
      break
  }
}

// ========== 列表页直接操作的辅助函数 ==========
async function openBatchTest(row: ChannelListItem) {
  // 前置验证：检查 URL
  if (!row.base_url) {
    message.warning(t('channels.noBaseUrl'), { duration: 5000 })
    return
  }
  // 前置验证：检查账号
  if (!row.active_account_count || row.active_account_count === 0) {
    message.warning(t('channels.noActiveAccount'), { duration: 5000 })
    return
  }
  batchTestChannelId.value = row.id
  batchTestChannelName.value = row.name
  batchTestModels.value = []
  batchTestResults.value = []
  batchTestPage.value = 1
  try {
    const res = await channelApi.getModelsByChannel(row.id)
    channelModels.value = res.data.data || []
  } catch { channelModels.value = [] }
  if (channelModels.value.filter((m: any) => m.status === 'enabled').length === 0) {
    message.warning(t('channels.noModelsConfigured'), { duration: 5000 })
    return
  }
  showBatchTest.value = true
}

async function openUpstreamUpdate(row: ChannelListItem) {
  if (!row.base_url) {
    message.warning(t('channels.noBaseUrl'), { duration: 5000 })
    return
  }
  upstreamChannelId.value = row.id
  upstreamChecked.value = false
  upstreamRemovedModels.value = []
  showUpstreamUpdate.value = true
  fetchingUpstream.value = true
  try {
    const [upstreamRes, localRes] = await Promise.all([
      channelApi.fetchModels(row.id),
      channelApi.getModelsByChannel(row.id),
    ])
    channelModels.value = localRes.data.data || []
    const upstreamIds = new Set((upstreamRes.data.data || []).map((m: any) => m.id))
    const localModels = localRes.data.data || []
    upstreamRemovedModels.value = localModels.filter(m => m.status === 'enabled' && !upstreamIds.has(m.actual_model_name))
    upstreamChecked.value = true
  } catch { message.error(t('common.operationFailed')) }
  finally { fetchingUpstream.value = false }
}

// ========== 弹窗状态 ==========
const showCreateModal = ref(false)
const showAddAccount = ref(false)
const showModelModal = ref(false)
const showBatchTest = ref(false)
const showUpstreamUpdate = ref(false)

const testingChannelId = ref<number | null>(null)

// 创建/编辑/账号表单
const createForm = reactive({ name: '', type: 'openai', base_url: '', api_key: '' })
const editForm = reactive({ name: '', base_url: '', weight: 0, max_rpm: 0, max_tpm: 0, max_daily_requests: 0, test_model: '' })
const accountForm = reactive({ api_key: '', remark: '' })

// 测试连接
const testingConnection = ref(false)
const testConnectionResult = ref<boolean | null>(null)
const testConnectionError = ref('')

// 账号编辑
const editingRemarkId = ref<number | null>(null)
const editingRemark = ref('')
const editingPriorityMap = reactive<Record<number, number>>({})
const editingWeightMap = reactive<Record<number, number>>({})
const editingWeightRowId = ref<number | null>(null)

// 批量测试
const batchTesting = ref(false)
const batchTestModels = ref<string[]>([])
const batchTestResults = ref<BatchTestResultItem[]>([])
const batchTestChannelId = ref<number>(0)
const batchTestChannelName = ref('')
const batchTestPage = ref(1)
const batchTestPageSize = 30
const batchTestEnabledModels = computed(() => channelModels.value.filter(m => m.status === 'enabled'))
const batchTestPagedModels = computed(() => {
  const start = (batchTestPage.value - 1) * batchTestPageSize
  return batchTestEnabledModels.value.slice(start, start + batchTestPageSize)
})

const batchTestResultColumns = computed(() => [
  { title: t('channels.testModel'), key: 'model' },
  {
    title: t('channels.testLatency'), key: 'latency', width: 100,
    render: (row: BatchTestResultItem) => h('span', { style: { color: latencyColor(row.latency) } }, `${row.latency}ms`),
  },
  {
    title: t('common.status'), key: 'success', width: 80,
    render: (row: BatchTestResultItem) => {
      if (row.testing) return h(NSpin, { size: 18 })
      return h(NTag, { type: row.success ? 'success' : 'error', size: 'small' }, () => row.success ? '✓' : `✗ ${row.status || ''}`)
    },
  },
  {
    title: t('channels.testError'), key: 'error',
    render: (row: BatchTestResultItem) => row.error ? h('span', { style: 'color: #ff6b6b; font-size: 12px; word-break: break-all' }, row.error.substring(0, 100)) : '-',
  },
])

function toggleAllBatchTest(checked: boolean) {
  batchTestModels.value = checked ? batchTestEnabledModels.value.map(m => m.actual_model_name) : []
}

// 上游更新
const fetchingUpstream = ref(false)
const upstreamChecked = ref(false)
const upstreamRemovedModels = ref<ChannelModel[]>([])
const upstreamChannelId = ref<number>(0)

// ========== 计算属性 ==========
const upstreamModels = computed(() => {
  const names = new Set<string>()
  channelModels.value.filter(m => m.status === 'enabled').forEach(m => names.add(m.actual_model_name))
  return Array.from(names)
})

const modelMappings = computed(() =>
  channelModels.value.filter(m => m.display_model_name !== m.actual_model_name && m.status === 'enabled')
)

const accounts = ref<Account[]>([])

// ========== 工具函数 ==========
function latencyColor(ms: number): string {
  if (ms < 500) return '#52c41a'
  if (ms <= 2000) return '#f0c040'
  return '#ff4d4f'
}

function formatTimeAgo(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m ago`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour}h ago`
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay < 30) return `${diffDay}d ago`
  return date.toLocaleDateString()
}

function rowProps(_row: ChannelListItem) {
  return { style: 'cursor: pointer', onClick: () => {} }
}

// ========== 数据加载 ==========
async function loadChannels() {
  loading.value = true
  try {
    const res = await channelApi.list({
      page: pagination.page,
      page_size: pagination.pageSize,
      type: filterType.value || undefined,
      search: searchText.value || undefined,
      sort_by: sortBy.value || undefined,
      sort_order: sortBy.value === 'id' ? 'asc' : 'desc',
    })
    channels.value = res.data.data
    pagination.itemCount = res.data.total
  } finally {
    loading.value = false
  }
}

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

async function selectChannel(ch: ChannelListItem, tab?: string) {
  selectedChannel.value = ch
  if (tab) activeDetailTab.value = tab
  editForm.name = ch.name
  editForm.base_url = ch.base_url
  editForm.weight = ch.weight
  editForm.max_rpm = ch.max_rpm ?? 0
  editForm.max_tpm = ch.max_tpm ?? 0
  editForm.max_daily_requests = ch.max_daily_requests ?? 0
  editForm.test_model = ch.test_model ?? ''
  await loadAccounts(ch.id)
  try {
    const res = await channelApi.getModelsByChannel(ch.id)
    channelModels.value = res.data.data || []
  } catch {
    channelModels.value = []
  }
}

// ========== 渠道操作 ==========
async function handleTestChannel(row: ChannelListItem) {
  // 前置验证
  if (!row.base_url) {
    message.warning(t('channels.noBaseUrl'), { duration: 5000 })
    return
  }
  if (!row.active_account_count || row.active_account_count === 0) {
    message.warning(t('channels.noActiveAccount'), { duration: 5000 })
    return
  }
  testingChannelId.value = row.id
  try {
    const res = await channelApi.testChannel(row.id)
    const result = res.data.data
    if (result.success) {
      message.success(`${t('channels.testSuccess')} ${result.latency}ms`, { duration: 3000 })
    } else {
      message.error(`${t('common.failed')}: ${result.error || t('common.operationFailed')}`, { duration: 5000 })
    }
    loadChannels()
  } catch (err: any) {
    const errMsg = err?.response?.data?.error?.message || err?.response?.data?.error || t('common.operationFailed')
    message.error(errMsg, { duration: 5000 })
  } finally {
    testingChannelId.value = null
  }
}

async function handleToggleChannel(row: ChannelListItem) {
  const newStatus = row.status === 'active' ? 'disabled' : 'active'
  try {
    await channelApi.updateStatus(row.id, newStatus)
    message.success(t('common.success'))
    loadChannels()
  } catch { message.error(t('common.operationFailed')) }
}

function handleDeleteChannel(row: ChannelListItem) {
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

async function handleCopyChannel(row: ChannelListItem) {
  try {
    const res = await channelApi.copyChannel(row.id)
    const newId = res.data.data?.id
    message.success(t('channels.channelCopied'), { duration: 5000 })
    loadChannels()
    // 跳转到新渠道的账号管理
    if (newId) {
      const newItem = channels.value.find(c => c.id === newId)
      if (newItem) {
        selectChannel(newItem, 'accounts')
      }
    }
  } catch { message.error(t('common.operationFailed')) }
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
    const res = await channelApi.testConnection({ type: createForm.type, base_url: createForm.base_url, api_key: createForm.api_key })
    testConnectionResult.value = res.data.success
    if (!res.data.success) testConnectionError.value = res.data.error || t('channels.testConnectionFailed')
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

async function handleSaveTestModel() {
  if (!selectedChannel.value) return
  try {
    await channelApi.updateTestModel(selectedChannel.value.id, editForm.test_model)
    message.success(t('common.success'))
    loadChannels()
  } catch { message.error(t('common.operationFailed')) }
}

async function handleModelSave(models: ChannelModel[]) {
  if (!selectedChannel.value) return
  try {
    await channelApi.saveModels(selectedChannel.value.id, models)
    message.success(t('common.success'))
    const res = await channelApi.getModelsByChannel(selectedChannel.value.id)
    channelModels.value = res.data.data || []
  } catch { message.error(t('common.operationFailed')) }
}

// ========== 批量测试 ==========
async function handleBatchTest() {
  const chId = batchTestChannelId.value || selectedChannel.value?.id
  if (!chId || batchTestModels.value.length === 0) return
  batchTesting.value = true
  batchTestResults.value = []
  try {
    // 逐个测试，每测完一个显示结果
    for (const model of batchTestModels.value) {
      batchTestResults.value = [...batchTestResults.value, { model, success: false, latency: 0, status: 0, error: '', testing: true }]
      const idx = batchTestResults.value.length - 1
      try {
        const res = await channelApi.batchTestModels(chId, [model])
        const result = (res.data.data || [])[0]
        if (result) {
          batchTestResults.value[idx] = { ...result, testing: false }
        }
      } catch (err: any) {
        batchTestResults.value[idx] = { model, success: false, latency: 0, status: 0, error: err?.message || 'Failed', testing: false }
      }
    }
  } finally { batchTesting.value = false }
}

// ========== 上游更新 ==========

async function handleRemoveUpstreamRemoved() {
  const chId = upstreamChannelId.value || selectedChannel.value?.id
  if (!chId) return
  const remaining = channelModels.value.filter(m =>
    m.status !== 'enabled' || !upstreamRemovedModels.value.find(r => r.actual_model_name === m.actual_model_name)
  )
  try {
    await channelApi.saveModels(chId, remaining)
    message.success(t('common.success'))
    const res = await channelApi.getModelsByChannel(chId)
    channelModels.value = res.data.data || []
    upstreamRemovedModels.value = []
    upstreamChecked.value = false
  } catch { message.error(t('common.operationFailed')) }
}

function goToModelConfig() {
  const chId = upstreamChannelId.value || selectedChannel.value?.id
  if (!chId) return
  const ch = channels.value.find(c => c.id === chId)
  if (ch) {
    showUpstreamUpdate.value = false
    selectChannel(ch, 'models')
  }
}

// ========== 账号操作 ==========
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
  if (editingRemarkId.value === null) return
  const newRemark = editingRemark.value.trim()
  editingRemarkId.value = null
  try {
    await accountApi.updateRemark(id, newRemark)
    message.success(t('common.success'))
    if (selectedChannel.value) selectChannel(selectedChannel.value)
  } catch { message.error(t('common.operationFailed')) }
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

function startEditWeight(row: ChannelListItem) {
  editingWeightRowId.value = row.id
  editingWeightMap[row.id] = row.weight
}

async function finishEditWeight(row: ChannelListItem, value: string) {
  const newWeight = Math.max(0, parseInt(value) || 0)
  editingWeightRowId.value = null
  delete editingWeightMap[row.id]
  if (newWeight === row.weight) return // 无变更，直接恢复
  try {
    await channelApi.updateWeight(row.id, newWeight)
    message.success(t('common.success'))
    loadChannels() // 有变更，刷新列表重新排列
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function adjustWeight(row: ChannelListItem, delta: number) {
  const newWeight = Math.max(0, row.weight + delta)
  if (newWeight === row.weight) return
  try {
    await channelApi.updateWeight(row.id, newWeight)
    row.weight = newWeight
    message.success(t('common.success'))
  } catch { message.error(t('common.operationFailed')) }
}

async function copyModelName(name: string) {
  try {
    await navigator.clipboard.writeText(name)
    message.success(t('common.copied'))
  } catch { message.error(t('common.copyFailed')) }
}

function copyText(text: string) { copyModelName(text) }

// 账号表格列
const accountColumns = computed(() => [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('channels.keyMask'), key: 'api_key_mask' },
  {
    title: t('channels.remark'), key: 'remark', width: 180,
    render: (row: Account) => {
      if (editingRemarkId.value === row.id) {
        return h(NInput, {
          value: editingRemark.value, size: 'small', autofocus: true, style: { width: '150px' },
          'onUpdate:value': (val: string) => { editingRemark.value = val },
          onKeyup: (e: KeyboardEvent) => {
            if (e.key === 'Enter') handleSaveRemark(row.id)
            if (e.key === 'Escape') editingRemarkId.value = null
          },
          onBlur: () => handleSaveRemark(row.id),
        })
      }
      return h('span', {
        style: { cursor: 'pointer', borderBottom: '1px dashed var(--text-tertiary)' },
        onDblclick: () => { editingRemarkId.value = row.id; editingRemark.value = row.remark || '' },
      }, row.remark || '-')
    },
  },
  {
    title: t('common.priority'), key: 'priority', width: 130,
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

// 创建时自动填充默认 Base URL
watch(() => createForm.type, (newType) => {
  const defaultURL = defaultBaseURLs[newType]
  if (defaultURL && (!createForm.base_url || Object.values(defaultBaseURLs).includes(createForm.base_url))) {
    createForm.base_url = defaultURL
  }
})

onMounted(() => {
  loadChannelTypes()
  loadChannels()
})
</script>

<style scoped>
</style>
