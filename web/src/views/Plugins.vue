<template>
  <n-card :title="t('plugins.title')">
    <n-empty :description="t('plugins.comingSoon')" />
  </n-card>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>
