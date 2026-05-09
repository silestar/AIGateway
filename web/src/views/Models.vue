<template>
  <n-card :bordered="false" class="glass-card">
    <template #header>
      <div style="display:flex;align-items:center;justify-content:space-between">
        <h2 class="page-title" style="margin:0">{{ t('models.title') }}</h2>
        <n-button tertiary @click="loadCatalog">
          <template #icon><n-icon><RefreshIcon /></n-icon></template>
          {{ t('models.refresh') }}
        </n-button>
      </div>
    </template>

    <!-- 空状态 -->
    <n-empty v-if="!loading && catalog.length === 0" :description="t('models.noModels')" style="padding:60px 0">
      <template #extra>
        <span style="color:#999;font-size:13px">{{ t('models.noModelsHint') }}</span>
      </template>
    </n-empty>

    <!-- 左右两列 -->
    <n-grid v-else :cols="2" :x-gap="24" :y-gap="0">
      <!-- 左列：已选模型 -->
      <n-gi>
        <n-card :bordered="true" size="small" :title="t('models.selectedModels')" style="margin-bottom:16px">
          <template #header-extra>
            <n-tag size="small" :bordered="false">{{ selectedModels.length }}</n-tag>
          </template>
          <n-list v-if="selectedModels.length > 0" bordered size="small">
            <n-list-item v-for="item in selectedModels" :key="item.id">
              <div style="display:flex;align-items:center;justify-content:space-between">
                <div style="display:flex;align-items:center;gap:8px">
                  <n-tag size="small" :bordered="false" type="info">{{ item.model_name }}</n-tag>
                  <span style="color:#999;font-size:12px">{{ t('models.refCount') }}: {{ item.ref_count }}</span>
                </div>
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-switch
                      size="small"
                      :value="item.visible"
                      @update:value="(v: boolean) => toggleVisibility(item, v)"
                    />
                  </template>
                  {{ item.visible ? t('models.visibilityOn') : t('models.visibilityOff') }}
                </n-tooltip>
              </div>
            </n-list-item>
          </n-list>
          <n-empty v-else :description="'—'" style="padding:20px 0" />
        </n-card>
      </n-gi>

      <!-- 右列：自定义映射模型 -->
      <n-gi>
        <n-card :bordered="true" size="small" :title="t('models.mappedModels')" style="margin-bottom:16px">
          <template #header-extra>
            <n-tag size="small" :bordered="false" type="warning">{{ mappedModels.length }}</n-tag>
          </template>
          <n-list v-if="mappedModels.length > 0" bordered size="small">
            <n-list-item v-for="item in mappedModels" :key="item.id">
              <div style="display:flex;align-items:center;justify-content:space-between">
                <div style="display:flex;align-items:center;gap:8px">
                  <n-tag size="small" :bordered="false" type="warning">{{ item.model_name }}</n-tag>
                  <span style="color:#999;font-size:12px">{{ t('models.refCount') }}: {{ item.ref_count }}</span>
                </div>
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <n-switch
                      size="small"
                      :value="item.visible"
                      @update:value="(v: boolean) => toggleVisibility(item, v)"
                    />
                  </template>
                  {{ item.visible ? t('models.visibilityOn') : t('models.visibilityOff') }}
                </n-tooltip>
              </div>
            </n-list-item>
          </n-list>
          <n-empty v-else :description="'—'" style="padding:20px 0" />
        </n-card>
      </n-gi>
    </n-grid>

    <!-- 底部统计 -->
    <div v-if="catalog.length > 0" style="text-align:center;color:#999;font-size:12px;margin-top:16px">
      {{ t('models.total', { count: catalog.length }) }}
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NCard, NGrid, NGi, NList, NListItem, NTag, NSwitch, NButton, NIcon, NEmpty, NTooltip } from 'naive-ui'
import { RefreshOutline as RefreshIcon } from '@vicons/ionicons5'
import { modelApi, type ModelCatalogItem } from '../api/model'

const { t } = useI18n()
const message = useMessage()

const loading = ref(false)
const catalog = ref<ModelCatalogItem[]>([])

const selectedModels = computed(() => catalog.value.filter(m => !m.is_mapped))
const mappedModels = computed(() => catalog.value.filter(m => m.is_mapped))

async function loadCatalog() {
  loading.value = true
  try {
    const { data } = await modelApi.listCatalog()
    catalog.value = data?.data || []
  } catch (e: any) {
    message.error(e?.response?.data?.error?.message || e.message)
  } finally {
    loading.value = false
  }
}

async function toggleVisibility(item: ModelCatalogItem, visible: boolean) {
  try {
    await modelApi.updateVisibility(item.id, visible)
    item.visible = visible
  } catch (e: any) {
    message.error(e?.response?.data?.error?.message || e.message)
  }
}

onMounted(loadCatalog)
</script>
