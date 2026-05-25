<template>
  <div style="display:flex;flex-direction:column;gap:16px">
    <!-- 配置已迁移提示 -->
    <n-card :bordered="false" class="glass-card">
      <n-result
        status="info"
        :title="t('monitor.migratedTitle')"
        :description="t('monitor.migratedDesc')"
      >
        <template #footer>
          <n-button type="primary" @click="$router.push('/settings')">
            {{ t('monitor.goToSettings') }}
          </n-button>
        </template>
      </n-result>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>