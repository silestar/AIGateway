<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="naiveThemeOverrides" :locale="naiveLocale">
    <n-message-provider>
      <n-dialog-provider>
        <!-- 登录页：全屏无侧边栏 -->
        <router-view v-if="isLoginPage" />

        <!-- 管理面板：侧边栏布局 -->
        <n-layout v-else has-sider class="app-layout">
          <!-- 侧边栏 -->
          <n-layout-sider
            bordered
            collapse-mode="width"
            :collapsed-width="64"
            :width="240"
            show-trigger
            class="app-sider"
          >
            <div class="sider-brand">
              <router-link to="/" class="brand-link">
                <div class="brand-icon">⚡</div>
                <transition name="fade">
                  <span v-if="!collapsed" class="brand-text">AIGateway</span>
                </transition>
              </router-link>
            </div>
            <n-menu
              :options="menuOptions"
              :value="currentRoute"
              @update:value="handleMenuClick"
            />
          </n-layout-sider>

          <!-- 右侧主体 -->
          <n-layout class="app-main">
            <n-layout-header bordered class="app-header">
              <div class="header-left">
                <h2 class="header-title">{{ pageTitle }}</h2>
              </div>
              <!-- === 图标按钮组 === -->
              <n-space align="center" :size="4">
                <!-- 语言切换 -->
                <n-popover trigger="click" placement="bottom-end" ref="langPopoverRef">
                  <template #trigger>
                    <n-button quaternary size="small" class="toolbar-btn" title="Language">
                      <template #icon><span class="toolbar-icon">🌐</span></template>
                      {{ currentLangLabel }}
                    </n-button>
                  </template>
                  <div class="popover-menu">
                    <div
                      v-for="opt in langOptions"
                      :key="opt.value"
                      class="popover-item"
                      :class="{ active: currentLang === opt.value }"
                      @click="switchLang(opt.value)"
                    >
                      {{ opt.label }}
                    </div>
                  </div>
                </n-popover>

                <!-- 主题切换 -->
                <n-popover trigger="click" placement="bottom-end">
                  <template #trigger>
                    <n-button quaternary circle size="medium" class="toolbar-btn" title="Theme">
                      <template #icon>
                        <span class="toolbar-icon">{{ themeIcon }}</span>
                      </template>
                    </n-button>
                  </template>
                  <div class="popover-menu">
                    <div
                      v-for="opt in themeOptions"
                      :key="opt.value"
                      class="popover-item"
                      :class="{ active: themeMode === opt.value }"
                      @click="themeMode = opt.value"
                    >
                      <span class="popover-item-icon">{{ opt.icon }}</span>
                      {{ opt.label }}
                    </div>
                  </div>
                </n-popover>

                <!-- 退出登录：图标 + 文字 -->
                <n-button quaternary size="small" class="toolbar-btn" @click="handleLogout">
                  <template #icon><span class="toolbar-icon">🚪</span></template>
                  {{ t('login.logout') }}
                </n-button>
              </n-space>
            </n-layout-header>

            <!-- 内容区（flex 撑满，Footer 置底） -->
            <n-layout-content class="app-content">
              <div class="content-inner">
                <router-view v-slot="{ Component }">
                  <transition name="fade" mode="out-in">
                    <component :is="Component" :key="route.path + '-' + viewKey" />
                  </transition>
                </router-view>
              </div>

              <!-- Footer -->
              <n-layout-footer bordered class="app-footer">
                <span>AIGateway v{{ appVersion }}</span>
                <span class="footer-divider">｜</span>
                <span>© {{ currentYear }} AGW Team</span>
              </n-layout-footer>
            </n-layout-content>
          </n-layout>
        </n-layout>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NConfigProvider,
  NLayout,
  NLayoutSider,
  NLayoutHeader,
  NLayoutContent,
  NLayoutFooter,
  NMenu,
  NMessageProvider,
  NDialogProvider,
  NPopover,
  NSpace,
  NButton,
  darkTheme,
  zhCN,
  enUS,
  type GlobalThemeOverrides,
} from 'naive-ui'
import { systemApi } from './api/system'

const router = useRouter()
const route = useRoute()
const { t, locale } = useI18n()

// === 语言切换 ===
const currentLang = ref(localStorage.getItem('agw_lang') || 'zh-CN')
watch(currentLang, (val) => {
  locale.value = val
  localStorage.setItem('agw_lang', val)
}, { immediate: true })

const currentRoute = computed(() => route.path)
const isLoginPage = computed(() => route.path === '/login')
const collapsed = computed(() => false)
const naiveLocale = computed(() => currentLang.value === 'zh-CN' ? zhCN : enUS)

// 页面刷新 key：菜单点击同路由时递增，强制子组件重新挂载
const viewKey = ref(0)

