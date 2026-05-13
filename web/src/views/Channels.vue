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
            <n-form-item :label="t('common.weight')">
              <n-input-number v-model:value="editForm.weight" :min="1" />
              <template #feedback>
                <n-text depth="3" style="font-size: 12px">{{ t('channels.weightTip') }}</n-text>
              </template>
            </n-form-item>
            <n-form-item :label="t('channels.maxRPM')">
              <n-input-number v-model:value="editForm.max_rpm" :min="0" :placeholder="t('channels.noLimit')" />
              <template #feedback>
                <n-text depth="3" style="font-size: 12px">{{ t('channels.maxRPMTip') }}</n-text>
              </template>
            </n-form-item>
            <n-form-item :label="t('channels.maxTPM')">
              <n-input-number v-model:value="editForm.max_tpm" :min="0" :placeholder="t('channels.noLimit')" />
              <template #feedback>
                <n-text depth="3" style="font-size: 12px">{{ t('channels.maxTPMTip') }}</n-text>
              </template>
            </n-form-item>
            <n-form-item :label="t('channels.maxDailyRequests')">
              <n-input-number v-model:value="editForm.max_daily_requests" :min="0" :placeholder="t('channels.noLimit')" />
              <template #feedback>
                <n-text depth="3" style="font-size: 12px">{{ t('channels.maxDailyRequestsTip') }}</n-text>
              </template>
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
          <n-space vertical size="small">
            <!-- 顶部工具栏 -->
            <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px">
              <n-space align="center" size="small">
                <n-button type="primary" size="small" @click="fetchModels" :loading="fetchingModels">
                  {{ t('channels.fetchModels') }}
                </n-button>
                <n-text depth="3" style="font-size: 12px">
                  {{ t('channels.selectedModels') }}：{{ selectedModelIds.size }}
                </n-text>
              </n-space>
              <n-space align="center" size="small">
                <n-text depth="3" style="font-size: 12px; white-space: nowrap">{{ t('channels.testModel') }}:</n-text>
                <n-input v-model:value="editForm.test_model" :placeholder="t('channels.testModelPlaceholder')" size="small" style="width: 180px" />
                <n-button size="small" @click="handleSaveTestModel">{{ t('common.save') }}</n-button>
              </n-space>
            </div>

            <!-- 筛选行 -->
            <div style="display: flex; align-items: center; gap: 12px">
              <n-tabs v-model:value="modelFilter" type="segment" size="small" style="flex: 1">
                <n-tab name="all">{{ t('channels.filterAll') }}</n-tab>
                <n-tab name="new">{{ t('channels.filterNew') }}</n-tab>
                <n-tab name="existing">{{ t('channels.filterExisting') }}</n-tab>
              </n-tabs>
              <n-input v-model:value="modelSearchQuery" :placeholder="t('channels.searchModels')" clearable size="small" style="width: 200px">
                <template #prefix><span style="opacity: 0.5">🔍</span></template>
              </n-input>
            </div>

            <!-- 主体：供应商 + 模型选择 + 已选标签 -->
            <div v-if="fetchingModels" style="display: flex; justify-content: center; padding: 40px">
              <n-spin size="large" />
            </div>
            <template v-else>
              <!-- 无数据 -->
              <n-empty v-if="availableModels.length === 0" :description="t('channels.noModelsTip')" style="padding: 30px 0" />

              <template v-else>
                <!-- 左侧供应商 + 右侧模型 -->
                <div style="display: flex; gap: 0; border: 1px solid var(--n-border-color, rgba(255,255,255,0.1)); border-radius: 6px; overflow: hidden; height: 360px">
                  <!-- 左侧供应商列表 -->
                  <div style="width: 200px; min-width: 200px; flex-shrink: 0; overflow-y: auto; border-right: 1px solid var(--n-border-color, rgba(255,255,255,0.1)); background: rgba(255,255,255,0.02)">
                    <div
                      v-for="group in modelGroups"
                      :key="group.owner"
                      style="display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; cursor: pointer; border-bottom: 1px solid rgba(255,255,255,0.04); transition: background-color 0.15s"
                      :style="{ background: activeModelOwner === group.owner ? 'rgba(0, 210, 255, 0.1)' : '', borderLeft: activeModelOwner === group.owner ? '3px solid #00d2ff' : '3px solid transparent' }"
                      @click="activeModelOwner = group.owner"
                    >
                      <span style="font-size: 13px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; margin-right: 8px">{{ group.owner }}</span>
                      <n-tag size="small" :type="group.selectedCount > 0 ? 'info' : 'default'">{{ group.selectedCount }}/{{ group.models.length }}</n-tag>
                    </div>
                  </div>

                  <!-- 右侧模型列表 -->
                  <div style="flex: 1; height: 360px; overflow-y: auto">
                    <template v-if="activeModelGroup">
                      <div style="display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border-bottom: 1px solid rgba(255,255,255,0.06)">
                        <n-text depth="3" style="font-size: 12px">{{ activeModelGroup.models.length }} {{ t('channels.modelCount') }}</n-text>
                        <n-button text type="info" size="tiny" @click="toggleModelGroupAll(activeModelGroup)">
                          {{ isModelGroupAllSelected(activeModelGroup) ? t('common.deselectAll') : t('common.selectAll') }}
                        </n-button>
                      </div>
                      <div style="display: grid; grid-template-columns: repeat(2, 1fr); gap: 4px 8px; padding: 8px 12px">
                        <div
                          v-for="m in activeModelGroup.models"
                          :key="m.id"
                          style="display: flex; align-items: center; gap: 6px; padding: 5px 8px; border-radius: 4px; cursor: pointer; position: relative; overflow: hidden; transition: background-color 0.15s"
                          :style="{ background: selectedModelIds.has(m.id) ? 'rgba(0, 210, 255, 0.06)' : '' }"
                          @click="toggleModel(m.id, !selectedModelIds.has(m.id))"
                        >
                          <div v-if="selectedModelIds.has(m.id)" style="position: absolute; left: 0; top: 0; bottom: 0; width: 3px; background-color: #00d2ff; border-radius: 3px 0 0 3px"></div>
                          <n-checkbox :checked="selectedModelIds.has(m.id)" @update:checked="(v: boolean) => toggleModel(m.id, v)" @click.stop />
                          <n-tooltip trigger="hover" placement="top">
                            <template #trigger>
                              <span style="font-size: 13px; font-family: 'Menlo', 'Consolas', monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; max-width: 220px">{{ m.id }}</span>
                            </template>
                            {{ m.id }}
                          </n-tooltip>
                        </div>
                      </div>
                    </template>
                    <n-empty v-else :description="t('channels.selectOwner')" style="padding: 40px 0" />
                  </div>
                </div>
              </template>
            </template>

            <!-- 已选模型标签 -->
            <div v-if="selectedModelIds.size > 0" style="padding-top: 8px; border-top: 1px solid var(--n-border-color, rgba(255,255,255,0.1))">
              <n-text depth="3" style="font-size: 13px; margin-bottom: 8px; display: block">{{ t('channels.selectedModels') }}（{{ selectedModelIds.size }}）</n-text>
              <n-space size="small">
                <n-tag v-for="id in selectedModelIds" :key="id" closable size="small" @close="toggleModel(id, false)" @click="copyModelName(id)" style="cursor: pointer" :title="t('channels.clickToCopyModel')">{{ id }}</n-tag>
              </n-space>
            </div>

            <!-- 自定义模型输入 -->
            <div style="display: flex; align-items: center; gap: 8px; padding: 8px 0; border-top: 1px solid var(--n-border-color, rgba(255,255,255,0.1))">
              <n-text depth="3" style="font-size: 13px; white-space: nowrap">{{ t('channels.addCustomModel') }}:</n-text>
              <n-input v-model:value="customModelName" :placeholder="t('channels.customModelPlaceholder')" size="small" clearable @keyup.enter="addCustomModel" style="max-width: 280px" />
              <n-button size="small" type="primary" ghost @click="addCustomModel" :disabled="!customModelName.trim()">{{ t('channels.addCustomModel') }}</n-button>
            </div>

            <!-- 模型映射 -->
            <div style="padding-top: 8px; border-top: 1px solid var(--n-border-color, rgba(255,255,255,0.1))">
              <n-space justify="space-between" align="center" style="margin-bottom: 8px">
                <n-text depth="3" style="font-size: 13px">{{ t('channels.modelMapping') }}</n-text>
                <n-button size="small" @click="addMapping">+ {{ t('channels.addMapping') }}</n-button>
              </n-space>
              <div v-for="(m, idx) in modelMappings" :key="idx" style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px">
                <n-auto-complete v-model:value="m.display" :options="displayNameOptions(m.display)" :placeholder="t('channels.displayName')" size="small" style="flex: 1" />
                <span style="color: var(--text-tertiary)">→</span>
                <n-select v-model:value="m.actual" :options="selectedModelOptions" :placeholder="t('channels.actualName')" size="small" style="flex: 1" filterable />
                <n-button size="small" quaternary type="error" @click="modelMappings.splice(idx, 1)">✕</n-button>
              </div>
            </div>

          <!-- 底部操作 -->
            <div style="display: flex; justify-content: space-between; align-items: center; padding-top: 8px; border-top: 1px solid var(--n-border-color, rgba(255,255,255,0.1))">
              <n-space align="center">
                <n-text depth="3" style="font-size: 12px">{{ t('channels.selectedModels') }}：{{ selectedModelIds.size }}</n-text>
                <n-button text type="info" size="small" @click="toggleModelSelectAll">{{ modelIsAllSelected ? t('common.deselectAll') : t('common.selectAll') }}</n-button>
              </n-space>
              <n-space>
                <n-button type="primary" size="small" @click="handleModelSave" :disabled="selectedModelIds.size === 0">{{ t('channels.saveModels') }}</n-button>
              </n-space>
            </div>
          </n-space>
        </n-tab-pane>

        <!-- 账号管理 -->
        <n-tab-pane name="accounts" :tab="t('channels.accounts')">
          <n-space vertical>
            <n-space>
              <n-button type="primary" @click="showAddAccount = true">{{ t('channels.addAccount') }}</n-button>
              <n-button v-if="hasDisabledAccounts" type="warning" @click="batchRecover" :loading="batchLoading">{{ t('channels.batchRecover') }}</n-button>
              <n-dropdown trigger="click" @select="handleBatchTestAccountSelect" :options="batchTestAccountOptions">
                <n-button :loading="batchTestAccountLoading" type="info">{{ t('channels.batchTest') || '批量测试' }}</n-button>
              </n-dropdown>
            </n-space>
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

    <!-- 上游更新弹窗 -->
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
import { channelApi, type ChannelListItem, type ChannelModel, type BatchTestResultItem, type ModelInfo } from '../api/channel'
import { accountApi, type Account } from '../api/account'


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
      selectChannel(row, 'models')
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
const showBatchTest = ref(false)
const showUpstreamUpdate = ref(false)
const testingIds = ref<number[]>([])
const batchLoading = ref(false)
const batchTestAccountLoading = ref(false)
const batchTestAccountOptions = [
  { label: t('channels.batchTestDisabled'), key: 'disabled' },
  { label: t('channels.batchTestActive'), key: 'active' },
  { label: t('channels.batchTestAll'), key: 'all' },
]
const hasDisabledAccounts = computed(() => accounts.value.some((a: Account) => a.status === 'disabled'))

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

