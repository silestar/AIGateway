<template>
  <div class="home-page">
    <!-- 导航栏 -->
    <nav class="home-nav">
      <div class="nav-brand">
        <span class="brand-icon">⚡</span>
        <span class="brand-name">AIGateway</span>
      </div>
      <div class="nav-links">
        <a href="/" class="nav-link active">{{ t('home.nav.home') }}</a>
        <router-link to="/console" class="nav-link">{{ t('home.nav.console') }}</router-link>
        <router-link to="/docs" class="nav-link">{{ t('home.nav.docs') }}</router-link>
        <router-link to="/about" class="nav-link">{{ t('home.nav.about') }}</router-link>
      </div>
      <div class="nav-right">
        <n-button size="small" class="lang-btn" @click="toggleLang">
          {{ currentLang === 'zh-CN' ? 'EN' : '中文' }}
        </n-button>
        <router-link to="/console">
          <n-button type="primary" size="small" round>{{ t('home.hero.cta') }}</n-button>
        </router-link>
      </div>
    </nav>

    <!-- Hero 区 -->
    <section class="hero-section">
      <div class="hero-bg-glow" />
      <div class="hero-content">
        <h1 class="hero-title">
          <span class="gradient-text">AIGateway</span>
        </h1>
        <p class="hero-subtitle">{{ t('home.hero.subtitle') }}</p>
        <p class="hero-desc">{{ t('home.hero.description') }}</p>
        <div class="hero-actions">
          <router-link to="/console">
            <n-button type="primary" size="large" round class="cta-btn">
              {{ t('home.hero.cta') }}
            </n-button>
          </router-link>
          <router-link to="/docs">
            <n-button size="large" round class="docs-btn" ghost>
              {{ t('home.hero.docs') }}
            </n-button>
          </router-link>
        </div>
      </div>
    </section>

    <!-- 特性区 -->
    <section class="features-section">
      <h2 class="section-title">{{ t('home.features.title') }}</h2>
      <p class="section-subtitle">{{ t('home.features.subtitle') }}</p>
      <div class="features-grid">
        <div v-for="f in features" :key="f.key" class="feature-card">
          <div class="feature-icon">{{ f.icon }}</div>
          <h3 class="feature-title">{{ f.title }}</h3>
          <p class="feature-desc">{{ f.desc }}</p>
        </div>
      </div>
    </section>

    <!-- 技术栈 -->
    <section class="tech-section">
      <h2 class="section-title">{{ t('home.tech.title') }}</h2>
      <div class="tech-tags">
        <span v-for="t in techs" :key="t" class="tech-tag">{{ t }}</span>
      </div>
    </section>

    <!-- 页脚 -->
    <footer class="home-footer">
      <p>© {{ currentYear }} AIGateway Team · v0.2.0</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton } from 'naive-ui'

const { t, locale } = useI18n()

const currentLang = computed(() => locale.value)
const currentYear = new Date().getFullYear()

function toggleLang() {
  locale.value = locale.value === 'zh-CN' ? 'en-US' : 'zh-CN'
  localStorage.setItem('agw_lang', locale.value)
}

const features = computed(() => [
  { key: 'multi', icon: '🔌', title: t('home.features.multi.title'), desc: t('home.features.multi.desc') },
  { key: 'route', icon: '🧠', title: t('home.features.route.title'), desc: t('home.features.route.desc') },
  { key: 'plugin', icon: '🧩', title: t('home.features.plugin.title'), desc: t('home.features.plugin.desc') },
  { key: 'keys', icon: '🔑', title: t('home.features.keys.title'), desc: t('home.features.keys.desc') },
  { key: 'stats', icon: '📊', title: t('home.features.stats.title'), desc: t('home.features.stats.desc') },
  { key: 'ha', icon: '🛡️', title: t('home.features.ha.title'), desc: t('home.features.ha.desc') },
])

const techs = ['Go', 'Vue 3', 'TypeScript', 'GORM', 'SQLite', 'Naive UI', 'Docker', 'Viper']
</script>

