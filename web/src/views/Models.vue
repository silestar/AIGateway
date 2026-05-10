<template>
  <n-card :bordered="false" class="glass-card">
    <template #header>
      <div style="display:flex;align-items:center;justify-content:space-between">
        <h2 class="page-title" style="margin:0">{{ t('models.title') }}</h2>
        <n-space>
          <n-button type="primary" size="small" @click="handleSave" :loading="saving">{{ t('common.save') }}</n-button>
          <n-button tertiary size="small" @click="loadCatalog" :loading="loading">
            <template #icon><n-icon><RefreshIcon /></n-icon></template>
          </n-button>
        </n-space>
      </div>
    </template>

    <n-empty v-if="!loading && catalog.length === 0" :description="t('models.noModels')" style="padding:60px 0">
      <template #extra>
        <span style="color:#999;font-size:13px">{{ t('models.noModelsHint') }}</span>
      </template>
    </n-empty>

    <n-grid v-else :cols="2" :x-gap="24" :y-gap="0">
      <n-gi>
        <n-card :bordered="true" size="small" :title="t('models.selectedModels')" style="margin-bottom:16px">
          <template #header-extra>
            <n-space :size="4" align="center">
              <n-button text size="tiny" @click="toggleNormalAll(!normalAllChecked)">{{ normalAllChecked ? t('common.deselectAll') : t('common.selectAll') }}</n-button>
              <n-tag size="small" :bordered="false">{{ normalModels.length }}</n-tag>
            </n-space>
          </template>
          <div v-if="normalModels.length > 0" style="max-height:400px;overflow-y:auto">
            <n-checkbox-group v-model:value="checkedIds">
              <n-space vertical :size="1" style="width:100%">
                <div v-for="item in normalModels" :key="item.id" style="display:flex;align-items:center;gap:10px;padding:5px 8px;border-radius:4px">
                  <n-checkbox :value="item.id" />
                  <n-tag size="small" :bordered="false" type="info">{{ item.model_name }}</n-tag>
                  <span style="color:#999;font-size:12px">{{ t('models.refCount') }}: {{ item.ref_count }}</span>
                </div>
              </n-space>
            </n-checkbox-group>
          </div>
          <n-empty v-else :description="'—'" style="padding:16px 0" />
        </n-card>
      </n-gi>

      <n-gi>
        <n-card :bordered="true" size="small" :title="t('models.mappedModels')" style="margin-bottom:16px">
          <template #header-extra>
            <n-space :size="4" align="center">
              <n-button text size="tiny" @click="toggleMappedAll(!mappedAllChecked)">{{ mappedAllChecked ? t('common.deselectAll') : t('common.selectAll') }}</n-button>
              <n-tag size="small" :bordered="false" type="warning">{{ mappedModels.length }}</n-tag>
            </n-space>
          </template>
          <div v-if="mappedModels.length > 0" style="max-height:400px;overflow-y:auto">
            <n-checkbox-group v-model:value="checkedIds">
              <n-space vertical :size="1" style="width:100%">
                <div v-for="item in mappedModels" :key="item.id" style="display:flex;align-items:center;gap:10px;padding:5px 8px;border-radius:4px">
                  <n-checkbox :value="item.id" />
                  <n-tag size="small" :bordered="false" type="warning">{{ item.model_name }}</n-tag>
                  <span style="color:#999;font-size:12px">{{ t('models.refCount') }}: {{ item.ref_count }}</span>
                </div>
              </n-space>
            </n-checkbox-group>
          </div>
          <n-empty v-else :description="'—'" style="padding:16px 0" />
        </n-card>
      </n-gi>
    </n-grid>

    <div v-if="catalog.length > 0" style="text-align:center;color:#999;font-size:12px;margin-top:16px">
      {{ t('models.total', { count: catalog.length }) }}
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NCard, NGrid, NGi, NSpace, NTag, NCheckbox, NCheckboxGroup, NButton, NIcon, NEmpty } from 'naive-ui'
import { RefreshOutline as RefreshIcon } from '@vicons/ionicons5'
import { modelApi, type ModelCatalogItem } from '../api/model'

const { t } = useI18n()
const message = useMessage()

const loading = ref(false)
const saving = ref(false)
const catalog = ref<ModelCatalogItem[]>([])
const checkedIds = ref<number[]>([])

const normalModels = computed(() => catalog.value.filter(m => !m.is_mapped))
const mappedModels = computed(() => catalog.value.filter(m => m.is_mapped))

const normalAllChecked = computed(() =>
  normalModels.value.length > 0 && normalModels.value.every(m => checkedIds.value.includes(m.id))
)
const mappedAllChecked = computed(() =>
  mappedModels.value.length > 0 && mappedModels.value.every(m => checkedIds.value.includes(m.id))
)

function toggleNormalAll(checked: boolean) {
  const ids = normalModels.value.map(m => m.id)
  if (checked) {
    for (const id of ids) { if (!checkedIds.value.includes(id)) checkedIds.value.push(id) }
  } else {
    checkedIds.value = checkedIds.value.filter(id => !ids.includes(id))
  }
}

function toggleMappedAll(checked: boolean) {
  const ids = mappedModels.value.map(m => m.id)
  if (checked) {
    for (const id of ids) { if (!checkedIds.value.includes(id)) checkedIds.value.push(id) }
  } else {
    checkedIds.value = checkedIds.value.filter(id => !ids.includes(id))
  }
}

function initCheckedIds() {
  checkedIds.value = catalog.value.filter(m => m.visible).map(m => m.id)
}

async function loadCatalog() {
  loading.value = true
  try {
    const { data } = await modelApi.listCatalog()
    catalog.value = data?.data || []
    initCheckedIds()
  } catch (e: any) {
    message.error(e?.response?.data?.error?.message || e.message)
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  const checkedSet = new Set(checkedIds.value)
  const hideIds: number[] = []
  const showIds: number[] = []
  for (const m of catalog.value) {
    if (m.visible && !checkedSet.has(m.id)) hideIds.push(m.id)
    else if (!m.visible && checkedSet.has(m.id)) showIds.push(m.id)
  }
  if (hideIds.length === 0 && showIds.length === 0) {
    message.info(t('common.noChanges'))
    return
  }
  saving.value = true
  try {
    if (hideIds.length > 0) await modelApi.batchUpdateVisibility(hideIds, false)
    if (showIds.length > 0) await modelApi.batchUpdateVisibility(showIds, true)
    for (const m of catalog.value) m.visible = checkedSet.has(m.id)
    message.success(t('common.success'))
  } catch (e: any) {
    message.error(e?.response?.data?.error?.message || e.message)
  } finally {
    saving.value = false
  }
}

onMounted(loadCatalog)
</script>