// ========== 模型配置页面整合（原 ModelSelectModal 逻辑）==========
// 状态
const fetchingModels = ref(false)
const availableModels = ref<ModelInfo[]>([])
const selectedModelIds = ref<Set<string>>(new Set())
const modelMappings = ref<{ display: string; actual: string }[]>([])
const customModelName = ref('')
const modelSearchQuery = ref('')
const modelFilter = ref('all')
const activeModelOwner = ref('')

// 已配置模型的名称集合（用 actual_model_name 匹配上游真实模型）
const existingNameSet = computed(() => {
  const enabled = new Set<string>()
  const disabled = new Set<string>()
  for (const m of channelModels.value) {
    if (m.status === 'enabled') enabled.add(m.actual_model_name)
    else disabled.add(m.actual_model_name)
  }
  return { enabled, disabled }
})

interface ModelGroup { owner: string; models: ModelInfo[]; selectedCount: number }

const modelGroups = computed<ModelGroup[]>(() => {
  const q = modelSearchQuery.value.toLowerCase().trim()
  const map = new Map<string, ModelInfo[]>()
  for (const m of availableModels.value) {
    const owner = m.owned_by || 'other'
    if (modelFilter.value === 'new') {
      if (existingNameSet.value.enabled.has(m.id) || existingNameSet.value.disabled.has(m.id)) continue
    } else if (modelFilter.value === 'existing') {
      if (!existingNameSet.value.enabled.has(m.id)) continue
    }
    if (q && !m.id.toLowerCase().includes(q) && !owner.toLowerCase().includes(q)) continue
    if (!map.has(owner)) map.set(owner, [])
    map.get(owner)!.push(m)
  }
  const groups = Array.from(map.entries()).map(([owner, models]) => ({ owner, models, selectedCount: models.filter(m => selectedModelIds.value.has(m.id)).length }))
  if (groups.length > 0 && !groups.find(g => g.owner === activeModelOwner.value)) {
    activeModelOwner.value = groups[0].owner
  }
  return groups
})

