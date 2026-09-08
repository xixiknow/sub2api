<template>
  <component :is="embedded ? 'div' : AppLayout">
    <div :class="embedded ? 'space-y-6' : 'mx-auto max-w-6xl space-y-6'">
      <div v-if="!embedded" class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-sm text-gray-600 dark:text-dark-300">{{ t('videoStudio.description') }}</p>
        <router-link to="/video-workbench?tab=catalog" class="btn btn-secondary btn-sm">{{ t('videoStudio.catalogLink') }}</router-link>
      </div>

      <section class="card space-y-5 p-5">
        <div v-if="!loadingKeys && studioKeys.length === 0" class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200">
          {{ t('videoStudio.noKeys') }}
          <router-link to="/keys" class="ml-2 underline">{{ t('videoStudio.goKeys') }}</router-link>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <label class="block text-sm">
            <span class="mb-1.5 block text-gray-700 dark:text-dark-200">{{ t('videoStudio.apiKey') }}</span>
            <Select v-model="apiKeyId" :options="keyOptions" :disabled="studioKeys.length === 0" />
          </label>
          <label class="block text-sm">
            <span class="mb-1.5 block text-gray-700 dark:text-dark-200">{{ t('videoStudio.model') }}</span>
            <Select v-model="familyName" :options="modelOptions" :disabled="families.length === 0" />
          </label>
        </div>

        <p v-if="selectedFamily" class="text-xs text-gray-500 dark:text-dark-400">
          {{ selectedFamily.billing_unit === 'per_second' ? t('videoStudio.billingPerSecond') : t('videoStudio.billingPerClip') }}
          · POST {{ selectedFamily.create_path }}
        </p>

        <div class="grid gap-4 lg:grid-cols-3">
          <div class="block text-sm lg:col-span-2">
            <div class="mb-1.5 flex items-baseline justify-between gap-2">
              <span class="text-gray-700 dark:text-dark-200">{{ t('videoStudio.seconds') }}</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ durationHint }}</span>
            </div>
            <div v-if="durationOptions.length > 1 && !selectedFamily?.fixed_duration" class="mb-2 flex flex-wrap gap-1.5">
              <button
                v-for="opt in durationOptions"
                :key="opt"
                type="button"
                class="rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors"
                :class="seconds === opt
                  ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-950/40 dark:text-primary-300'
                  : 'border-gray-200 text-gray-600 hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:text-dark-300 dark:hover:border-primary-500'"
                @click="seconds = opt"
              >
                {{ opt }}s
              </button>
            </div>
            <input
              v-model.number="seconds"
              type="number"
              step="1"
              class="input"
              :min="selectedFamily?.min_duration || 1"
              :max="selectedFamily?.max_duration || 30"
              :disabled="Boolean(selectedFamily?.fixed_duration)"
              :aria-invalid="durationInvalid"
              @blur="clampSeconds"
            >
            <p v-if="durationInvalid" class="mt-1.5 text-xs text-red-600 dark:text-red-400">
              {{ t('videoStudio.secondsOutOfRange', { min: selectedFamily?.min_duration || 1, max: selectedFamily?.max_duration || 30 }) }}
            </p>
          </div>
          <div
            v-if="costEstimate"
            class="flex flex-col justify-center rounded-xl border border-primary-200 bg-primary-50/70 px-4 py-3 dark:border-primary-900/50 dark:bg-primary-950/30"
          >
            <div class="flex items-baseline justify-between gap-3">
              <span class="text-xs font-medium text-gray-600 dark:text-dark-300">{{ t('videoStudio.estimatedCost') }}</span>
              <span class="font-mono text-lg font-semibold text-gray-900 dark:text-white">${{ costEstimate.total.toFixed(4) }}</span>
            </div>
            <p class="mt-1 text-xs text-gray-600 dark:text-dark-300">{{ costEstimateDetail }}</p>
            <p class="mt-1 text-[11px] text-gray-500 dark:text-dark-400">{{ t('videoStudio.estimatedCostHint') }}</p>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <label class="block text-sm">
            <span class="mb-1.5 block text-gray-700 dark:text-dark-200">{{ t('videoStudio.resolution') }}</span>
            <Select v-model="resolution" :options="resolutionOptions" />
          </label>
          <label class="block text-sm">
            <span class="mb-1.5 block text-gray-700 dark:text-dark-200">{{ t('videoStudio.aspect') }}</span>
            <Select v-model="aspectRatio" :options="aspectOptions" :disabled="lockAutoAspect" />
          </label>
        </div>

        <label v-if="selectedFamily?.allow_generate_audio" class="flex items-center gap-2 text-sm">
          <input v-model="generateAudio" type="checkbox" class="rounded">
          {{ t('videoStudio.generateAudio') }}
        </label>

        <label class="block text-sm">
          <span class="mb-1.5 block text-gray-700 dark:text-dark-200">{{ t('videoStudio.prompt') }}</span>
          <textarea v-model="prompt" rows="5" class="input min-h-[8rem]" :placeholder="t('videoStudio.promptPlaceholder')" />
          <p v-if="showPlaceholderHint" class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">{{ t('videoStudio.placeholderHint') }}</p>
        </label>

        <p v-if="usingFirstLast || usingRefs" class="text-xs text-amber-700 dark:text-amber-300">{{ t('videoStudio.mixHint') }}</p>

        <div v-if="selectedFamily?.allow_first_last && !usingRefs" class="grid gap-4 md:grid-cols-2">
          <AssetSlot
            :title="t('videoStudio.firstFrame')"
            kind="image"
            accept="image/*"
            :assets="firstFrame ? [firstFrame] : []"
            :can-add="!firstFrame"
            :show-insert="false"
            :multiple="false"
            @files="onFirstLastFiles('first_frame', $event)"
            @url="onFirstLastUrl('first_frame', $event)"
            @remove="firstFrame = null"
          />
          <AssetSlot
            :title="t('videoStudio.lastFrame')"
            kind="image"
            accept="image/*"
            :assets="lastFrame ? [lastFrame] : []"
            :can-add="!lastFrame"
            :show-insert="false"
            :multiple="false"
            @files="onFirstLastFiles('last_frame', $event)"
            @url="onFirstLastUrl('last_frame', $event)"
            @remove="lastFrame = null"
          />
        </div>

        <div v-if="!usingFirstLast" class="grid gap-4 lg:grid-cols-3">
          <AssetSlot
            :title="t('videoStudio.images')"
            kind="image"
            accept="image/*"
            :assets="imageRefs"
            :can-add="canAddKind('image')"
            :show-insert="true"
            @files="onRefFiles('image', $event)"
            @url="onRefUrl('image', $event)"
            @remove="removeRef"
            @insert="insertToken"
          />
          <AssetSlot
            v-if="selectedFamily?.allow_video"
            :title="t('videoStudio.videos')"
            kind="video"
            accept="video/*"
            :assets="videoRefs"
            :can-add="canAddKind('video')"
            :show-insert="true"
            @files="onRefFiles('video', $event)"
            @url="onRefUrl('video', $event)"
            @remove="removeRef"
            @insert="insertToken"
          />
          <AssetSlot
            v-if="selectedFamily?.allow_audio"
            :title="t('videoStudio.audios')"
            kind="audio"
            accept="audio/*"
            :assets="audioRefs"
            :can-add="canAddKind('audio')"
            :show-insert="true"
            @files="onRefFiles('audio', $event)"
            @url="onRefUrl('audio', $event)"
            @remove="removeRef"
            @insert="insertToken"
          />
        </div>

        <div class="flex justify-end">
          <button type="button" class="btn btn-primary" :disabled="submitting || !selectedKey || !selectedFamily || durationInvalid" @click="submit">
            {{ submitting ? t('videoStudio.submitting') : t('videoStudio.submit') }}
          </button>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-gray-100 px-5 py-3 dark:border-dark-700">
          <h2 class="font-semibold">{{ t('videoStudio.tasks') }}</h2>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingTasks" @click="loadTasks()">
            <Icon name="refresh" size="sm" :class="loadingTasks ? 'animate-spin' : ''" class="mr-1.5" />
            {{ t('videoStudio.refresh') }}
          </button>
        </div>
        <div v-if="tasks.length === 0" class="px-5 py-8 text-sm text-gray-500">{{ t('videoStudio.emptyTasks') }}</div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
              <tr>
                <th class="px-4 py-2">ID</th>
                <th class="px-4 py-2">{{ t('videoStudio.model') }}</th>
                <th class="px-4 py-2">{{ t('videoStudio.status') }}</th>
                <th class="px-4 py-2">{{ t('videoStudio.progress') }}</th>
                <th class="px-4 py-2">{{ t('videoStudio.cost') }}</th>
                <th class="px-4 py-2">{{ t('videoStudio.expiresAt') }}</th>
                <th class="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="task in tasks" :key="task.id" class="border-t border-gray-100 dark:border-dark-700">
                <td class="px-4 py-2 font-mono text-xs">{{ task.id }}</td>
                <td class="px-4 py-2">{{ task.model }}</td>
                <td class="px-4 py-2">
                  <div>{{ statusLabel(task.status) }}</div>
                  <p v-if="taskErrorMessage(task.error)" class="mt-1 max-w-xs truncate text-xs text-red-600 dark:text-red-400" :title="taskErrorMessage(task.error)">
                    {{ taskErrorMessage(task.error) }}
                  </p>
                </td>
                <td class="px-4 py-2">{{ task.progress }}%</td>
                <td class="px-4 py-2 font-mono">{{ formatCost(task) }}</td>
                <td class="px-4 py-2 text-xs" :class="isExpired(task) ? 'text-red-600 dark:text-red-400' : expiresSoon(task) ? 'text-amber-700 dark:text-amber-300' : ''">
                  {{ formatExpires(task) }}
                </td>
                <td class="px-4 py-2 text-right">
                  <button type="button" class="btn btn-secondary btn-sm mr-2" :disabled="!canOpenContent(task)" @click="previewTask(task)">{{ t('videoStudio.preview') }}</button>
                  <button type="button" class="btn btn-secondary btn-sm" :disabled="!canOpenContent(task)" @click="downloadTask(task)">{{ t('videoStudio.download') }}</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <div v-if="previewUrl" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="closePreview">
        <div class="w-full max-w-3xl rounded-xl bg-white p-4 dark:bg-dark-900">
          <div class="mb-3 flex justify-end">
            <button type="button" class="btn btn-secondary btn-sm" @click="closePreview">{{ t('videoStudio.close') }}</button>
          </div>
          <video :src="previewUrl" controls class="max-h-[70vh] w-full rounded-lg bg-black" />
        </div>
      </div>
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import * as keysAPI from '@/api/keys'
import { getVideoCatalog, type VideoCatalogFamily } from '@/api/videoCatalog'
import { uploadVideoAsset, videoAssetToStudio } from '@/api/videoAssets'
import {
  buildCreatePayload,
  createVideoStudioTask,
  clampDuration,
  durationPresets,
  estimateVideoCost,
  getVideoStudioContent,
  isVideoStudioKeyPlatform,
  listVideoStudioTasks,
  maxBytesForKind,
  missingPlaceholders,
  placeholderToken,
  saveBlob,
  taskErrorMessage,
  type VideoStudioAsset,
  type VideoStudioAssetKind,
  type VideoStudioAssetRole,
  type VideoStudioTask
} from '@/api/videoStudio'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ApiKey } from '@/types'
import AssetSlot from './VideoStudioAssetSlot.vue'

