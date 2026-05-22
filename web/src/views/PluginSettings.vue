<template>
  <div class="plugin-settings">
    <h2>🔌 插件配置</h2>

    <n-tabs type="line">
      <!-- Tab 1: 钩子管理 -->
      <n-tab-pane name="hooks" tab="钩子管理">
        <n-data-table
          :columns="hookColumns"
          :data="hooks"
          :loading="loading"
          :row-key="(row: HookItem) => row.id"
        />

        <!-- 插件管理弹窗 -->
        <n-modal v-model:show="showPluginsModal" title="插件管理">
          <n-card>
            <n-data-table
              :columns="pluginColumns"
              :data="hookPlugins"
              :row-key="(row: any) => row.id"
            />
          </n-card>
        </n-modal>
      </n-tab-pane>

      <!-- Tab 2: 预留 -->
      <n-tab-pane name="other" tab="其他">
        <n-empty description="暂无配置" />
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import {
  NDataTable, NTabs, NTabPane, NModal, NCard,
  NSwitch, NButton, NEmpty, NInputNumber,
  useMessage
} from 'naive-ui'
import { hookApi, pluginApi, type HookItem, type PluginItem } from '../api/plugin'

const message = useMessage()
const hooks = ref<HookItem[]>([])
const loading = ref(false)

// 钩子表格列
const hookColumns = [
  { title: '钩子标识', key: 'name', width: 200 },
  { title: '类型', key: 'hook_type', width: 100 },
  { title: '描述', key: 'description' },
  { title: '超时(ms)', key: 'timeout', width: 100 },
  {
    title: '钩子状态',
    key: 'enabled',
    width: 100,
    render(row: HookItem) {
      return h(NSwitch, {
        value: row.enabled,
        onUpdateValue: (val: boolean) => toggleHook(row, val)
      })
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    render(row: HookItem) {
      return h(NButton, { size: 'small', onClick: () => openPluginManager(row) }, () => '插件管理')
    }
  },
]

// 钩子状态切换
const toggleHook = async (hook: HookItem, enabled: boolean) => {
  try {
    await hookApi.updateEnabled(hook.id, enabled)
    hook.enabled = enabled
    message.success(enabled ? '钩子已启用' : '钩子已禁用')
  } catch (e: any) {
    message.error(e.message || '操作失败')
  }
}

// ===== 插件管理弹窗 =====
const showPluginsModal = ref(false)
const currentHook = ref<HookItem | null>(null)
const hookPlugins = ref<PluginItem[]>([])

const pluginColumns = [
  { title: '中文名', key: 'display_name', render: (row: any) => row.display_name || row.name },
  { title: '标识', key: 'name', width: 150 },
  { title: '描述', key: 'description' },
  {
    title: '优先级',
    key: 'priority',
    width: 120,
    render(row: any) {
      return h(NInputNumber, {
        value: row.priority,
        min: 0,
        size: 'small',
        onUpdateValue: (val: number | null) => updatePriority(row, val)
      })
    }
  },
]

const openPluginManager = async (hook: HookItem) => {
  currentHook.value = hook
  try {
    const res = await hookApi.getPlugins(hook.name)
    hookPlugins.value = res.data.data || []
  } catch (e: any) {
    message.error('加载插件失败: ' + (e.message || ''))
    hookPlugins.value = []
  }
  showPluginsModal.value = true
}

const updatePriority = async (plugin: PluginItem, priority: number | null) => {
  if (priority === null) return
  try {
    await pluginApi.updatePriority(plugin.id, priority)
    plugin.priority = priority
    message.success('优先级已更新')
  } catch (e: any) {
    message.error(e.message || '更新失败')
  }
}

// 加载钩子
const fetchHooks = async () => {
  loading.value = true
  try {
    const res = await hookApi.list()
    hooks.value = res.data.data || []
  } catch (e: any) {
    message.error('加载钩子失败: ' + (e.message || ''))
  } finally {
    loading.value = false
  }
}

onMounted(fetchHooks)
</script>

<style scoped>
.plugin-settings {
  padding: 24px;
}
</style>