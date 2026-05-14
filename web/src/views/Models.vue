<template>
  <n-card :bordered="false" class="glass-card">
    <template #header>
      <div style="display:flex;align-items:center;justify-content:space-between">
        <h2 class="page-title" style="margin:0">{{ t('models.title') }}</h2>
        <n-space>
          <n-button type="primary" size="small" @click="handleSave" :loading="saving">{{ t('common.save') }}</n-button>
          <n-button tertiary size="small" @click="loadData" :loading="loading">
            <template #icon><n-icon><RefreshIcon /></n-icon></template>
          </n-button>
        </n-space>
      </div>
    </template>

    <n-empty v-if="!loading && upstreamModels.length === 0 && displayModels.length === 0" :description="t('models.noModels')" style="padding:60px 0">
      <template #extra>
        <span style="color:#999;font-size:13px">{{ t('models.noModelsHint') }}</span>
      </template>
    </n-empty>

    <n-grid v-else :cols="2" :x-gap="24" :y-gap="0">
      <!-- 左列：上游模型 -->
      <n-gi>
        <n-card :bordered="true" size="small" :title="t('models.upstreamModels')" style="margin-bottom:16px">
          <template #header-extra>
            <n-space :size="4" align="center">
              <n-button text size="tiny" @click="toggleUpstreamAll(!upstreamAllChecked)">{{ upstreamAllChecked ? t('common.deselectAll') : t('common.selectAll') }}</n-button>
              <n-tag size="small" :bordered="false">{{ upstreamModels.length }}</n-tag>
            </n-space>
          </template>
          <div v-if="upstreamModels.length > 0" style="max-height:500px;overflow-y:auto">
            <n-checkbox-group v-model:value="upstreamChecked">
              <n-space vertical :size="1" style="width:100%">
                <div v-for="item in upstreamModels" :key="'u-'+item.actual_model_name" style="display:flex;align-items:center;gap:10px;padding:5px 8px;border-radius:4px">
                  <n-checkbox :value="item.actual_model_name" />
                  <n-tag size="small" :bordered="false" type="info" class="copyable-tag" @click="copyText(item.actual_model_name)">{{ item.actual_model_name }}</n-tag>
                  <span style="color:#999;font-size:12px">{{ t('models.refCount') }}: {{ item.ref_count }}</span>
                </div>
              </n-space>
            </n-checkbox-group>
          </div>
          <n-empty v-else :description="'—'" style="padding:16px 0" />
        </n-card>
      </n-gi>

      <!-- 右列：映射模型 -->
      <n-gi>
        <n-card :bordered="true" size="small" :title="t('models.mappedModels')" style="margin-bottom:16px">
          <template #header-extra>
            <n-space :size="4" align="center">
              <n-button text size="tiny" @click="toggleDisplayAll(!displayAllChecked)">{{ displayAllChecked ? t('common.deselectAll') : t('common.selectAll') }}</n-button>
              <n-tag size="small" :bordered="false" type="warning">{{ displayModels.length }}</n-tag>
            </n-space>
          </template>
          <div v-if="displayModels.length > 0" style="max-height:500px;overflow-y:auto">
            <n-checkbox-group v-model:value="displayChecked">
              <n-space vertical :size="1" style="width:100%">
                <div v-for="item in displayModels" :key="'d-'+item.display_model_name" style="display:flex;align-items:center;gap:10px;padding:5px 8px;border-radius:4px">
                  <n-checkbox :value="item.display_model_name" />
                  <n-tag size="small" :bordered="false" type="warning" class="copyable-tag" @click="copyText(item.display_model_name)">{{ item.display_model_name }}</n-tag>
                  <span style="color:#999;font-size:12px">{{ t('models.refCount') }}: {{ item.ref_count }}</span>
                </div>
              </n-space>
            </n-checkbox-group>
          </div>
          <n-empty v-else :description="'—'" style="padding:16px 0" />
        </n-card>
      </n-gi>
    </n-grid>
  </n-card>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NCard, NGrid, NGi, NSpace, NTag, NCheckbox, NCheckboxGroup, NButton, NIcon, NEmpty } from 'naive-ui'