// === 页面标题（从 i18n 读取，跟随语言切换） ===
const pageTitle = computed(() => {
  const keyMap: Record<string, string> = {
    '/': 'menu.dashboard',
    '/keys': 'menu.keys',
    '/channels': 'menu.channels',
    '/groups': 'menu.groups',
    '/stats': 'menu.stats',
    '/logs': 'menu.logs',
    '/plugins': 'menu.plugins',
    '/settings': 'menu.settings',
  }
  const key = keyMap[route.path]
  return key ? t(key) : 'AIGateway'
})

const langOptions = [
  { label: '中文', value: 'zh-CN' },
  { label: 'English', value: 'en-US' },
]

const currentLangLabel = computed(() => {
  const opt = langOptions.find(o => o.value === currentLang.value)
  return opt?.label || '中文'
})

import { type PopoverInst } from 'naive-ui'
const langPopoverRef = ref<PopoverInst | null>(null)
function switchLang(val: string) {
  currentLang.value = val
  langPopoverRef.value?.setShow(false)
}

// ============================================
// === 主题切换（含深色/浅色两套 themeOverrides）===
// ============================================
type ThemeMode = 'auto' | 'dark' | 'light'
const THEME_KEY = 'agw_theme'
const themeMode = ref<ThemeMode>((localStorage.getItem(THEME_KEY) as ThemeMode) || 'auto')
const resolvedTheme = ref<'dark' | 'light'>('dark')

// 系统偏好监听
let mediaQuery: MediaQueryList | null = null

function updateResolved() {
  if (themeMode.value === 'auto') {
    resolvedTheme.value = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  } else {
    resolvedTheme.value = themeMode.value
  }
}
function setupSystemListener() {
  if (mediaQuery) mediaQuery.removeEventListener('change', updateResolved)
  mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  mediaQuery.addEventListener('change', updateResolved)
}
watch(themeMode, (mode) => {
  localStorage.setItem(THEME_KEY, mode)
  if (mode === 'auto') setupSystemListener()
  updateResolved()
}, { immediate: true })
onMounted(() => { if (themeMode.value === 'auto') setupSystemListener() })

const naiveTheme = computed(() => resolvedTheme.value === 'dark' ? darkTheme : null)

// 同步 CSS 变量到 html 根元素
watch(resolvedTheme, (theme) => {
  document.documentElement.setAttribute('data-theme', theme)
}, { immediate: true })

const themeIcon = computed(() => {
  switch (themeMode.value) {
    case 'auto': return '🖥️'
    case 'dark': return '🌙'
    case 'light': return '☀️'
  }
})
const themeOptions: { label: string; value: ThemeMode; icon: string }[] = [
  { label: '跟随系统', value: 'auto', icon: '🖥️' },
  { label: '深色模式', value: 'dark', icon: '🌙' },
  { label: '浅色模式', value: 'light', icon: '☀️' },
]