defineProps<{
  embedded?: boolean
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loadingKeys = ref(true)
const loadingTasks = ref(false)
const submitting = ref(false)
const families = ref<VideoCatalogFamily[]>([])
const apiKeys = ref<ApiKey[]>([])
const apiKeyId = ref<number | string>('')
const familyName = ref('')
const prompt = ref('')
const seconds = ref(4)
const resolution = ref('720p')
const aspectRatio = ref('16:9')
const generateAudio = ref(false)
const references = ref<VideoStudioAsset[]>([])
const firstFrame = ref<VideoStudioAsset | null>(null)
const lastFrame = ref<VideoStudioAsset | null>(null)
const tasks = ref<VideoStudioTask[]>([])
const previewUrl = ref('')
let pollTimer: number | null = null

const studioKeys = computed(() =>
  apiKeys.value.filter((key) => key.status === 'active' && isVideoStudioKeyPlatform(key.group?.platform))
)
const selectedKey = computed(() => studioKeys.value.find((key) => key.id === Number(apiKeyId.value)) || null)
const selectedFamily = computed(() => families.value.find((item) => item.family === familyName.value) || null)
const lockAutoAspect = computed(() => Boolean(selectedFamily.value?.first_last_requires_auto && firstFrame.value && lastFrame.value))
const imageRefs = computed(() => references.value.filter((item) => item.kind === 'image'))
const videoRefs = computed(() => references.value.filter((item) => item.kind === 'video'))
const audioRefs = computed(() => references.value.filter((item) => item.kind === 'audio'))
const usingFirstLast = computed(() => Boolean(firstFrame.value || lastFrame.value))
const usingRefs = computed(() => references.value.length > 0)
const showPlaceholderHint = computed(() => Boolean(selectedFamily.value?.require_a_placeholders || selectedFamily.value?.require_image_placeholders))
const keyOptions = computed(() => studioKeys.value.map((key) => ({ value: String(key.id), label: `${key.name || key.id} (${key.group?.platform || ''})` })))
const modelOptions = computed(() => families.value.map((item) => ({ value: item.family, label: item.family })))
const resolutionOptions = computed(() => (selectedFamily.value?.prices || []).map((item) => ({ value: item.resolution, label: item.resolution })))
const aspectOptions = computed(() => (selectedFamily.value?.aspect_ratios || []).map((item) => ({ value: item, label: item })))
const durationOptions = computed(() => {
  const family = selectedFamily.value
  if (!family) return []
  if (family.fixed_duration) return [family.fixed_duration]
  return durationPresets(family.min_duration, family.max_duration)
})
const durationInvalid = computed(() => {
  const family = selectedFamily.value
  if (!family) return false
  const duration = family.fixed_duration || seconds.value
  return !Number.isFinite(duration) || duration < family.min_duration || duration > family.max_duration || !Number.isInteger(duration)
})
const durationHint = computed(() => {
  const family = selectedFamily.value
  if (!family) return ''
  if (family.fixed_duration) return t('videoStudio.secondsFixed', { value: family.fixed_duration })
  return t('videoStudio.secondsHint', { min: family.min_duration, max: family.max_duration })
})
const costEstimate = computed(() => {
  const family = selectedFamily.value
  if (!family) return null
  return estimateVideoCost(family, seconds.value, resolution.value)
})
const costEstimateDetail = computed(() => {
  const estimate = costEstimate.value
  if (!estimate) return ''
  const price = Number.isInteger(estimate.unitPrice)
    ? String(estimate.unitPrice)
    : estimate.unitPrice.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')
  if (estimate.billingUnit === 'per_second') {
    return t('videoStudio.estimatedCostPerSecond', {
      resolution: estimate.resolution,
      price,
      seconds: estimate.seconds
    })
  }
  return t('videoStudio.estimatedCostPerClip', { resolution: estimate.resolution, price })
})

watch(studioKeys, (keys) => {
  if (!selectedKey.value && keys.length) apiKeyId.value = String(keys[0].id)
}, { immediate: true })

watch(families, (items) => {
  if (!selectedFamily.value && items.length) familyName.value = items[0].family
}, { immediate: true })

watch(() => selectedFamily.value?.family, () => {
  const family = selectedFamily.value
  if (!family) return
  seconds.value = family.fixed_duration || family.default_duration || family.min_duration || 4
  resolution.value = family.default_resolution || family.prices[0]?.resolution || '720p'
  aspectRatio.value = family.default_aspect || family.aspect_ratios[0] || '16:9'
  if (!family.allow_first_last) {
    firstFrame.value = null
    lastFrame.value = null
  }
  references.value = references.value.filter((item) => {
    if (item.kind === 'video' && !family.allow_video) return false
    if (item.kind === 'audio' && !family.allow_audio) return false
    return true
  })
})

watch(lockAutoAspect, (locked) => {
  if (locked) aspectRatio.value = 'auto'
})

onMounted(async () => {
  try {
    const [catalog, keys] = await Promise.all([
      getVideoCatalog(),
      keysAPI.list(1, 100, { status: 'active', sort_by: 'created_at', sort_order: 'desc' })
    ])
    families.value = catalog?.families || []
    apiKeys.value = keys.items || []
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('videoStudio.loadFailed')))
  } finally {
    loadingKeys.value = false
  }
  await loadTasks()
  pollTimer = window.setInterval(() => {
    if (tasks.value.some((task) => task.status === 'queued' || task.status === 'in_progress')) {
      void loadTasks(true)
    }
  }, 5000)
})