import { RefreshOutline as RefreshIcon } from '@vicons/ionicons5'
import { modelApi, type UpstreamModelItem, type DisplayModelItem } from '../api/model'

const { t } = useI18n()
const message = useMessage()

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    message.success(t('common.copied'))
  }).catch(() => {
    message.error(t('common.copyFailed'))
  })
}

const loading = ref(false)
const saving = ref(false)
const upstreamModels = ref<UpstreamModelItem[]>([])
const displayModels = ref<DisplayModelItem[]>([])
const upstreamChecked = ref<string[]>([])
const displayChecked = ref<string[]>([])

// 原始数据（用于 diff）
const upstreamOriginal = ref<Map<string, boolean>>(new Map())
const displayOriginal = ref<Map<string, boolean>>(new Map())

const upstreamAllChecked = computed(() =>
  upstreamModels.value.length > 0 && upstreamModels.value.every(m => upstreamChecked.value.includes(m.actual_model_name))
)

const displayAllChecked = computed(() =>
  displayModels.value.length > 0 && displayModels.value.every(m => displayChecked.value.includes(m.display_model_name))
)

function toggleUpstreamAll(checked: boolean) {
  if (checked) {
    upstreamChecked.value = upstreamModels.value.map(m => m.actual_model_name)
  } else {
    upstreamChecked.value = []
  }
}

function toggleDisplayAll(checked: boolean) {
  if (checked) {
    displayChecked.value = displayModels.value.map(m => m.display_model_name)
  } else {
    displayChecked.value = []
  }
}

async function loadData() {
  loading.value = true
  try {
    const { data } = await modelApi.listModels()
    const { upstream, display } = data?.data || { upstream: [], display: [] }
    upstreamModels.value = upstream
    displayModels.value = display

    // 初始化勾选状态 + 原始记录
    upstreamOriginal.value = new Map()
    displayOriginal.value = new Map()
    upstreamChecked.value = []
    displayChecked.value = []
    for (const m of upstream) {
      upstreamOriginal.value.set(m.actual_model_name, m.visible)
      if (m.visible) upstreamChecked.value.push(m.actual_model_name)
    }
    for (const m of display) {
      displayOriginal.value.set(m.display_model_name, m.visible)
      if (m.visible) displayChecked.value.push(m.display_model_name)
    }
  } catch (e: any) {
    message.error(e?.response?.data?.error?.message || e.message)
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  const checkedSet = new Set([...upstreamChecked.value, ...displayChecked.value])

  // 上游模型变更
  const upstreamDiffs: Promise<any>[] = []
  for (const m of upstreamModels.value) {
    const oldVisible = upstreamOriginal.value.get(m.actual_model_name) ?? true
    const newVisible = checkedSet.has(m.actual_model_name)
    if (oldVisible !== newVisible) {
      upstreamDiffs.push(modelApi.setUpstreamVisible(m.actual_model_name, newVisible))
    }
  }

  // 映射模型变更
  const displayDiffs: Promise<any>[] = []
  for (const m of displayModels.value) {
    const oldVisible = displayOriginal.value.get(m.display_model_name) ?? true
    const newVisible = checkedSet.has(m.display_model_name)
    if (oldVisible !== newVisible) {
      displayDiffs.push(modelApi.setDisplayVisible(m.display_model_name, newVisible))
    }
  }

  const allDiffs = [...upstreamDiffs, ...displayDiffs]
  if (allDiffs.length === 0) {
    message.info(t('common.noChanges'))
    return
  }

  saving.value = true
  try {
    await Promise.all(allDiffs)
    message.success(t('common.saveSuccess'))
    await loadData()
  } catch (e: any) {
    message.error(e?.response?.data?.error?.message || e.message)
  } finally {
    saving.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.copyable-tag {
  cursor: pointer;
}
.copyable-tag:hover {
  opacity: 0.8;
}
</style>