<template>
  <n-config-provider :locale="naiveLocale" :theme="null">
    <n-message-provider>
      <!-- 登录页：全屏无侧边栏 -->
      <router-view v-if="isLoginPage" />

      <!-- 管理面板：带侧边栏布局 -->
      <n-layout v-else has-sider style="height: 100vh">
        <n-layout-sider
          bordered
          collapse-mode="width"
          :collapsed-width="64"
          :width="220"
          show-trigger
        >
          <div class="logo">
            <h2>AGW</h2>
          </div>
          <n-menu
            :options="menuOptions"
            :value="currentRoute"
            @update:value="handleMenuClick"
          />
        </n-layout-sider>
        <n-layout>
          <n-layout-header bordered style="padding: 12px 24px; display: flex; justify-content: space-between; align-items: center">
            <span>{{ t('app.title') }}</span>
            <n-space align="center">
              <n-select
                v-model:value="currentLang"
                :options="langOptions"
                style="width: 120px"
                size="small"
              />
              <n-button size="small" @click="handleLogout">{{ t('login.logout') }}</n-button>
            </n-space>
          </n-layout-header>
          <n-layout-content style="padding: 24px">
            <router-view />
          </n-layout-content>
        </n-layout>
      </n-layout>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NConfigProvider,
  NLayout,
  NLayoutSider,
  NLayoutHeader,
  NLayoutContent,
  NMenu,
  NMessageProvider,
  NSelect,
  NSpace,
  NButton,
  zhCN,
  enUS,
} from 'naive-ui'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const currentLang = ref('zh-CN')
const currentRoute = computed(() => route.path)
const isLoginPage = computed(() => route.path === '/login')

const naiveLocale = computed(() => currentLang.value === 'zh-CN' ? zhCN : enUS)

const langOptions = [
  { label: '中文', value: 'zh-CN' },
  { label: 'English', value: 'en-US' },
]

const menuOptions = computed(() => [
  { label: t('menu.dashboard'), key: '/' },
  { label: t('menu.consumers'), key: '/consumers' },
  { label: t('menu.channels'), key: '/channels' },
  { label: t('menu.groups'), key: '/groups' },
  { label: t('menu.stats'), key: '/stats' },
  { label: t('menu.logs'), key: '/logs' },
  { label: t('menu.plugins'), key: '/plugins' },
  { label: t('menu.settings'), key: '/settings' },
])

function handleMenuClick(key: string) {
  router.push(key)
}

function handleLogout() {
  localStorage.removeItem('agw_token')
  router.push('/login')
}
</script>

<style>
.logo {
  padding: 16px;
  text-align: center;
}
.logo h2 {
  margin: 0;
  color: #18a058;
}
</style>