onUnmounted(() => {
  if (pollTimer) window.clearInterval(pollTimer)
  closePreview()
})

watch(apiKeyId, () => {
  void loadTasks()
})

function canAddKind(kind: VideoStudioAssetKind): boolean {
  const family = selectedFamily.value
  if (!family) return false
  const counts = {
    image: imageRefs.value.length,
    video: videoRefs.value.length,
    audio: audioRefs.value.length
  }
  counts[kind] += 1
  if (kind === 'image' && counts.image > family.max_images) return false
  if (kind === 'video' && counts.video > family.max_videos) return false
  if (kind === 'audio' && counts.audio > family.max_audios) return false
  if (counts.image + counts.video + counts.audio > family.max_references) return false
  if (family.max_video_audio && counts.video + counts.audio > family.max_video_audio) return false
  return true
}

function newAsset(kind: VideoStudioAssetKind, role: VideoStudioAssetRole, name: string, source: string, previewUrl?: string): VideoStudioAsset {
  return { id: `${Date.now()}-${Math.random().toString(16).slice(2)}`, kind, role, name, source, previewUrl }
}

async function ingestFiles(kind: VideoStudioAssetKind, files: File[]): Promise<VideoStudioAsset[]> {
  const out: VideoStudioAsset[] = []
  for (const file of files) {
    if (file.size > maxBytesForKind(kind)) {
      appStore.showError(t('videoStudio.fileTooLarge', { kind, mb: Math.round(maxBytesForKind(kind) / 1024 / 1024) }))
      continue
    }
    try {
      const record = await uploadVideoAsset(file)
      const preview = kind === 'image' ? URL.createObjectURL(file) : record.preview_path
      out.push(videoAssetToStudio(record, 'reference'))
      if (preview) out[out.length - 1].previewUrl = preview
    } catch (error) {
      appStore.showError(extractApiErrorMessage(error, t('videoWorkbench.uploadFailed')))
    }
  }
  return out
}