<style scoped>
.home-page {
  min-height: 100vh;
  background: var(--bg-outer);
  color: var(--text-primary);
}

/* === 导航栏 === */
.home-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 48px;
  height: 64px;
  background: rgba(8, 12, 24, 0.92);
  backdrop-filter: blur(20px);
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  z-index: 1000;
}
.nav-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}
.brand-icon { font-size: 28px; }
.brand-name {
  font-size: 20px;
  font-weight: 700;
  background: var(--primary-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
.nav-links { display: flex; gap: 4px; }
.nav-link {
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 14px;
  color: var(--text-secondary);
  text-decoration: none;
  transition: all 0.2s;
}
.nav-link:hover { color: var(--text-primary); background: var(--bg-hover); }
.nav-link.active { color: var(--primary); }
.nav-right { display: flex; align-items: center; gap: 12px; }
.lang-btn { min-width: 48px !important; }

/* === Hero === */
.hero-section {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 120px 24px 100px;
  overflow: hidden;
}
.hero-bg-glow {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 800px;
  height: 800px;
  background: radial-gradient(circle, rgba(0, 210, 255, 0.08) 0%, transparent 70%);
  pointer-events: none;
}
.hero-content { position: relative; z-index: 1; text-align: center; max-width: 720px; }
.hero-title { font-size: 64px; font-weight: 800; margin: 0 0 16px; line-height: 1.15; }
.gradient-text {
  background: linear-gradient(135deg, #00d2ff 0%, #7b2ff7 50%, #ff2d95 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
.hero-subtitle {
  font-size: 22px;
  font-weight: 500;
  color: var(--text-secondary);
  margin: 0 0 12px;
}
.hero-desc {
  font-size: 15px;
  color: var(--text-tertiary);
  line-height: 1.7;
  margin: 0 0 40px;
}
.hero-actions { display: flex; gap: 16px; justify-content: center; }
.cta-btn { padding: 12px 36px !important; font-size: 16px !important; height: auto !important; }
.docs-btn { padding: 12px 36px !important; font-size: 16px !important; height: auto !important; }
a { text-decoration: none; }

/* === 特性区 === */
.features-section {
  padding: 80px 48px;
  text-align: center;
}
.section-title {
  font-size: 36px;
  font-weight: 700;
  margin: 0 0 12px;
}
.section-subtitle {
  color: var(--text-secondary);
  font-size: 16px;
  margin: 0 0 48px;
}
.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
  max-width: 1080px;
  margin: 0 auto;
}
.feature-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px 24px;
  text-align: left;
  transition: all 0.3s;
}
.feature-card:hover {
  border-color: rgba(0, 210, 255, 0.3);
  transform: translateY(-4px);
  box-shadow: 0 8px 30px rgba(0, 210, 255, 0.08);
}
.feature-icon { font-size: 36px; margin-bottom: 16px; }
.feature-title { font-size: 18px; font-weight: 600; margin: 0 0 8px; }
.feature-desc { font-size: 14px; color: var(--text-secondary); line-height: 1.6; margin: 0; }

/* === 技术栈 === */
.tech-section {
  padding: 60px 48px 80px;
  text-align: center;
}
.tech-tags {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
  max-width: 720px;
  margin: 32px auto 0;
}
.tech-tag {
  padding: 8px 20px;
  border-radius: 20px;
  background: var(--bg-hover);
  border: 1px solid var(--border);
  font-size: 14px;
  color: var(--text-secondary);
}

/* === 页脚 === */
.home-footer {
  padding: 24px;
  text-align: center;
  border-top: 1px solid var(--border);
  color: var(--text-tertiary);
  font-size: 13px;
}
.home-footer p { margin: 0; }

@media (max-width: 768px) {
  .features-grid { grid-template-columns: 1fr; }
  .hero-title { font-size: 42px; }
  .home-nav { padding: 0 16px; }
  .nav-links { display: none; }
}
</style>