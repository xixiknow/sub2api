<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <p class="text-sm text-gray-600 dark:text-dark-300">{{ t('videoWorkbench.adminDescription') }}</p>

      <div class="flex flex-wrap items-end gap-3">
        <label class="block text-sm">
          <span class="mb-1.5 block text-gray-700 dark:text-dark-200">{{ t('videoWorkbench.userId') }}</span>
          <input v-model="userId" type="number" min="0" class="input w-40" :placeholder="t('videoWorkbench.filterUser')">
        </label>
        <label class="block text-sm">
          <span class="mb-1.5 block text-gray-700 dark:text-dark-200">{{ t('videoWorkbench.kind') }}</span>
          <select v-model="kind" class="input w-36">
            <option value="">{{ t('videoWorkbench.filterAll') }}</option>
            <option value="image">{{ t('videoWorkbench.filterImage') }}</option>
            <option value="video">{{ t('videoWorkbench.filterVideo') }}</option>
            <option value="audio">{{ t('videoWorkbench.filterAudio') }}</option>
          </select>
        </label>
        <button type="button" class="btn btn-primary btn-sm" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
      </div>

      <p class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('videoWorkbench.usedBytes') }}: {{ formatBytes(usedBytes) }} · {{ t('common.total') }} {{ total }}
      </p>

      <div class="card overflow-x-auto">
        <table class="min-w-full text-left text-sm">
          <thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
            <tr>
              <th class="px-4 py-2">ID</th>
              <th class="px-4 py-2">{{ t('videoWorkbench.userId') }}</th>
              <th class="px-4 py-2">{{ t('videoWorkbench.name') }}</th>
              <th class="px-4 py-2">{{ t('videoWorkbench.kind') }}</th>
              <th class="px-4 py-2">{{ t('videoWorkbench.size') }}</th>
              <th class="px-4 py-2">{{ t('videoWorkbench.usedAt') }}</th>
              <th class="px-4 py-2"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td colspan="7" class="px-4 py-8 text-center text-gray-500">{{ t('common.loading') }}</td>
            </tr>
            <tr v-else-if="items.length === 0">
              <td colspan="7" class="px-4 py-8 text-center text-gray-500">{{ t('videoWorkbench.emptyAdmin') }}</td>
            </tr>
            <tr v-for="item in items" :key="item.id" class="border-b border-gray-100 dark:border-dark-800">
              <td class="px-4 py-2 font-mono text-xs">{{ item.id }}</td>
              <td class="px-4 py-2">{{ item.user_id }}</td>
              <td class="max-w-xs truncate px-4 py-2">{{ item.original_name || '—' }}</td>
              <td class="px-4 py-2">{{ item.kind }}</td>
              <td class="px-4 py-2">{{ formatBytes(item.bytes) }}</td>
              <td class="px-4 py-2 text-xs">{{ formatTime(item.last_used_at) }}</td>
              <td class="px-4 py-2 text-right">
                <button type="button" class="btn btn-secondary btn-sm" @click="remove(item)">{{ t('videoWorkbench.delete') }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="total > limit" class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary btn-sm" :disabled="offset <= 0" @click="prevPage">{{ t('videoWorkbench.prevPage') }}</button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="offset + limit >= total" @click="nextPage">{{ t('videoWorkbench.nextPage') }}</button>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminDeleteVideoAsset, adminListVideoAssets, type VideoAssetRecord } from '@/api/videoAssets'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const userId = ref('')
const kind = ref('')
const items = ref<Array<VideoAssetRecord & { user_id: number }>>([])
const total = ref(0)
const usedBytes = ref(0)
const offset = ref(0)
const limit = 50

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function formatTime(unix: number): string {
  if (!unix) return '—'
  return new Date(unix * 1000).toLocaleString()
}

async function load() {
  loading.value = true
  try {
    const parsedUser = Number(userId.value)
    const data = await adminListVideoAssets({
      user_id: parsedUser > 0 ? parsedUser : undefined,
      kind: kind.value || undefined,
      limit,
      offset: offset.value
    })
    items.value = data.items
    total.value = data.total
    usedBytes.value = data.used_bytes
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('videoWorkbench.loadFailed')))
  } finally {
    loading.value = false
  }
}

function prevPage() {
  offset.value = Math.max(0, offset.value - limit)
  void load()
}

function nextPage() {
  offset.value += limit
  void load()
}

async function remove(item: VideoAssetRecord) {
  if (!window.confirm(t('videoWorkbench.deleteConfirm', { name: item.original_name || item.id }))) return
  try {
    await adminDeleteVideoAsset(item.id)
    appStore.showSuccess(t('videoWorkbench.deleted'))
    await load()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('videoWorkbench.deleteFailed')))
  }
}

onMounted(() => {
  void load()
})
</script>
