<template>
  <n-button size="small" type="primary" ghost :loading="loading" @click="handleCopy">
    <template #icon><n-icon><CopyOutlined /></n-icon></template>
    {{ t('common.copyKey') }}
  </n-button>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage, NButton, NIcon } from 'naive-ui'
import { CopyOutlined } from '@vicons/antd'

const props = defineProps<{
  type: 'keys' | 'account'
  id: number
  revealFn: (id: number) => Promise<{ data: { api_key: string } }>
}>()

const { t } = useI18n()
const message = useMessage()
const loading = ref(false)

async function handleCopy() {
  // 密钥无法 reveal，只支持复制新创建时显示的 key
  // 账号密钥可以通过 reveal-key API 获取
  loading.value = true
  try {
    const res = await props.revealFn(props.id)
    const apiKey = res.data.api_key
    if (!apiKey) {
      message.warning(t('common.keyNotAvailable'))
      return
    }
    await navigator.clipboard.writeText(apiKey)
    message.success(t('common.copied'))
  } catch (e: unknown) {
    message.error(t('common.copyFailed'))
  } finally {
    loading.value = false
  }
}
</script>
