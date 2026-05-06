<template>
  <n-card :bordered="false" class="glass-card">
    <template #header>
      <h2 class="page-title" style="margin:0">{{ t('plugins.title') }}</h2>
    </template>
    <n-empty :description="t('plugins.comingSoon')" style="padding: 60px 0" />
  </n-card>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { NCard, NEmpty } from 'naive-ui'
const { t } = useI18n()
</script>