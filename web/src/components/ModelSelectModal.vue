<template>
  <n-modal v-model:show="dialogVisible" preset="card" style="width: 850px" :content-style="{ display: 'flex', flexDirection: 'column', maxHeight: '85vh', overflow: 'hidden' }" :mask-closable="false">
    <!-- 顶部标题 -->
    <template #header>
      <span>{{ t('channels.fetchModels') }}：{{ channelName }}</span>
    </template>

    <!-- 筛选行：tabs + 搜索 -->
    <div class="filter-bar">
      <n-tabs v-model:value="activeFilter" type="segment" size="small" style="flex: 1">
        <n-tab name="all">{{ t('channels.filterAll') }}</n-tab>
        <n-tab name="new">{{ t('channels.filterNew') }}</n-tab>
        <n-tab name="existing">{{ t('channels.filterExisting') }}</n-tab>
      </n-tabs>
      <n-input v-model:value="searchQuery" :placeholder="t('channels.searchModels')" clearable size="small" style="width: 220px; margin-left: 12px">
        <template #prefix>
          <span style="opacity: 0.5">🔍</span>
        </template>
      </n-input>
    </div>

    <!-- 中间：左侧供应商 + 右侧模型 -->
    <div class="content-area-wrapper">
      <div v-if="fetching" class="loading-placeholder">
        <n-spin size="large" />
      </div>
      <div v-else class="content-area">
        <!-- 左侧供应商列表 -->
        <div class="owner-list">
          <div
            v-for="group in filteredGroups"
            :key="group.owner"
            class="owner-item"
            :class="{ 'owner-item--active': activeOwner === group.owner }"
            @click="activeOwner = group.owner"
          >
            <span class="owner-item__name">{{ group.owner }}</span>
            <n-tag size="small" :type="group.selectedCount > 0 ? 'info' : 'default'">
              {{ group.selectedCount }}/{{ group.models.length }}
            </n-tag>
          </div>
          <n-empty v-if="!fetching && filteredGroups.length === 0" :description="t('channels.noModelsTip')" style="padding: 20px 0" />
        </div>

        <!-- 右侧模型列表 -->
        <div class="model-list">
          <template v-if="currentGroup">
            <div class="model-list__header">
              <n-text depth="3" style="font-size: 12px">{{ currentGroup.models.length }} {{ t('channels.modelCount') }}</n-text>
              <n-button text type="info" size="tiny" @click="toggleGroupAll(currentGroup)">
                {{ isGroupAllSelected(currentGroup) ? t('channels.deselectAll') : t('channels.selectAll') }}
              </n-button>
            </div>
            <div class="model-grid">
              <div
                v-for="m in currentGroup.models"
                :key="m.id"
                class="model-item"
                :class="{ 'model-item--selected': selectedIds.has(m.id) }"
                @click="toggleModel(m.id, !selectedIds.has(m.id))"
              >
                <div v-if="selectedIds.has(m.id)" class="model-item__bar"></div>
                <n-checkbox
                  :checked="selectedIds.has(m.id)"
                  @update:checked="(v: boolean) => toggleModel(m.id, v)"
                  @click.stop
                />
                <n-tooltip trigger="hover" placement="top">
                  <template #trigger>
                    <span class="model-item__name">{{ m.id }}</span>
                  </template>
                  {{ m.id }}
                </n-tooltip>
              </div>
            </div>
          </template>
          <n-empty v-else-if="!fetching" :description="t('channels.selectOwner')" style="padding: 40px 0" />
        </div>
      </div>
    </div>

    <!-- 已选模型标签区 -->
    <div v-if="selectedIds.size > 0" class="selected-area">
      <n-text depth="3" style="font-size: 13px; margin-bottom: 8px; display: block">
        {{ t('channels.selectedModels') }}（{{ selectedIds.size }}）
      </n-text>
      <n-space size="small">
        <n-tag
          v-for="id in selectedIds"
          :key="id"
          closable
          size="small"
          @close="toggleModel(id, false)"
          @click="copyModelName(id)"
          style="cursor: pointer"
          :title="t('channels.clickToCopyModel')"
        >
          {{ id }}
        </n-tag>
      </n-space>
    </div>

    <!-- 模型映射配置 -->
    <div class="mapping-area">
      <n-space justify="space-between" align="center" style="margin-bottom: 8px">
        <n-text depth="3" style="font-size: 13px">{{ t('channels.modelMapping') }}</n-text>
        <n-button size="small" @click="addMapping">+ {{ t('channels.addMapping') }}</n-button>
      </n-space>
      <div v-for="(m, idx) in mappings" :key="idx" style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px">
        <n-input v-model:value="m.display" :placeholder="t('channels.displayName')" size="small" style="flex: 1" />
        <span style="color: var(--text-tertiary)">→</span>
        <n-select
          v-model:value="m.actual"
          :options="selectedModelOptions"
          :placeholder="t('channels.actualName')"
          size="small"
          style="flex: 1"
          filterable
        />
        <n-button size="small" quaternary type="error" @click="mappings.splice(idx, 1)">✕</n-button>
      </div>
    </div>

    <!-- 底部操作栏 -->
    <template #footer>
      <n-space justify="space-between" align="center" style="width: 100%">
        <n-space align="center">
          <n-text>{{ t('channels.selectedModels') }}：{{ selectedIds.size }}</n-text>
          <n-button text type="info" size="small" @click="toggleSelectAll">{{ isAllSelected ? t('channels.deselectAll') : t('channels.selectAll') }}</n-button>
        </n-space>
        <n-space>
          <n-button @click="dialogVisible = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" @click="handleSave" :disabled="selectedIds.size === 0">{{ t('channels.saveModels') }}</n-button>
        </n-space>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, useDialog } from 'naive-ui'