function addLibraryAsset(asset: VideoStudioAsset): boolean {
  if (selectedFamily.value && !canAddKind(asset.kind)) {
    appStore.showError(t('videoStudio.cannotAdd'))
    return false
  }
  if (asset.role === 'first_frame') {
    firstFrame.value = { ...asset, role: 'first_frame' }
    return true
  }
  if (asset.role === 'last_frame') {
    lastFrame.value = { ...asset, role: 'last_frame' }
    return true
  }
  references.value = [...references.value, { ...asset, role: 'reference' }]
  return true
}

defineExpose({ addLibraryAsset })

async function onRefFiles(kind: VideoStudioAssetKind, files: File[]) {
  for (const file of files) {
    if (!canAddKind(kind)) break
    const added = await ingestFiles(kind, [file])
    if (added[0]) references.value = [...references.value, added[0]]
  }
}

async function onFirstLastFiles(role: 'first_frame' | 'last_frame', files: File[]) {
  const added = await ingestFiles('image', files.slice(0, 1))
  const asset = added[0]
  if (!asset) return
  asset.role = role
  if (role === 'first_frame') firstFrame.value = asset
  else lastFrame.value = asset
}

function onRefUrl(kind: VideoStudioAssetKind, url: string) {
  const source = url.trim()
  if (!source.startsWith('https://')) {
    appStore.showError(t('videoStudio.invalidUrl'))
    return
  }
  if (!canAddKind(kind)) return
  references.value = [...references.value, newAsset(kind, 'reference', source, source, kind === 'image' ? source : undefined)]
}

