<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="min-w-0 flex-1">
        <p class="text-sm text-gray-600 dark:text-dark-300">{{ quotaLabel }}</p>
        <div v-if="list && list.quota_bytes > 0" class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
          <div class="h-full rounded-full bg-primary-500" :style="{ width: `${quotaPercent}%` }" />
        </div>
      </div>
      <label class="btn btn-primary btn-sm">
        {{ t('videoWorkbench.upload') }}
        <input type="file" class="hidden" multiple accept="image/jpeg,image/png,image/webp,video/mp4,video/quicktime,audio/mpeg,audio/wav" @change="onFiles">
      </label>
    </div>

    <div class="flex flex-wrap gap-2">
      <button
        v-for="item in filters"
        :key="item.id"
        type="button"
        class="btn btn-sm"
        :class="kind === item.id ? 'btn-primary' : 'btn-secondary'"
        @click="setKind(item.id)"
      >
        {{ item.label }}
      </button>
    </div>

    <div v-if="loading" class="card px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="items.length === 0" class="card px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('videoWorkbench.emptyAssets') }}
    </div>
    <ul v-else class="grid gap-3 sm:grid-cols-2">
      <li v-for="item in items" :key="item.id" class="card flex gap-3 p-3">
        <div class="h-16 w-16 shrink-0 overflow-hidden rounded-lg bg-gray-100 dark:bg-dark-800">
          <img v-if="previews[item.id]" :src="previews[item.id]" alt="" class="h-full w-full object-cover">
          <div v-else class="flex h-full items-center justify-center text-[10px] uppercase text-gray-400">{{ item.kind }}</div>
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ item.original_name || item.id }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            {{ t(`videoWorkbench.filter${capitalize(item.kind)}`) }} · {{ formatBytes(item.bytes) }}
          </p>
          <p class="mt-1 text-xs" :class="expiresClass(item)">{{ expiresLabel(item) }}</p>
          <div class="mt-2 flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary btn-sm" @click="addToTask(item)">{{ t('videoWorkbench.addToTask') }}</button>
            <button type="button" class="btn btn-secondary btn-sm" @click="remove(item)">{{ t('videoWorkbench.delete') }}</button>
          </div>
        </div>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { deleteVideoAsset, getVideoAssetContent, listVideoAssets, uploadVideoAsset, videoAssetToStudio, type VideoAssetList, type VideoAssetRecord } from '@/api/videoAssets'
import type { VideoStudioAsset } from '@/api/videoStudio'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const emit = defineEmits<{
  'add-to-task': [asset: VideoStudioAsset]
}>()

const { t } = useI18n()
const appStore = useAppStore()

type KindFilter = '' | 'image' | 'video' | 'audio'

const kind = ref<KindFilter>('')
const loading = ref(false)
const list = ref<VideoAssetList | null>(null)
const previews = ref<Record<string, string>>({})

const items = computed(() => list.value?.items ?? [])

const filters = computed(() => [
  { id: '' as const, label: t('videoWorkbench.filterAll') },
  { id: 'image' as const, label: t('videoWorkbench.filterImage') },
  { id: 'video' as const, label: t('videoWorkbench.filterVideo') },
  { id: 'audio' as const, label: t('videoWorkbench.filterAudio') }
])

const quotaPercent = computed(() => {
  const quota = list.value?.quota_bytes ?? 0
  const used = list.value?.used_bytes ?? 0
  if (quota <= 0) return 0
  return Math.min(100, Math.round((used / quota) * 100))
})

const quotaLabel = computed(() => {
  const used = formatBytes(list.value?.used_bytes ?? 0)
  const quota = list.value?.quota_bytes ?? 0
  if (quota <= 0) return t('videoWorkbench.assetQuotaUnlimited', { used })
  return t('videoWorkbench.assetQuota', { used, quota: formatBytes(quota) })
})

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1)
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function expiresAt(item: VideoAssetRecord): number | null {
  const days = list.value?.retention_days ?? 0
  if (days <= 0) return null
  const base = item.last_used_at || item.created_at
  return base + days * 24 * 60 * 60
}

function isExpired(item: VideoAssetRecord): boolean {
  const at = expiresAt(item)
  return Boolean(at && at * 1000 <= Date.now())
}

function expiresClass(item: VideoAssetRecord): string {
  const at = expiresAt(item)
  if (!at) return 'text-gray-500 dark:text-dark-400'
  if (isExpired(item)) return 'text-red-600 dark:text-red-400'
  if (at * 1000 - Date.now() < 24 * 60 * 60 * 1000) return 'text-amber-700 dark:text-amber-300'
  return 'text-gray-500 dark:text-dark-400'
}

function expiresLabel(item: VideoAssetRecord): string {
  const at = expiresAt(item)
  if (!at) return `${t('videoWorkbench.expiresAt')}: ${t('videoWorkbench.neverExpires')}`
  if (isExpired(item)) return t('videoWorkbench.expired')
  return `${t('videoWorkbench.expiresAt')}: ${new Date(at * 1000).toLocaleString()}`
}

function setKind(next: KindFilter) {
  kind.value = next
}

function revokePreviews() {
  for (const url of Object.values(previews.value)) {
    URL.revokeObjectURL(url)
  }
  previews.value = {}
}

async function loadPreviews(records: VideoAssetRecord[]) {
  revokePreviews()
  const next: Record<string, string> = {}
  await Promise.all(records.filter((item) => item.kind === 'image').map(async (item) => {
    try {
      const blob = await getVideoAssetContent(item.id)
      next[item.id] = URL.createObjectURL(blob)
    } catch {
      // preview is optional
    }
  }))
  previews.value = next
}

async function load() {
  loading.value = true
  try {
    const data = await listVideoAssets(kind.value || undefined)
    list.value = data
    await loadPreviews(data.items)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('videoWorkbench.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function onFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  for (const file of files) {
    try {
      await uploadVideoAsset(file)
      appStore.showSuccess(t('videoWorkbench.uploaded'))
    } catch (error) {
      appStore.showError(extractApiErrorMessage(error, t('videoWorkbench.uploadFailed')))
    }
  }
  if (files.length > 0) await load()
}

function addToTask(item: VideoAssetRecord) {
  emit('add-to-task', videoAssetToStudio(item, 'reference'))
}

async function remove(item: VideoAssetRecord) {
  if (!window.confirm(t('videoWorkbench.deleteConfirm', { name: item.original_name || item.id }))) return
  try {
    await deleteVideoAsset(item.id)
    appStore.showSuccess(t('videoWorkbench.deleted'))
    await load()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('videoWorkbench.deleteFailed')))
  }
}

watch(kind, () => {
  void load()
})

onMounted(() => {
  void load()
})

onUnmounted(() => {
  revokePreviews()
})
</script>
