<template>
  <div class="login-page">
    <div class="login-bg-glow" />
    <n-card :bordered="false" class="login-card">
      <div class="login-brand">
        <div class="login-icon">⚡</div>
        <h1 class="login-title">AIGateway</h1>
        <p class="login-subtitle">{{ t('home.hero.description') }}</p>
      </div>
      <n-form @submit.prevent="handleLogin">
        <n-form-item>
          <n-input
            v-model:value="username"
            size="large"
            :placeholder="t('login.usernamePlaceholder')"
            @keyup.enter="handleLogin"
          />
        </n-form-item>
        <n-form-item>
          <n-input
            v-model:value="password"
            type="password"
            show-password-on="click"
            size="large"
            :placeholder="t('login.passwordPlaceholder')"
            @keyup.enter="handleLogin"
          />
        </n-form-item>
        <n-button type="primary" block size="large" :loading="loading" @click="handleLogin">
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

const username = ref('')
const password = ref('')
const loading = ref(false)

async function handleLogin() {
  if (!username.value || !password.value) return
  loading.value = true
  try {
    const res = await authApi.login(username.value, password.value)
    const sessionToken = res.data.data.token
    localStorage.setItem('agw_token', sessionToken)
    message.success(t('common.success'))
    router.push('/console')
  } catch {
    message.error(t('login.failed'))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100vh;
  background: var(--bg-outer);
  position: relative;
  overflow: hidden;
}

.login-bg-glow {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 600px;
  height: 600px;
  background: radial-gradient(circle, var(--primary-suppl) 0%, transparent 70%);
  pointer-events: none;
}

.login-card {
  width: 420px;
  background: var(--bg-card) !important;
  backdrop-filter: blur(24px);
  border: 1px solid var(--border) !important;
  border-radius: 16px !important;
  box-shadow: var(--shadow-card);
  padding: 40px !important;
  position: relative;
  z-index: 1;
}

.login-brand {
  text-align: center;
  margin-bottom: 32px;
}

.login-icon {
  font-size: 48px;
  margin-bottom: 12px;
}

.login-title {
  font-size: 28px;
  font-weight: 700;
  background: var(--primary-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0 0 8px;
}

.login-subtitle {
  color: var(--text-secondary);
  font-size: 14px;
  margin: 0;
}
</style>