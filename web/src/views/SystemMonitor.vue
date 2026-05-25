<template>
  <div style="display:flex;flex-direction:column;gap:16px">
    <!-- 渠道健康快照卡片 -->
    <n-card :bordered="false" class="glass-card" title="渠道健康快照">
      <template #header-extra>
        <n-button size="small" @click="refresh" :loading="loading">
          <template #icon><n-icon><RefreshOutline /></n-icon></template>
          刷新
        </n-button>
      </template>

      <n-grid cols="1" x-gap="12" y-gap="12" responsive="screen">
        <n-grid-item v-for="ch in channels" :key="ch.id">
          <n-card size="small" :bordered="true" class="channel-health-card">
            <n-space vertical :size="8">
              <!-- 渠道名 + 冷却等级 -->
              <n-space justify="space-between" align="center">
                <n-text strong style="font-size:15px">{{ ch.name }}</n-text>
                <n-space :size="6">
                  <n-tag v-if="ch.cooldown_level !== 'normal'"
                    :type="ch.cooldown_level === 'L2' ? 'error' : 'warning'" size="small" round>
                    {{ ch.cooldown_level }}冷却
                  </n-tag>
                  <n-tag :type="ch.active_ratio >= 0.5 ? 'success' : ch.active_ratio > 0 ? 'warning' : 'error'" size="small" round>
                    {{ (ch.active_ratio * 100).toFixed(0) }}%可用
                  </n-tag>
                </n-space>
              </n-space>

              <!-- 进度条 -->
              <n-progress
                type="line"
                :percentage="ch.active_ratio * 100"
                :color="ch.active_ratio >= 0.5 ? '#18a058' : ch.active_ratio > 0 ? '#f0a020' : '#d03050'"
                :height="8"
                :show-indicator="false"
                :border-radius="4"
              />

              <!-- 账号分布 -->
              <n-space :size="16">
                <n-text depth="3" style="font-size:12px">
                  活跃 <n-text strong type="success">{{ ch.active_accounts }}</n-text>
                </n-text>
                <n-text depth="3" style="font-size:12px">
                  禁用 <n-text strong type="error">{{ ch.disabled_accounts }}</n-text>
                </n-text>
                <n-text depth="3" style="font-size:12px;color:var(--n-warning-color)">
                  冷却 <n-text strong>{{ ch.cooling_accounts }}</n-text>
                </n-text>
                <n-text depth="3" style="font-size:12px">
                  共 {{ ch.total_accounts }}
                </n-text>
              </n-space>

              <!-- 上次探测 -->
              <n-text depth="3" style="font-size:11px">
                上次探测: {{ ch.last_probed_at || t('monitor.neverProbed') }}
              </n-text>
            </n-space>
          </n-card>
        </n-grid-item>
      </n-grid>

      <!-- 空态 -->
      <n-empty v-if="!loading && channels.length === 0" :description="t('monitor.noChannels')" />
    </n-card>

    <!-- 冷却账号汇总 -->
    <n-card :bordered="false" class="glass-card" :title="t('monitor.coolingAccounts')">
      <n-data-table
        :columns="coolingColumns"
        :data="coolingAccounts"
        :loading="loading"
        :pagination="false"
        size="small"
        :bordered="false"
      />
      <n-empty v-if="!loading && coolingAccounts.length === 0" :description="t('monitor.noCoolingAccounts')" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RefreshOutline } from '@vicons/ionicons5'
import { systemApi } from '../api/system'
import type { DataTableColumn } from 'naive-ui'

const { t } = useI18n()

// ========== Data ==========
interface ChannelHealth {
  id: number
  name: string
  active_accounts: number
  disabled_accounts: number
  cooling_accounts: number
  total_accounts: number
  active_ratio: number
  cooldown_cycles: number
  cooldown_level: string
  last_probed_at: string
}

interface CoolingAccount {
  id: number
  account_id: number
  channel_id: number
  channel_name: string
  cooldown_until: string
  cooldown_cycles: number
  cooldown_level: string
  consecutive_failures: number
}

const loading = ref(false)
const channels = ref<ChannelHealth[]>([])
const coolingAccounts = ref<CoolingAccount[]>([])

// ========== Computed ==========
const coolingColumns = computed<DataTableColumn<CoolingAccount>[]>(() => [
  { title: '渠道', key: 'channel_name', width: 120, ellipsis: { tooltip: true } },
  {
    title: '冷却等级',
    key: 'cooldown_level',
    width: 80,
    render(row) {
      return h('span', row.cooldown_level)
    }
  },
  { title: '冷却周期', key: 'cooldown_cycles', width: 80 },
  { title: '连续失败', key: 'consecutive_failures', width: 80 },
  {
    title: '冷却至',
    key: 'cooldown_until',
    width: 160,
    render(row) {
      return row.cooldown_until || '-'
    }
  }
])

// ========== Methods ==========
async function fetchData() {
  loading.value = true
  try {
    const res = await systemApi.getChannelHealth()
    const payload = (res.data as any).data || res.data
    channels.value = payload.channels || []
    coolingAccounts.value = (payload.cooling_accounts || []).map((a: any) => ({
      ...a,
      cooldown_level: a.cooldown_cycles === 1 ? 'L1' : a.cooldown_cycles >= 2 ? 'L2' : 'normal'
    }))
  } catch {
    // silently ignore
  } finally {
    loading.value = false
  }
}

function refresh() {
  fetchData()
}

onMounted(() => {
  fetchData()
})

// NaiveUI render 需要 h 函数
import { h } from 'vue'
</script>

<style scoped>
.channel-health-card {
  border-radius: 8px;
  transition: box-shadow 0.2s;
}
.channel-health-card:hover {
  box-shadow: 0 0 12px rgba(0,0,0,0.06);
}
</style>