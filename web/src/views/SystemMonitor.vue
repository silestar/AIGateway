<template>
  <div style="display:flex;flex-direction:column;gap:16px">
    <!-- 定期渠道测试 -->
    <n-card :bordered="false" class="glass-card settings-section" size="small">
      <template #header>
        <div class="section-header" @click="toggleSection('channelTest')">
          <span class="section-title">{{ t('monitor.channelTest') }}</span>
          <span class="section-desc">{{ t('monitor.testIntervalHint') }}</span>
          <span class="section-arrow" :class="{ expanded: expandedSections.channelTest }">▶</span>
        </div>
      </template>
      <div v-show="expandedSections.channelTest" class="section-body">
        <n-form label-placement="left" label-width="210">
          <n-form-item :label="t('monitor.testInterval')">
            <n-input-number v-model:value="form.channel_health_check_interval" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('monitor.testIntervalHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('monitor.disableOnFailure')">
            <n-switch v-model:value="form.channel_disable_on_failure" />
            <div class="form-hint">{{ t('monitor.disableOnFailureHint') }}</div>
          </n-form-item>
          <n-form-item :label="t('monitor.enableOnSuccess')">
            <n-switch v-model:value="form.channel_enable_on_success" />
            <div class="form-hint">{{ t('monitor.enableOnSuccessHint') }}</div>
          </n-form-item>
        </n-form>
      </div>
    </n-card>

    <!-- 响应时间限制 -->
    <n-card :bordered="false" class="glass-card settings-section" size="small">
      <template #header>
        <div class="section-header" @click="toggleSection('latency')">
          <span class="section-title">{{ t('monitor.latencyThreshold') }}</span>
          <span class="section-desc">{{ t('monitor.latencyThresholdHint') }}</span>
          <span class="section-arrow" :class="{ expanded: expandedSections.latency }">▶</span>
        </div>
      </template>
      <div v-show="expandedSections.latency" class="section-body">
        <n-form label-placement="left" label-width="210">
          <n-form-item :label="t('monitor.latencyThreshold')">
            <n-input-number v-model:value="form.channel_disable_latency_threshold" :min="0" style="width:200px">
              <template #suffix>{{ t('settings.seconds') }}</template>
            </n-input-number>
            <div class="form-hint">{{ t('monitor.latencyThresholdHint') }}</div>
          </n-form-item>
        </n-form>
      </div>
    </n-card>

    <!-- 自动禁用状态码 -->
    <n-card :bordered="false" class="glass-card settings-section" size="small">
      <template #header>
        <div class="section-header" @click="toggleSection('statusCodes')">
          <span class="section-title">{{ t('monitor.disableStatusCodes') }}</span>
          <span class="section-desc">{{ t('monitor.disableStatusCodesHint') }}</span>
          <span class="section-arrow" :class="{ expanded: expandedSections.statusCodes }">▶</span>
        </div>
      </template>
      <div v-show="expandedSections.statusCodes" class="section-body">
        <n-form label-placement="left" label-width="210">
          <n-form-item :label="t('monitor.disableStatusCodes')">
            <n-dynamic-tags v-model:value="disableStatusTags" />
            <div class="form-hint">{{ t('monitor.disableStatusCodesHint') }}</div>
          </n-form-item>
        </n-form>
      </div>
    </n-card>

    <!-- 自动重试状态码 -->
    <n-card :bordered="false" class="glass-card settings-section" size="small">
      <template #header>
        <div class="section-header" @click="toggleSection('retryCodes')">
          <span class="section-title">{{ t('monitor.retryStatusCodes') }}</span>
          <span class="section-desc">{{ t('monitor.retryStatusCodesHint') }}</span>
          <span class="section-arrow" :class="{ expanded: expandedSections.retryCodes }">▶</span>
        </div>
      </template>
      <div v-show="expandedSections.retryCodes" class="section-body">
        <n-form label-placement="left" label-width="210">
          <n-form-item :label="t('monitor.retryStatusCodes')">
            <n-dynamic-tags v-model:value="retryStatusTags" />
            <div class="form-hint">{{ t('monitor.retryStatusCodesHint') }}</div>
          </n-form-item>
        </n-form>
      </div>
    </n-card>

    <!-- 失败关键词 -->
    <n-card :bordered="false" class="glass-card settings-section" size="small">
      <template #header>
        <div class="section-header" @click="toggleSection('keywords')">
          <span class="section-title">{{ t('monitor.failureKeywords') }}</span>
          <span class="section-desc">{{ t('monitor.failureKeywordsHint') }}</span>
          <span class="section-arrow" :class="{ expanded: expandedSections.keywords }">▶</span>
        </div>
      </template>
      <div v-show="expandedSections.keywords" class="section-body">
        <n-form label-placement="left" label-width="210">
          <n-form-item :label="t('monitor.failureKeywords')">
            <div class="keywords-section">
              <div class="keywords-tags">
                <n-tag
                  v-for="(kw, idx) in form.channel_disable_keywords"
                  :key="idx"
                  closable
                  @close="() => removeKeyword(Number(idx))"
                >{{ kw }}</n-tag>
                <span v-if="!form.channel_disable_keywords?.length" class="keywords-empty">
                  {{ t('monitor.keywordsEmpty') }}
                </span>
              </div>
              <div class="keywords-input-row">
                <n-input
                  v-model:value="keywordInput"
                  :placeholder="t('monitor.keywordsPlaceholder')"
                  style="flex:1"
                  @keydown.enter.prevent="addKeyword"
                />
                <n-button @click="addKeyword" size="small">{{ t('monitor.keywordsAdd') }}</n-button>
              </div>
            </div>
            <div class="form-hint">{{ t('monitor.failureKeywordsHint') }}</div>
          </n-form-item>
        </n-form>
      </div>
    </n-card>

    <!-- 保存按钮 -->
    <n-space justify="end">
      <n-button @click="loadConfig">{{ t('settings.loadConfig') }}</n-button>
      <n-button type="primary" @click="handleSave" :loading="saving">{{ t('monitor.save') }}</n-button>
    </n-space>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { systemApi } from '../api/system'

const { t } = useI18n()
const message = useMessage()

const saving = ref(false)

const expandedSections = reactive<Record<string, boolean>>({
  channelTest: true,
  latency: false,
  statusCodes: false,
  retryCodes: false,
  keywords: false,
})

function toggleSection(key: string) {
  expandedSections[key] = !expandedSections[key]
}

// 表单数据
const form = reactive<Record<string, any>>({
  channel_health_check_interval: 43200,
  channel_disable_latency_threshold: 0,
  channel_disable_on_failure: true,
  channel_enable_on_success: true,
  channel_disable_status_codes: [401, 403],
  channel_retry_status_codes: [502, 503, 504],
  channel_disable_keywords: [],
})

// 状态码标签（n-dynamic-tags 需要 string[]）
const disableStatusTags = computed({
  get: () => (form.channel_disable_status_codes || []).map(String),
  set: (val: string[]) => {
    form.channel_disable_status_codes = val.map(Number).filter(n => !isNaN(n))
  },
})

const retryStatusTags = computed({
  get: () => (form.channel_retry_status_codes || []).map(String),
  set: (val: string[]) => {
    form.channel_retry_status_codes = val.map(Number).filter(n => !isNaN(n))
  },
})

const keywordInput = ref('')

function addKeyword() {
  const val = keywordInput.value.trim()
  if (!val) return
  if (!form.channel_disable_keywords.includes(val)) {
    form.channel_disable_keywords.push(val)
  }
  keywordInput.value = ''
}

function removeKeyword(idx: number) {
  form.channel_disable_keywords.splice(idx, 1)
}

async function loadConfig() {
  try {
    const res = await systemApi.getConfig()
    const data = res.data.data
    if (data.account_manager) {
      const am = data.account_manager
      form.channel_health_check_interval = am.channel_health_check_interval ?? 43200
      form.channel_disable_latency_threshold = am.channel_disable_latency_threshold ?? 0
      form.channel_disable_on_failure = am.channel_disable_on_failure ?? true
      form.channel_enable_on_success = am.channel_enable_on_success ?? true
      form.channel_disable_status_codes = am.channel_disable_status_codes ?? [401, 403]
      form.channel_retry_status_codes = am.channel_retry_status_codes ?? [502, 503, 504]
      form.channel_disable_keywords = am.channel_disable_keywords ?? []
    }
  } catch {
    message.error(t('common.operationFailed'))
  }
}

async function handleSave() {
  saving.value = true
  try {
    await systemApi.updateConfig({
      account_manager: {
        channel_health_check_interval: form.channel_health_check_interval,
        channel_disable_latency_threshold: form.channel_disable_latency_threshold,
        channel_disable_on_failure: form.channel_disable_on_failure,
        channel_enable_on_success: form.channel_enable_on_success,
        channel_disable_status_codes: form.channel_disable_status_codes,
        channel_retry_status_codes: form.channel_retry_status_codes,
        channel_disable_keywords: form.channel_disable_keywords,
      },
    })
    message.success(t('monitor.saveSuccess'))
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    saving.value = false
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

.keywords-section {
  width: 100%;
}
.keywords-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
  min-height: 28px;
  align-items: flex-start;
}
.keywords-empty {
  font-size: 13px;
  color: var(--text-tertiary);
  line-height: 28px;
}
.keywords-input-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>
