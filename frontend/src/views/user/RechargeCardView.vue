<template>
  <AppLayout>
    <div class="flex min-h-[calc(100dvh-10rem)] flex-col gap-4">
      <div class="card flex flex-wrap items-center justify-between gap-4 p-5">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('rechargeCard.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('rechargeCard.description') }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <router-link to="/redeem" class="btn btn-secondary">{{ t('rechargeCard.redeem') }}</router-link>
          <a v-if="shop.enabled" :href="shop.url" target="_blank" rel="noopener noreferrer" class="btn btn-primary">
            {{ t('rechargeCard.newWindow') }}
          </a>
        </div>
      </div>
      <div v-if="settingsLoading" role="status" class="card p-10 text-center text-gray-500">{{ t('rechargeCard.loading') }}</div>
      <div v-else-if="!shop.enabled" class="card p-10 text-center text-gray-500">{{ t('rechargeCard.unavailable') }}</div>
      <template v-else-if="shop.mode === 'iframe'">
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('rechargeCard.embedHint') }}</p>
        <p v-if="frameLoading || frameFailed" role="status" class="text-sm text-gray-500">
          {{ t(frameFailed ? 'rechargeCard.loadError' : 'rechargeCard.loading') }}
        </p>
        <iframe
          :key="shop.url" :src="shop.url" :title="t('rechargeCard.title')"
          class="card min-h-[65dvh] w-full flex-1 border-0 bg-white"
          referrerpolicy="no-referrer" allow="payment" allowfullscreen
          @load="frameLoading = false" @error="onFrameError"
        />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { resolveRechargeCard } from '@/utils/rechargeCard'

const { t } = useI18n()
const appStore = useAppStore()
const shop = computed(() => resolveRechargeCard(appStore.cachedPublicSettings))
const settingsLoading = ref(!appStore.publicSettingsLoaded)
const frameLoading = ref(true)
const frameFailed = ref(false)
watch(() => shop.value.url, () => { frameLoading.value = true; frameFailed.value = false })
function onFrameError() { frameLoading.value = false; frameFailed.value = true }
onMounted(async () => {
  try { await appStore.fetchPublicSettings() }
  finally { settingsLoading.value = false }
})
</script>