const activeModelGroup = computed(() => modelGroups.value.find(g => g.owner === activeModelOwner.value) || null)
const selectedModelOptions = computed(() => Array.from(selectedModelIds.value).map(id => ({ label: id, value: id })))
const modelIsAllSelected = computed(() => availableModels.value.length > 0 && availableModels.value.every(m => selectedModelIds.value.has(m.id)))

// 自定义模型名补全（从 channelModels 映射 + 当前未保存映射 + 跨渠道自定义名）
const customNameOptions = computed(() => {
  const seen = new Set<string>()
  const opts: { label: string; value: string }[] = []
  for (const m of channelModels.value) {
    if (m.display_model_name !== m.actual_model_name && !seen.has(m.display_model_name)) {
      seen.add(m.display_model_name)
      opts.push({ label: m.display_model_name, value: m.display_model_name })
    }
  }
  for (const m of modelMappings.value) {
    if (m.display && !seen.has(m.display)) {
      seen.add(m.display)
      opts.push({ label: m.display, value: m.display })
    }
  }
  // 合并跨渠道自定义模型名
  for (const name of crossChannelCustomNames.value) {
    if (!seen.has(name)) {
      seen.add(name)
      opts.push({ label: name, value: name })
    }
  }
  return opts
})

const crossChannelCustomNames = ref<string[]>([])