function onFirstLastUrl(role: 'first_frame' | 'last_frame', url: string) {
  const source = url.trim()
  if (!source.startsWith('https://')) {
    appStore.showError(t('videoStudio.invalidUrl'))
    return
  }
  const asset = newAsset('image', role, source, source, source)
  if (role === 'first_frame') firstFrame.value = asset
  else lastFrame.value = asset
}

function removeRef(id: string) {
  references.value = references.value.filter((item) => item.id !== id)
}

function insertToken(asset: VideoStudioAsset) {
  const family = selectedFamily.value
  if (!family) return
  const list = asset.kind === 'image' ? imageRefs.value : asset.kind === 'video' ? videoRefs.value : audioRefs.value
  const index = list.findIndex((item) => item.id === asset.id)
  if (index < 0) return
  const token = placeholderToken(family, asset.kind, index)
  prompt.value = prompt.value ? `${prompt.value.trim()} ${token}` : token
}

function clampSeconds() {
  const family = selectedFamily.value
  if (!family) return
  seconds.value = clampDuration(seconds.value, family.min_duration, family.max_duration, family.fixed_duration)
}

async function submit() {
  const family = selectedFamily.value
  const key = selectedKey.value
  if (!family || !key?.key) return
  clampSeconds()
  if (durationInvalid.value) {
    appStore.showError(t('videoStudio.secondsOutOfRange', { min: family.min_duration, max: family.max_duration }))
    return
  }
  const usingFirstLast = Boolean(firstFrame.value || lastFrame.value)
  if (usingFirstLast && (!firstFrame.value || !lastFrame.value || references.value.length > 0)) {
    appStore.showError(t('videoStudio.firstLastPair'))
    return
  }
  const missing = missingPlaceholders(family, prompt.value, {
    image: usingFirstLast ? 0 : imageRefs.value.length,
    video: usingFirstLast ? 0 : videoRefs.value.length,
    audio: usingFirstLast ? 0 : audioRefs.value.length
  })
  if (missing.length) {
    appStore.showError(t('videoStudio.missingPlaceholders', { tokens: missing.join(' ') }))
    return
  }
  submitting.value = true
  try {
    const payload = buildCreatePayload({
      family,
      prompt: prompt.value,
      seconds: seconds.value,
      resolution: resolution.value,
      aspectRatio: aspectRatio.value,
      generateAudio: generateAudio.value,
      references: usingFirstLast ? [] : references.value,
      firstFrame: firstFrame.value,
      lastFrame: lastFrame.value
    })
    await createVideoStudioTask(key.key, family.create_path, payload)
    appStore.showSuccess(t('videoStudio.submitSuccess'))
    await loadTasks()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('videoStudio.submitFailed')))
  } finally {
    submitting.value = false
  }
}

