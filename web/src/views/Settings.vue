<template>
  <div style="display:flex;flex-direction:column;gap:16px">
    <!-- 服务器设置 -->
    <n-card :bordered="false" class="glass-card settings-section" size="small">
      <template #header>
        <div class="section-header" @click="toggleSection('server')">
          <span class="section-title">{{ t('settings.server') }}</span>
          <span class="section-desc">{{ t('settings.serverDesc') }}</span>
          <span class="section-arrow" :class="{ expanded: expandedSections.server }">▶</span>
        </div>
      </template>
      <div v-show="expandedSections.server" class="section-body">
        <n-form label-placement="left" label-width="180">
          <n-form-item :label="t('settings.serverMode')">
            <n-select v-model:value="form.server.mode" :options="modeOptions" style="width:200px" />
            <div class="form-hint">{{ t('settings.serverModeHint') }}</div>
          </n-form-item>
        </n-form>
      </div>
    </n-card>

    <!-- 日志设置 -->
    <n-card :bordered="false" class="glass-card settings-section" size="small">
      <template #header>
        <div class="section-header" @click="toggleSection('log')">
          <span class="section-title">{{ t('settings.log') }}</span>
          <span class="section-desc">{{ t('settings.logDesc') }}</span>
          <span class="section-arrow" :class="{ expanded: expandedSections.log }">▶</span>
        </div>
      </template>
      <div v-show="expandedSections.log" class="section-body">
        <n-form label-placement="left" label-width="180">
          <n-form-item :label="t('settings.logLevel')">
            <n-select v-model:value="form.log.level" :options="levelOptions" style="width:200px" />
            <div class="form-hint">{{ t('settings.logLevelHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.logDir')">
            <n-input v-model:value="form.log.dir" style="width:200px" />
            <div class="form-hint">{{ t('settings.logDirHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.logMaxAge')">
            <n-input-number v-model:value="form.log.max_age_days" :min="1" :max="365" style="width:200px">
              <template #suffix>{{ t('settings.days') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.logMaxAgeHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.detailLogEnabled')">
            <n-switch v-model:value="form.log.detail_log_enabled" />
            <div class="form-hint">{{ t('settings.detailLogEnabledHint') }}</div>
          </n-form-item>
        </n-form>
      </div>
    </n-card>

    <!-- 代理设置 -->
    <n-card :bordered="false" class="glass-card settings-section" size="small">
      <template #header>
        <div class="section-header" @click="toggleSection('proxy')">
          <span class="section-title">{{ t('settings.proxy') }}</span>
          <span class="section-desc">{{ t('settings.proxyDesc') }}</span>
          <span class="section-arrow" :class="{ expanded: expandedSections.proxy }">▶</span>
        </div>
      </template>
      <div v-show="expandedSections.proxy" class="section-body">
        <n-form label-placement="left" label-width="180">
          <n-form-item :label="t('settings.connectTimeout')">
            <n-input-number v-model:value="form.proxy.connect_timeout" :min="1" :max="300" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.connectTimeoutHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.readTimeout')">
            <n-input-number v-model:value="form.proxy.read_timeout" :min="1" :max="600" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.readTimeoutHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.maxIdleConns')">
            <n-input-number v-model:value="form.proxy.max_idle_conns" :min="1" :max="10000" style="width:200px" />
            <div class="form-hint">{{ t('settings.maxIdleConnsHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.idleConnTimeout')">
            <n-input-number v-model:value="form.proxy.idle_conn_timeout" :min="1" :max="600" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.idleConnTimeoutHint') }}</div>
          </n-form-item>
        </n-form>
      </div>
    </n-card>

    <!-- 账号管理器设置 -->
    <n-card :bordered="false" class="glass-card settings-section" size="small">
      <template #header>
        <div class="section-header" @click="toggleSection('accountManager')">
          <span class="section-title">{{ t('settings.accountManager') }}</span>
          <span class="section-desc">{{ t('settings.accountManagerDesc') }}</span>
          <span class="section-arrow" :class="{ expanded: expandedSections.accountManager }">▶</span>
        </div>
      </template>
      <div v-show="expandedSections.accountManager" class="section-body">
        <n-form label-placement="left" label-width="210">
          <n-form-item :label="t('settings.affinityTTL')">
            <n-input-number v-model:value="form.account_manager.affinity_ttl" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.affinityTTLHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.consecutiveFailureThreshold')">
            <n-input-number v-model:value="form.account_manager.consecutive_failure_threshold" :min="1" style="width:200px" />
            <div class="form-hint">{{ t('settings.consecutiveFailureThresholdHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.minDisableDuration')">
            <n-input-number v-model:value="form.account_manager.min_disable_duration" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.minDisableDurationHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.probeInterval')">
            <n-input-number v-model:value="form.account_manager.probe_interval" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.probeIntervalHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.probeActiveRatioThreshold')">
            <n-input-number v-model:value="form.account_manager.probe_active_ratio_threshold" :min="0" :max="1" :step="0.05" style="width:200px" />
            <div class="form-hint">{{ t('settings.probeActiveRatioThresholdHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.maxProbeFailures')">
            <n-input-number v-model:value="form.account_manager.max_probe_failures" :min="1" style="width:200px" />
            <div class="form-hint">{{ t('settings.maxProbeFailuresHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.maxProbeRecoverPerCycle')">
            <n-input-number v-model:value="form.account_manager.max_probe_recover_per_cycle" :min="1" style="width:200px" />
            <div class="form-hint">{{ t('settings.maxProbeRecoverPerCycleHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.probeCooldownDuration')">
            <n-input-number v-model:value="form.account_manager.probe_cooldown_duration" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.probeCooldownDurationHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.probeCooldownDurationL2')">
            <n-input-number v-model:value="form.account_manager.probe_cooldown_duration_l2" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.probeCooldownDurationL2Hint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.globalHealthCheckInterval')">
            <n-input-number v-model:value="form.account_manager.global_health_check_interval" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.globalHealthCheckIntervalHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.accountStatusCacheTTL')">
            <n-input-number v-model:value="form.account_manager.account_status_cache_ttl" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.accountStatusCacheTTLHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('settings.accountKeyCacheTTL')">
            <n-input-number v-model:value="form.account_manager.account_key_cache_ttl" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('settings.accountKeyCacheTTLHint') }}</div>
          </n-form-item>
        </n-form>
      </div>
    </n-card>

    <!-- 操作按钮 -->
    <n-space justify="end">
      <n-button @click="loadConfig">{{ t('settings.loadConfig') }}</n-button>
      <n-button type="primary" @click="handleSave" :loading="saving">{{ t('common.save') }}</n-button>
      <n-button @click="handleDownloadLogs">{{ t('settings.downloadLogs') }}</n-button>
    </n-space>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { systemApi, systemLogApi } from '../api/system'

const { t } = useI18n()
const message = useMessage()

const saving = ref(false)

// 折叠状态 — 默认全部折叠
const expandedSections = reactive<Record<string, boolean>>({
  server: false,
  log: false,
  proxy: false,
  accountManager: false,
})

function toggleSection(key: string) {
  expandedSections[key] = !expandedSections[key]
}

const form = reactive({
  server: { mode: 'debug' } as Record<string, any>,
  log: { level: 'info', dir: 'logs', max_age_days: 30, detail_log_enabled: true } as Record<string, any>,
  proxy: { connect_timeout: 5, read_timeout: 60, max_idle_conns: 100, idle_conn_timeout: 90 } as Record<string, any>,
  account_manager: {
    affinity_ttl: 3600, consecutive_failure_threshold: 5, min_disable_duration: 120,
    probe_interval: 30, probe_active_ratio_threshold: 0.4, max_probe_failures: 10,
    max_probe_recover_per_cycle: 1, probe_cooldown_duration: 7200,
    probe_cooldown_duration_l2: 86400, global_health_check_interval: 3600,
    account_status_cache_ttl: 30, account_key_cache_ttl: 60,
  } as Record<string, any>,
})

const modeOptions = computed(() => [
  { label: 'debug', value: 'debug' },
  { label: 'release', value: 'release' },
])

const levelOptions = computed(() => [
  { label: 'debug', value: 'debug' },
  { label: 'info', value: 'info' },
  { label: 'warn', value: 'warn' },
  { label: 'error', value: 'error' },
])

async function loadConfig() {
  try {
    const res = await systemApi.getConfig()
    const data = res.data.data
    if (data.server) Object.assign(form.server, data.server)
    if (data.log) Object.assign(form.log, data.log)
    if (data.proxy) Object.assign(form.proxy, data.proxy)
    if (data.account_manager) Object.assign(form.account_manager, data.account_manager)
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleSave() {
  saving.value = true
  try {
    await systemApi.updateConfig({
      server: form.server,
      log: form.log,
      proxy: form.proxy,
      account_manager: form.account_manager,
    })
    message.success(t('common.success'))
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    saving.value = false
  }
}

async function handleDownloadLogs() {
  try {
    const today = new Date().toISOString().slice(0, 10)
    const res = await systemLogApi.download(today)
    const url = URL.createObjectURL(new Blob([res.data]))
    const a = document.createElement('a')
    a.href = url; a.download = `agw-${today}.log`; a.click()
    URL.revokeObjectURL(url)
  } catch {
    message.error(t('common.operationFailed'))
  }
}

onMounted(() => loadConfig())
</script>

<style scoped>
.section-header {
  display: flex;
  align-items: center;
  cursor: pointer;
  user-select: none;
  width: 100%;
}
.section-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}
.section-desc {
  font-size: 13px;
  color: var(--text-secondary);
  margin-left: 12px;
}
.section-arrow {
  margin-left: auto;
  font-size: 11px;
  color: var(--text-tertiary);
  transition: transform 0.25s ease;
}
.section-arrow.expanded {
  transform: rotate(90deg);
}
.section-body {
  padding-top: 8px;
}
.form-hint {
  font-size: 12px;
  color: var(--text-tertiary);
  line-height: 1.5;
  margin-top: 8px;
  padding-left: 4px;
}
.form-hint::before {
  content: 'ⓘ ';
  font-size: 12px;
  opacity: 0.7;
}
</style>