// 深色主题覆盖
const darkOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#00d2ff',
    primaryColorHover: '#33ddff',
    primaryColorPressed: '#00b8e6',
    primaryColorSuppl: 'rgba(0, 210, 255, 0.12)',
    borderRadius: '10px',
    borderRadiusSmall: '6px',
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
    bodyColor: '#0a0e1a',
    cardColor: 'rgba(12, 16, 30, 0.85)',
    modalColor: 'rgba(12, 16, 30, 0.95)',
    popoverColor: 'rgba(14, 18, 34, 0.95)',
    tableColor: 'rgba(12, 16, 30, 0.7)',
    inputColor: 'rgba(16, 22, 42, 0.9)',
    actionColor: 'rgba(16, 22, 42, 0.6)',
    textColorBase: '#e8eaed',
    textColor1: '#e8eaed',
    textColor2: '#8e94a0',
    textColor3: '#5a6070',
    borderColor: 'rgba(255, 255, 255, 0.06)',
    dividerColor: 'rgba(255, 255, 255, 0.04)',
    hoverColor: 'rgba(0, 210, 255, 0.06)',
  },
  Card: {
    color: 'rgba(12, 16, 30, 0.85)',
    textColor: '#e8eaed',
    borderColor: 'rgba(255, 255, 255, 0.06)',
    borderRadius: '14px',
    paddingMedium: '24px',
    titleTextColor: '#e8eaed',
    titleFontWeight: '600',
  },
  Button: { borderRadiusMedium: '8px', borderRadiusSmall: '6px', fontWeight: '600', heightMedium: '38px' },
  Input: {
    color: 'rgba(16, 22, 42, 0.9)', textColor: '#e8eaed', borderRadius: '8px', heightMedium: '38px',
    borderHover: 'rgba(0, 210, 255, 0.4)', borderFocus: 'rgba(0, 210, 255, 0.6)',
    placeholderColor: '#5a6070', colorFocus: 'rgba(16, 22, 42, 0.95)',
  },
  Select: { peers: { InternalSelection: { textColor: '#e8eaed', color: 'rgba(16, 22, 42, 0.9)', placeholderColor: '#5a6070' } } },
  DataTable: {
    tdColor: 'rgba(12, 16, 30, 0.7)', thColor: 'rgba(16, 22, 42, 0.9)',
    thTextColor: '#e8eaed', tdTextColor: '#e8eaed', borderColor: 'rgba(255, 255, 255, 0.06)', thFontWeight: '600',
  },
  Layout: { siderColor: 'rgba(8, 12, 24, 0.95)', headerColor: 'rgba(8, 12, 24, 0.95)', footerColor: 'rgba(8, 12, 24, 0.95)' },
  Menu: {
    itemColorActive: 'rgba(0, 210, 255, 0.12)', itemColorActiveHover: 'rgba(0, 210, 255, 0.18)',
    itemTextColor: '#8e94a0', itemTextColorActive: '#00d2ff', itemTextColorHover: '#e8eaed',
    itemTextColorChildActive: '#00d2ff', itemIconColorActive: '#00d2ff', itemIconColorHover: '#e8eaed',
    arrowColor: '#5a6070', arrowColorActive: '#00d2ff', itemHeight: '44px', borderRadius: '8px',
  },
  Tag: { borderRadius: '6px' },
  DatePicker: { itemTextColor: '#e8eaed', itemColorActive: 'rgba(0, 210, 255, 0.12)', panelColor: 'rgba(14, 18, 34, 0.98)' },
  Pagination: {
    itemTextColor: '#8e94a0', itemTextColorActive: '#00d2ff',
    itemColor: 'rgba(16, 22, 42, 0.6)', itemColorActive: 'rgba(0, 210, 255, 0.12)',
  },
  Divider: { color: 'rgba(255, 255, 255, 0.06)' },
  Notification: { color: 'rgba(14, 18, 34, 0.98)', textColor: '#e8eaed', titleTextColor: '#e8eaed', borderRadius: '10px' },
}

// 浅色主题覆盖
const lightOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#0099cc',
    primaryColorHover: '#33bbee',
    primaryColorPressed: '#007799',
    primaryColorSuppl: 'rgba(0, 153, 204, 0.12)',
    borderRadius: '10px',
    borderRadiusSmall: '6px',
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
    bodyColor: '#f0f2f5',
    cardColor: '#ffffff',
    modalColor: '#ffffff',
    popoverColor: '#ffffff',
    tableColor: '#ffffff',
    inputColor: '#f5f7fa',
    actionColor: '#f5f7fa',
    textColorBase: '#1a1a2e',
    textColor1: '#1a1a2e',
    textColor2: '#555770',
    textColor3: '#8889a0',
    borderColor: 'rgba(0, 0, 0, 0.08)',
    dividerColor: 'rgba(0, 0, 0, 0.04)',
    hoverColor: 'rgba(0, 153, 204, 0.06)',
  },
  Card: {
    color: '#ffffff', textColor: '#1a1a2e', borderColor: 'rgba(0, 0, 0, 0.06)',
    borderRadius: '14px', paddingMedium: '24px', titleTextColor: '#1a1a2e', titleFontWeight: '600',
  },
  Button: { borderRadiusMedium: '8px', borderRadiusSmall: '6px', fontWeight: '600', heightMedium: '38px' },
  Input: {
    color: '#f5f7fa', textColor: '#1a1a2e', borderRadius: '8px', heightMedium: '38px',
    borderHover: 'rgba(0, 153, 204, 0.4)', borderFocus: 'rgba(0, 153, 204, 0.6)',
    placeholderColor: '#8889a0', colorFocus: '#ffffff',
  },
  Select: { peers: { InternalSelection: { textColor: '#1a1a2e', color: '#f5f7fa', placeholderColor: '#8889a0' } } },
  DataTable: {
    tdColor: '#ffffff', thColor: '#f5f7fa', thTextColor: '#1a1a2e', tdTextColor: '#1a1a2e',
    borderColor: 'rgba(0, 0, 0, 0.06)', thFontWeight: '600',
  },
  Layout: { siderColor: '#ffffff', headerColor: '#ffffff', footerColor: '#ffffff' },
  Menu: {
    itemColorActive: 'rgba(0, 153, 204, 0.1)', itemColorActiveHover: 'rgba(0, 153, 204, 0.15)',
    itemTextColor: '#555770', itemTextColorActive: '#0099cc', itemTextColorHover: '#1a1a2e',
    itemTextColorChildActive: '#0099cc', itemIconColorActive: '#0099cc', itemIconColorHover: '#1a1a2e',
    arrowColor: '#8889a0', arrowColorActive: '#0099cc', itemHeight: '44px', borderRadius: '8px',
  },
  Tag: { borderRadius: '6px' },
  DatePicker: { itemTextColor: '#1a1a2e', itemColorActive: 'rgba(0, 153, 204, 0.1)', panelColor: '#ffffff' },
  Pagination: {
    itemTextColor: '#555770', itemTextColorActive: '#0099cc',
    itemColor: '#f5f7fa', itemColorActive: 'rgba(0, 153, 204, 0.1)',
  },
  Divider: { color: 'rgba(0, 0, 0, 0.06)' },
  Notification: { color: '#ffffff', textColor: '#1a1a2e', titleTextColor: '#1a1a2e', borderRadius: '10px' },
}