import { channelApi, type ChannelModel, type ModelInfo } from '../api/channel'

const props = defineProps<{
  show: boolean
  channelId: number
  channelName: string
  existingModels: ChannelModel[]
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'save', models: ChannelModel[]): void
}>()

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const dialogVisible = computed({
  get: () => props.show,
  set: (v) => emit('update:show', v),
})

const fetching = ref(false)
const availableModels = ref<ModelInfo[]>([])
const selectedIds = ref<Set<string>>(new Set())
const mappings = ref<{ display: string; actual: string }[]>([])
const searchQuery = ref('')
const activeFilter = ref('all')
const activeOwner = ref('')

// 已配置模型的名称集合（用于判断新/现有/已移除）— 用 actual_model_name 匹配上游真实模型
const existingNameSet = computed(() => {
  const enabled = new Set<string>()
  const disabled = new Set<string>()
  for (const m of props.existingModels) {
    if (m.status === 'enabled') enabled.add(m.actual_model_name)
    else disabled.add(m.actual_model_name)
  }
  return { enabled, disabled }
})

// 分组结构
interface ModelGroup {
  owner: string
  models: ModelInfo[]
  selectedCount: number
}

// 根据筛选 + 搜索过滤后的分组
const filteredGroups = computed<ModelGroup[]>(() => {
  const q = searchQuery.value.toLowerCase().trim()
  const map = new Map<string, ModelInfo[]>()

  for (const m of availableModels.value) {
    const owner = m.owned_by || 'other'

    // 分类筛选
    if (activeFilter.value === 'new') {
      // 新模型：不在已配置中的
      if (existingNameSet.value.enabled.has(m.id) || existingNameSet.value.disabled.has(m.id)) continue
    } else if (activeFilter.value === 'existing') {
      // 现有模型：已配置且启用的
      if (!existingNameSet.value.enabled.has(m.id)) continue
    }

    // 搜索过滤
    if (q && !m.id.toLowerCase().includes(q) && !owner.toLowerCase().includes(q)) continue

    if (!map.has(owner)) map.set(owner, [])
    map.get(owner)!.push(m)
  }

  const groups = Array.from(map.entries()).map(([owner, models]) => ({
    owner,
    models,
    selectedCount: models.filter(m => selectedIds.value.has(m.id)).length,
  }))

  // 自动选中第一个分组
  if (groups.length > 0 && !groups.find(g => g.owner === activeOwner.value)) {
    activeOwner.value = groups[0].owner
  }

  return groups
})

// 当前选中的分组
const currentGroup = computed(() =>
  filteredGroups.value.find(g => g.owner === activeOwner.value) || null
)

// 映射的目标模型选项
const selectedModelOptions = computed(() =>
  Array.from(selectedIds.value).map(id => ({ label: id, value: id }))
)

// 全选状态
const isAllSelected = computed(() => {
  const all = availableModels.value.map(m => m.id)
  return all.length > 0 && all.every(id => selectedIds.value.has(id))
})

// 弹窗打开时自动获取模型
watch(() => props.show, async (val) => {
  if (val && props.channelId) {
    selectedIds.value = new Set(
      props.existingModels
        .filter(m => m.status === 'enabled')
        .map(m => m.actual_model_name)
    )
    mappings.value = props.existingModels
      .filter(m => m.display_model_name !== m.actual_model_name)
      .map(m => ({ display: m.display_model_name, actual: m.actual_model_name }))
    searchQuery.value = ''
    activeFilter.value = 'all'
    activeOwner.value = ''
    await fetchModels()
  }
})

async function fetchModels() {
  fetching.value = true
  try {
    const res = await channelApi.fetchModels(props.channelId, '')
    const raw: ModelInfo[] = res.data.data || []
    // 按 id 去重，只保留首次出现
    const seen = new Set<string>()
    availableModels.value = raw.filter(m => {
      if (seen.has(m.id)) return false
      seen.add(m.id)
      return true
    })
    // 默认选中第一个分组
    if (availableModels.value.length > 0) {
      const firstOwner = (availableModels.value[0].owned_by || 'other')
      activeOwner.value = firstOwner
    }
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    fetching.value = false
  }
}

