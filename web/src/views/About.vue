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
        <router-link to="/docs" class="nav-link">{{ t('home.nav.docs') }}</router-link>
        <router-link to="/about" class="nav-link active">{{ t('home.nav.about') }}</router-link>
      </div>
    </nav>
    <div class="generic-content">
      <div class="about-card">
        <div class="about-header">
          <span class="about-icon">⚡</span>
          <h1>AIGateway</h1>
          <p class="version">v{{ appVersion }}</p>
        </div>
        <p class="about-desc">{{ t('about.description') }}</p>
        <div class="about-info">
          <div class="info-item">
            <span class="info-label">{{ t('about.license') }}</span>
            <span class="info-value">Apache 2.0</span>
          </div>
          <div class="info-item">
            <span class="info-label">{{ t('about.tech') }}</span>
            <span class="info-value">Go + Vue 3 + TypeScript</span>
          </div>
          <div class="info-item">
            <span class="info-label">{{ t('about.storage') }}</span>
            <span class="info-value">SQLite / MySQL / PostgreSQL</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const appVersion = __APP_VERSION__
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
  display: flex; align-items: center; justify-content: center; padding: 80px 24px;
}
.about-card {
  background: var(--card-bg); border: 1px solid var(--border);
  border-radius: 16px; padding: 48px; max-width: 540px; width: 100%; text-align: center;
}
.about-header { margin-bottom: 24px; }
.about-icon { font-size: 48px; display: block; margin-bottom: 8px; }
h1 { font-size: 28px; margin: 0 0 4px; }
.version { color: var(--text-tertiary); font-size: 14px; margin: 0; }
.about-desc { color: var(--text-secondary); font-size: 15px; line-height: 1.7; margin: 0 0 32px; }
.about-info { display: flex; flex-direction: column; gap: 12px; }
.info-item { display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid var(--border); }
.info-label { font-size: 14px; color: var(--text-secondary); }
.info-value { font-size: 14px; color: var(--text-primary); font-weight: 500; }
</style>