const naiveThemeOverrides = computed(() =>
  resolvedTheme.value === 'dark' ? darkOverrides : lightOverrides
)

// === Footer 版本信息 ===
const appVersion = ref('0.1.0')
const currentYear = new Date().getFullYear()
onMounted(async () => {
  try {
    const res = await systemApi.info()
    appVersion.value = (res.data as { data: { version: string } }).data.version
  } catch { /* default */ }
})

// === 方法 ===
function handleMenuClick(key: string) {
  if (route.path === key) {
    // 点击当前已激活的菜单 → 强制刷新页面状态
    viewKey.value++
  }
  router.push(key)
}
function handleLogout() { localStorage.removeItem('agw_token'); router.push('/login') }

const menuOptions = computed(() => [
  { label: t('menu.dashboard'), key: '/', icon: () => '📊' },
  { label: t('menu.keys'), key: '/keys', icon: () => '🔑' },
  { label: t('menu.channels'), key: '/channels', icon: () => '🔌' },
  { label: t('menu.groups'), key: '/groups', icon: () => '📁' },
  { label: t('menu.stats'), key: '/stats', icon: () => '📈' },
  { label: t('menu.logs'), key: '/logs', icon: () => '📋' },
  { label: t('menu.plugins'), key: '/plugins', icon: () => '🧩' },
  { label: t('menu.settings'), key: '/settings', icon: () => '⚙️' },
])
</script>

<style>
/* === GLOBAL (non-scoped) === */
.app-layout {
  height: 100vh;
  background: var(--bg-outer) !important;
}

.app-sider {
  background: var(--sider-bg) !important;
  backdrop-filter: blur(20px);
  border-right: 1px solid var(--border) !important;
}

.app-main {
  background: transparent !important;
  display: flex !important;
  flex-direction: column !important;
  height: 100vh !important;
}

.app-header {
  background: var(--header-bg) !important;
  backdrop-filter: blur(16px);
  border-bottom: 1px solid var(--border) !important;
  padding: 0 24px !important;
  display: flex !important;
  align-items: center;
  justify-content: space-between;
  height: 60px;
  flex-shrink: 0;
  position: sticky !important;
  top: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
}

.header-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  background: var(--primary-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

/* === 工具栏按钮 === */
.toolbar-btn {
  color: var(--text-secondary) !important;
  transition: color 0.2s, background 0.2s;
}
.toolbar-btn:hover {
  color: var(--primary) !important;
  background: var(--bg-hover) !important;
}
.toolbar-icon {
  font-size: 17px;
  line-height: 1;
}

/* === Popover 菜单 === */
.popover-menu {
  min-width: 120px;
  padding: 4px;
}
.popover-item {
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.15s;
}
.popover-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}
.popover-item.active {
  color: var(--primary);
  background: rgba(0, 210, 255, 0.1);
}
.popover-item-icon { font-size: 15px; }

/* === 内容区 === */
.app-content {
  flex: 1;
  display: flex !important;
  flex-direction: column !important;
  overflow: auto !important;
  background: transparent !important;
  padding: 0 !important;
  min-height: 0 !important;
}
.content-inner {
  flex: 1 1 auto;
  padding: 24px;
}

/* === Footer === */
.app-footer {
  background: var(--footer-bg) !important;
  backdrop-filter: blur(16px);
  border-top: 1px solid var(--border) !important;
  padding: 10px 24px !important;
  text-align: center;
  font-size: 12px;
  color: var(--text-tertiary);
  flex-shrink: 0;
  margin-top: auto;
}
.footer-divider { color: var(--border); }

/* 侧边栏品牌区 */
.sider-brand {
  display: flex; align-items: center; gap: 10px;
  padding: 20px 20px 24px;
}
.brand-link {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
  cursor: pointer;
}
.brand-icon { font-size: 28px; flex-shrink: 0; }
.brand-text {
  font-size: 18px; font-weight: 700;
  background: var(--primary-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  white-space: nowrap;
}

/* 菜单项 */
.n-menu .n-menu-item-content { margin: 2px 8px; }
</style>