async function loadTasks(silent = false) {
  const key = selectedKey.value
  if (!key?.key) {
    tasks.value = []
    return
  }
  if (!silent) loadingTasks.value = true
  try {
    const result = await listVideoStudioTasks(key.key, 50, 0)
    tasks.value = result.data || []
  } catch (error) {
    if (!silent) appStore.showError(extractApiErrorMessage(error, t('videoStudio.loadFailed')))
  } finally {
    loadingTasks.value = false
  }
}

function formatCost(task: VideoStudioTask): string {
  const value = task.actual_cost ?? task.hold_amount
  if (value == null) return '—'
  return `$${Number(value).toFixed(4)}`
}

function statusLabel(status: string): string {
  if (status === 'queued') return t('videoStudio.statusQueued')
  if (status === 'in_progress') return t('videoStudio.statusInProgress')
  if (status === 'completed') return t('videoStudio.statusCompleted')
  if (status === 'failed') return t('videoStudio.statusFailed')
  if (status === 'canceled') return t('videoStudio.statusCanceled')
  return status
}

function isExpired(task: VideoStudioTask): boolean {
  return Boolean(task.expires_at && task.expires_at * 1000 <= Date.now())
}

function expiresSoon(task: VideoStudioTask): boolean {
  if (!task.expires_at || isExpired(task)) return false
  return task.expires_at * 1000 - Date.now() < 24 * 60 * 60 * 1000
}

function canOpenContent(task: VideoStudioTask): boolean {
  return task.status === 'completed' && !isExpired(task)
}

function formatExpires(task: VideoStudioTask): string {
  if (!task.expires_at) return t('videoWorkbench.neverExpires')
  if (isExpired(task)) return t('videoStudio.expired')
  return new Date(task.expires_at * 1000).toLocaleString()
}

async function previewTask(task: VideoStudioTask) {
  const key = selectedKey.value
  if (!key?.key) return
  if (!canOpenContent(task)) {
    appStore.showError(isExpired(task) ? t('videoStudio.expired') : t('videoStudio.noPreview'))
    return
  }
  try {
    const blob = await getVideoStudioContent(key.key, task.id)
    closePreview()
    previewUrl.value = URL.createObjectURL(blob)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('videoStudio.downloadFailed')))
  }
}

async function downloadTask(task: VideoStudioTask) {
  const key = selectedKey.value
  if (!key?.key) return
  if (!canOpenContent(task)) {
    appStore.showError(isExpired(task) ? t('videoStudio.expired') : t('videoStudio.noPreview'))
    return
  }
  try {
    const blob = await getVideoStudioContent(key.key, task.id)
    saveBlob(blob, `${task.id}.mp4`)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('videoStudio.downloadFailed')))
  }
}

function closePreview() {
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
  previewUrl.value = ''
}
</script>
