<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <p class="text-sm text-gray-600 dark:text-dark-300">{{ t('videoWorkbench.description') }}</p>

      <div class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-900/50 dark:bg-amber-950/30">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="text-sm font-medium text-amber-900 dark:text-amber-100">{{ t('videoWorkbench.noticeTitle') }}</p>
            <p v-if="noticeExpanded" class="mt-1.5 text-sm leading-6 text-amber-900/90 dark:text-amber-200">
              {{ noticeBody }}
            </p>
            <p v-else class="mt-1 text-xs text-amber-800 dark:text-amber-300">{{ t('videoWorkbench.noticeShort') }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="noticeExpanded = !noticeExpanded">
            {{ noticeExpanded ? t('videoWorkbench.noticeDismiss') : t('videoWorkbench.noticeExpand') }}
          </button>
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button
          v-for="item in tabs"
          :key="item.id"
          type="button"
          class="btn btn-sm"
          :class="tab === item.id ? 'btn-primary' : 'btn-secondary'"
          @click="setTab(item.id)"
        >
          {{ item.label }}
        </button>
      </div>

      <VideoStudioView ref="studioRef" v-show="tab === 'studio'" embedded />
      <VideoAssetLibrary v-show="tab === 'assets'" @add-to-task="onAddToTask" />
      <VideoCatalogView v-show="tab === 'catalog'" embedded />
    </div>

    <div
      v-if="showAck"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      role="dialog"
      aria-modal="true"
      :aria-label="t('videoWorkbench.noticeTitle')"
    >
      <div class="max-w-lg rounded-2xl border border-gray-200 bg-white p-6 shadow-xl dark:border-dark-700 dark:bg-dark-900">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('videoWorkbench.noticeTitle') }}</h2>
        <p class="mt-3 text-sm leading-6 text-gray-700 dark:text-dark-200">{{ noticeBody }}</p>
        <div class="mt-5 flex justify-end">
          <button type="button" class="btn btn-primary" @click="ackNotice">{{ t('videoWorkbench.noticeAck') }}</button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { getVideoWorkbenchMeta } from '@/api/videoAssets'
import type { VideoStudioAsset } from '@/api/videoStudio'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import VideoAssetLibrary from './VideoAssetLibrary.vue'
import VideoCatalogView from './VideoCatalogView.vue'
import VideoStudioView from './VideoStudioView.vue'

const NOTICE_KEY = 'video-workbench-notice-ack:v1'
const DEFAULT_OUTPUT_DAYS = 30
const DEFAULT_ASSET_DAYS = 30

type WorkbenchTab = 'studio' | 'assets' | 'catalog'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const tab = ref<WorkbenchTab>('studio')
const noticeExpanded = ref(false)
const showAck = ref(false)
const outputDays = ref(DEFAULT_OUTPUT_DAYS)
const assetDays = ref(DEFAULT_ASSET_DAYS)
const studioRef = ref<{ addLibraryAsset: (asset: VideoStudioAsset) => boolean } | null>(null)

const tabs = computed(() => [
  { id: 'studio' as const, label: t('videoWorkbench.tabStudio') },
  { id: 'assets' as const, label: t('videoWorkbench.tabAssets') },
  { id: 'catalog' as const, label: t('videoWorkbench.tabCatalog') }
])

function retentionLabel(days: number): string {
  if (days <= 0) return t('videoWorkbench.noticeForever')
  return t('videoWorkbench.noticeDays', { days })
}

const noticeBody = computed(() =>
  t('videoWorkbench.noticeBody', {
    outputRetention: retentionLabel(outputDays.value),
    assetRetention: retentionLabel(assetDays.value)
  })
)

function parseTab(value: unknown): WorkbenchTab {
  if (value === 'assets' || value === 'catalog' || value === 'studio') return value
  return 'studio'
}

function setTab(next: WorkbenchTab) {
  tab.value = next
  const query = { ...route.query, tab: next === 'studio' ? undefined : next }
  void router.replace({ path: '/video-workbench', query })
}

function ackNotice() {
  try {
    localStorage.setItem(NOTICE_KEY, '1')
  } catch {
    // ignore quota / private mode
  }
  showAck.value = false
}

watch(
  () => route.query.tab,
  (value) => {
    tab.value = parseTab(value)
  },
  { immediate: true }
)

function onAddToTask(asset: VideoStudioAsset) {
  if (!studioRef.value?.addLibraryAsset(asset)) return
  appStore.showSuccess(t('videoWorkbench.addedToTask'))
  setTab('studio')
}

onMounted(() => {
  try {
    showAck.value = localStorage.getItem(NOTICE_KEY) !== '1'
  } catch {
    showAck.value = true
  }
  void getVideoWorkbenchMeta()
    .then((meta) => {
      outputDays.value = meta.output_retention_days
      assetDays.value = meta.asset_retention_days
    })
    .catch(() => {
      // keep defaults
    })
})
</script>
