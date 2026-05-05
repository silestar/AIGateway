<template>
  <div style="display: flex; justify-content: center; align-items: center; height: 100vh; background: #f5f5f5">
    <n-card :title="t('login.title')" style="width: 400px">
      <n-form @submit.prevent="handleLogin">
        <n-form-item>
          <n-input
            v-model:value="token"
            type="password"
            show-password-on="click"
            :placeholder="t('login.tokenPlaceholder')"
            @keyup.enter="handleLogin"
          />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="handleLogin">
          {{ t('login.submit') }}
        </n-button>
      </n-form>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useMessage, NCard, NForm, NFormItem, NInput, NButton } from 'naive-ui'
import { authApi } from '../api/auth'

const { t } = useI18n()
const router = useRouter()
const message = useMessage()

const token = ref('')
const loading = ref(false)

async function handleLogin() {
  if (!token.value) return
  loading.value = true
  try {
    const res = await authApi.login(token.value)
    const sessionToken = res.data.data.token
    localStorage.setItem('agw_token', sessionToken)
    message.success(t('common.success'))
    router.push('/')
  } catch {
    message.error(t('login.failed'))
  } finally {
    loading.value = false
  }
}
</script>