async function fetchCrossChannelCustomNames() {
  try {
    const res = await channelApi.getCustomModelNames()
    crossChannelCustomNames.value = res.data.data || []
  } catch { crossChannelCustomNames.value = [] }
}

async function fetchModels() {
  if (!selectedChannel.value?.id) return
  fetchingModels.value = true
  try {
    const res = await channelApi.fetchModels(selectedChannel.value.id, '')
    const raw: ModelInfo[] = res.data.data || []
    const seen = new Set<string>()
    availableModels.value = raw.filter(m => { if (seen.has(m.id)) return false; seen.add(m.id); return true })
    if (availableModels.value.length > 0) {
      activeModelOwner.value = (availableModels.value[0].owned_by || 'other')
    }
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    fetchingModels.value = false
  }
}

function toggleModel(id: string, checked: boolean) {
  const newSet = new Set(selectedModelIds.value)
  if (checked) newSet.add(id); else newSet.delete(id)
  selectedModelIds.value = newSet
}

function isModelGroupAllSelected(group: ModelGroup) {
  return group.models.length > 0 && group.models.every(m => selectedModelIds.value.has(m.id))
}

function toggleModelGroupAll(group: ModelGroup) {
  if (isModelGroupAllSelected(group)) {
    const newSet = new Set(selectedModelIds.value)
    for (const m of group.models) newSet.delete(m.id)
    selectedModelIds.value = newSet
  } else {
    const newSet = new Set(selectedModelIds.value)
    for (const m of group.models) newSet.add(m.id)
    selectedModelIds.value = newSet
  }
}

function toggleModelSelectAll() {
  if (modelIsAllSelected.value) {
    selectedModelIds.value = new Set()
  } else {
    selectedModelIds.value = new Set(availableModels.value.map(m => m.id))
  }
}

function addCustomModel() {
  const name = customModelName.value.trim()
  if (!name) return
  const newSet = new Set(selectedModelIds.value)
  newSet.add(name)
  selectedModelIds.value = newSet
  customModelName.value = ''
  message.success(t('channels.customModelAdded', { name }))
}

function displayNameOptions(input: string) {
  if (!input || input.trim() === '') return []
  const q = input.toLowerCase().trim()
  return customNameOptions.value.filter(o => o.value.toLowerCase().includes(q))
}

function addMapping() {
  modelMappings.value.push({ display: '', actual: '' })
}

// 修改 handleModelSave 为直接保存（不依赖 emit）
function handleModelSave() {
  if (!selectedChannel.value) return
  // 校验映射：映射的目标模型必须在已选模型中
  const invalidMappings = modelMappings.value.filter(m => m.display && m.actual && !selectedModelIds.value.has(m.actual))
  if (invalidMappings.length > 0) {
    const names = invalidMappings.map(m => m.actual).join('、')
    dialog.warning({
      title: t('channels.mappingInvalidTitle'),
      content: t('channels.mappingInvalidContent', { names }),
      positiveText: t('channels.autoFixMapping'),
      negativeText: t('channels.backToEdit'),
      onPositiveClick: () => {
        const newSet = new Set(selectedModelIds.value)
        for (const m of invalidMappings) newSet.add(m.actual)
        selectedModelIds.value = newSet
        doModelSave()
      },
    })
    return
  }
  doModelSave()
}

