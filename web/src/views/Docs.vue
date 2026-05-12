<template>
  <div class="generic-page">
    <nav class="generic-nav">
      <router-link to="/" class="nav-brand">
        <span class="brand-icon">⚡</span>
        <span class="brand-name">AIGateway</span>
      </router-link>
      <div class="nav-links">
        <router-link to="/" class="nav-link">{{ t('home.nav.home') }}</router-link>
        <router-link to="/console" class="nav-link">{{ t('home.nav.console') }}</router-link>
        <router-link to="/docs" class="nav-link active">{{ t('home.nav.docs') }}</router-link>
        <router-link to="/about" class="nav-link">{{ t('home.nav.about') }}</router-link>
      </div>
    </nav>
    <div class="generic-content">
      <div class="coming-soon">
        <span class="soon-icon">📖</span>
        <h1>{{ t('docs.title') }}</h1>
        <p>{{ t('docs.comingSoon') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>

<style scoped>
.generic-page { min-height: 100vh; background: var(--bg-outer); }
.generic-nav {
  display: flex; align-items: center; gap: 32px;
  padding: 0 48px; height: 64px;
  background: rgba(8, 12, 24, 0.92); backdrop-filter: blur(20px);
  border-bottom: 1px solid var(--border);
}
.nav-brand { display: flex; align-items: center; gap: 10px; text-decoration: none; }
.brand-icon { font-size: 28px; }
.brand-name { font-size: 20px; font-weight: 700; background: var(--primary-gradient); -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text; }
.nav-links { display: flex; gap: 4px; }
.nav-link { padding: 8px 16px; border-radius: 8px; font-size: 14px; color: var(--text-secondary); text-decoration: none; }
.nav-link.active { color: var(--primary); }
.generic-content {
  display: flex; align-items: center; justify-content: center; padding: 120px 24px;
}
.coming-soon { text-align: center; }
.soon-icon { font-size: 64px; display: block; margin-bottom: 16px; }
h1 { font-size: 28px; margin: 0 0 8px; }
p { color: var(--text-secondary); font-size: 16px; }
</style>