function toggleModel(id: string, checked: boolean) {
  const newSet = new Set(selectedIds.value)
  if (checked) {
    newSet.add(id)
  } else {
    newSet.delete(id)
  }
  selectedIds.value = newSet
}

function isGroupAllSelected(group: ModelGroup) {
  return group.models.length > 0 && group.models.every(m => selectedIds.value.has(m.id))
}

function toggleGroupAll(group: ModelGroup) {
  if (isGroupAllSelected(group)) {
    // 取消该组全选
    const newSet = new Set(selectedIds.value)
    for (const m of group.models) newSet.delete(m.id)
    selectedIds.value = newSet
  } else {
    // 全选该组
    const newSet = new Set(selectedIds.value)
    for (const m of group.models) newSet.add(m.id)
    selectedIds.value = newSet
  }
}

function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedIds.value = new Set()
  } else {
    selectedIds.value = new Set(availableModels.value.map(m => m.id))
  }
}

async function copyModelName(name: string) {
  try {
    await navigator.clipboard.writeText(name)
    message.success(t('common.copied'))
  } catch {
    message.error(t('common.copyFailed'))
  }
}

function addMapping() {
  mappings.value.push({ display: '', actual: '' })
}

function handleSave() {
  // 校验映射：映射的目标模型必须在已选模型中
  const invalidMappings = mappings.value.filter(
    m => m.display && m.actual && !selectedIds.value.has(m.actual)
  )

  if (invalidMappings.length > 0) {
    const names = invalidMappings.map(m => m.actual).join('、')
    dialog.warning({
      title: t('channels.mappingInvalidTitle'),
      content: t('channels.mappingInvalidContent', { names }),
      positiveText: t('channels.autoFixMapping'),
      negativeText: t('channels.backToEdit'),
      onPositiveClick: () => {
        // 自动补齐：把映射中缺失的模型加入 selectedIds
        const newSet = new Set(selectedIds.value)
        for (const m of invalidMappings) {
          newSet.add(m.actual)
        }
        selectedIds.value = newSet
        // 补齐后重新保存
        doSave()
      },
    })
    return
  }

  doSave()
}

function doSave() {
  const models: ChannelModel[] = []
  const mappedActuals = new Set(mappings.value.filter(m => m.display && m.actual).map(m => m.actual))
  for (const id of selectedIds.value) {
    if (!mappedActuals.has(id)) {
      models.push({
        channel_id: props.channelId,
        display_model_name: id,
        actual_model_name: id,
        status: 'enabled',
      })
    }
  }
  for (const m of mappings.value) {
    if (m.display && m.actual) {
      models.push({
        channel_id: props.channelId,
        display_model_name: m.display,
        actual_model_name: m.actual,
        status: 'enabled',
      })
    }
  }
  emit('save', models)
  dialogVisible.value = false
}
</script>

<style scoped>
.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  flex-shrink: 0;
}

.content-area-wrapper {
  flex-shrink: 0;
  height: 35vh;
}

.loading-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}

.content-area {
  display: flex;
  gap: 0;
  height: 100%;
  border: 1px solid var(--n-border-color, rgba(255,255,255,0.1));
  border-radius: 6px;
  overflow: hidden;
}

.owner-list {
  width: 200px;
  min-width: 200px;
  flex-shrink: 0;
  overflow-y: auto;
  height: 100%;
  border-right: 1px solid var(--n-border-color, rgba(255,255,255,0.1));
  background: rgba(255,255,255,0.02);
}

.owner-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  cursor: pointer;
  transition: background-color 0.15s;
  border-bottom: 1px solid rgba(255,255,255,0.04);
}

.owner-item:hover {
  background: rgba(255,255,255,0.06);
}

.owner-item--active {
  background: rgba(0, 210, 255, 0.1);
  border-left: 3px solid #00d2ff;
}

.owner-item__name {
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  margin-right: 8px;
}

.model-list {
  flex: 1;
  min-height: 0;
  height: 100%;
  overflow-y: auto;
  padding: 0;
}

.model-list__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
  position: sticky;
  top: 0;
  background: var(--n-card-color, #1a1a2e);
  z-index: 1;
}

.model-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 4px 8px;
  padding: 8px 12px;
}

.model-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 8px;
  border-radius: 4px;
  cursor: pointer;
  position: relative;
  overflow: hidden;
  transition: background-color 0.15s;
}

.model-item:hover {
  background-color: rgba(255, 255, 255, 0.06);
}

.model-item--selected {
  background-color: rgba(0, 210, 255, 0.06);
}

.model-item__bar {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 3px;
  background-color: #00d2ff;
  border-radius: 3px 0 0 3px;
}

.model-item__name {
  font-size: 13px;
  font-family: 'Menlo', 'Consolas', monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  max-width: 220px;
}

.selected-area {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--n-border-color, rgba(255,255,255,0.1));
  max-height: 80px;
  overflow-y: auto;
  flex-shrink: 0;
}

.mapping-area {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--n-border-color, rgba(255,255,255,0.1));
  max-height: 160px;
  overflow-y: auto;
  flex-shrink: 0;
}
</style>