async function doModelSave() {
  if (!selectedChannel.value) return
  const models: ChannelModel[] = []
  const mappedActuals = new Set(modelMappings.value.filter(m => m.display && m.actual).map(m => m.actual))
  for (const id of selectedModelIds.value) {
    if (!mappedActuals.has(id)) {
      models.push({ channel_id: selectedChannel.value.id, display_model_name: id, actual_model_name: id, status: 'enabled' })
    }
  }
  for (const m of modelMappings.value) {
    if (m.display && m.actual) {
      models.push({ channel_id: selectedChannel.value.id, display_model_name: m.display, actual_model_name: m.actual, status: 'enabled' })
    }
  }
  try {
    await channelApi.saveModels(selectedChannel.value.id, models)
    message.success(t('common.success'))
    const res = await channelApi.getModelsByChannel(selectedChannel.value.id)
    channelModels.value = res.data.data || []
  } catch { message.error(t('common.operationFailed')) }
}

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
    // 初始化模型选择状态
    selectedModelIds.value = new Set(
      channelModels.value.filter(m => m.status === 'enabled').map(m => m.actual_model_name)
    )
    modelMappings.value = channelModels.value
      .filter(m => m.display_model_name !== m.actual_model_name)
      .map(m => ({ display: m.display_model_name, actual: m.actual_model_name }))
  } catch {
    channelModels.value = []
  }
  // 加载跨渠道自定义模型名
  fetchCrossChannelCustomNames()
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
      value: editingPriorityMap[row.id] ?? row.priority, size: 'small', min: 1, style: { width: '90px' },
      'onUpdate:value': (val: number | null) => { if (val !== null) editingPriorityMap[row.id] = val },
      onBlur: () => handleUpdatePriority(row),
    }),
  },
  {
    title: t('common.status'), key: 'status', width: 100,
    render: (row: Account) => h(NTag, { type: row.status === 'active' ? 'success' : 'error', size: 'small' }, () => row.status === 'active' ? t('common.active') : t('common.disabled')),
  },
  {
    title: t('channels.disabledReason'), key: 'disabled_reason', width: 160, ellipsis: { tooltip: true },
    render: (row: Account) => row.status === 'disabled' ? (row as any).disabled_reason || '-' : '-',
  },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row: Account) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', type: row.status === 'active' ? 'error' : 'success', onClick: () => handleToggleAccount(row) }, () => row.status === 'active' ? t('common.disable') : t('common.enable')),
      row.status === 'disabled' ? h(NButton, { size: 'small', type: 'warning', onClick: () => testAccount(row), loading: testingIds.value.includes(row.id) }, () => t('channels.accountTest')) : null,
      h(NButton, { size: 'small', type: 'error', ghost: true, onClick: () => handleDeleteAccount(row.id) }, () => t('common.delete')),
    ]),
  },
])

async function testAccount(row: Account) {
  testingIds.value.push(row.id)
  try {
    const res: any = await accountApi.testAccount(row.channel_id, row.id)
    const result = res.data
    if (result.success) {
      message.success(t('channels.accountTestPassed'))
      if (selectedChannel.value) await selectChannel(selectedChannel.value)
    } else {
      message.error(result.error || t('channels.accountTestFailed'))
    }
  } catch {
    message.error(t('channels.accountTestFailed'))
  } finally {
    testingIds.value = testingIds.value.filter(id => id !== row.id)
  }
}

async function batchRecover() {
  if (!selectedChannel.value) return
  batchLoading.value = true
  try {
    await accountApi.batchRecover(selectedChannel.value.id)
    message.success('批量恢复已提交，正在后台执行')
    // 延迟刷新，等待后台恢复完成
    setTimeout(async () => {
      if (selectedChannel.value) await selectChannel(selectedChannel.value)
    }, 3000)
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    batchLoading.value = false
  }
}

async function handleBatchTestAccountSelect(mode: string) {
  if (!selectedChannel.value) return
  batchTestAccountLoading.value = true
  try {
    await accountApi.batchTest(selectedChannel.value.id, mode)
    message.success('批量测试已提交，正在后台执行')
    setTimeout(async () => {
      if (selectedChannel.value) await selectChannel(selectedChannel.value)
    }, 3000)
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    batchTestAccountLoading.value = false
  }
}

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
