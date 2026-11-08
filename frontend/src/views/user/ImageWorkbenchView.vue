<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="page-title">{{ t('imageWorkbench.title') }}</h1>
          <p class="page-description">{{ t('imageWorkbench.description') }}</p>
        </div>
        <span v-if="store.pendingRecords.length" class="badge badge-primary">
          {{ t('imageWorkbench.pendingBadge', { count: store.pendingRecords.length }) }}
        </span>
      </div>

      <div v-if="!dbAvailable" class="card border-red-200 p-4 text-sm text-red-600 dark:border-red-900/50 dark:text-red-400" role="alert">
        {{ t('imageWorkbench.errors.noIndexedDb') }}
      </div>

      <div v-if="loadingKeys" class="flex justify-center py-16"><LoadingSpinner /></div>

      <div v-else-if="eligibleKeys.length === 0" class="card flex flex-col items-center gap-3 p-10 text-center">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('imageWorkbench.empty.title') }}</h2>
        <p class="max-w-lg text-sm text-gray-500 dark:text-dark-400">{{ t('imageWorkbench.empty.description') }}</p>
        <router-link to="/keys" class="btn btn-primary">{{ t('imageWorkbench.empty.goKeys') }}</router-link>
      </div>

      <div v-else class="grid grid-cols-1 gap-6 lg:grid-cols-[380px_minmax(0,1fr)]">
        <!-- 参数面板 -->
        <section class="card h-fit space-y-5 p-5 lg:sticky lg:top-20">
          <div class="grid grid-cols-2 gap-1 rounded-xl bg-gray-100 p-1 dark:bg-dark-800" role="tablist">
            <button
              v-for="option in modeOptions"
              :key="option.value"
              type="button"
              role="tab"
              :aria-selected="form.mode === option.value"
              class="rounded-lg px-3 py-1.5 text-sm font-medium transition"
              :class="form.mode === option.value ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white' : 'text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200'"
              @click="form.mode = option.value"
            >
              {{ option.label }}
            </button>
          </div>

          <div>
            <label class="input-label">{{ t('imageWorkbench.form.apiKey') }}</label>
            <Select v-model="form.apiKeyId" :options="keyOptions" :aria-label="t('imageWorkbench.form.apiKey')" />
          </div>

          <div>
            <label class="input-label">{{ t('imageWorkbench.form.model') }}</label>
            <Select v-model="form.model" :options="modelOptions" :aria-label="t('imageWorkbench.form.model')" />
          </div>

          <div>
            <div class="mb-1.5 flex items-center justify-between">
              <label class="input-label !mb-0" for="iw-prompt">{{ t('imageWorkbench.form.prompt') }}</label>
              <span class="text-xs text-gray-400 dark:text-dark-500">{{ form.prompt.length }}/4000</span>
            </div>
            <textarea
              id="iw-prompt"
              v-model="form.prompt"
              class="input min-h-[120px] resize-y"
              maxlength="4000"
              :placeholder="t(form.mode === 'edit' ? 'imageWorkbench.form.editPromptPlaceholder' : 'imageWorkbench.form.promptPlaceholder')"
              @keydown.ctrl.enter.prevent="submit"
              @keydown.meta.enter.prevent="submit"
            />
            <div class="mt-2 flex flex-wrap gap-1.5">
              <button
                v-for="style in STYLE_PRESETS"
                :key="style"
                type="button"
                class="rounded-full border border-gray-200 px-2.5 py-0.5 text-xs text-gray-600 transition hover:border-primary-400 hover:text-primary-600 dark:border-dark-600 dark:text-dark-300 dark:hover:border-primary-500 dark:hover:text-primary-400"
                @click="appendStyle(style)"
              >
                + {{ t(`imageWorkbench.styles.${style}.name`) }}
              </button>
            </div>
            <button type="button" class="mt-2 text-xs font-medium text-primary-600 hover:underline dark:text-primary-400" @click="showPresets = !showPresets">
              {{ t(showPresets ? 'imageWorkbench.presets.hide' : 'imageWorkbench.presets.show') }}
            </button>
            <div v-if="showPresets" class="mt-2 grid grid-cols-2 gap-2">
              <button
                v-for="preset in PROMPT_PRESETS"
                :key="preset.id"
                type="button"
                class="rounded-lg border border-gray-200 p-2 text-left text-xs transition hover:border-primary-400 hover:bg-primary-50/50 dark:border-dark-600 dark:hover:border-primary-500 dark:hover:bg-primary-900/10"
                @click="applyPreset(preset)"
              >
                <div class="font-medium text-gray-800 dark:text-dark-100">{{ t(`imageWorkbench.presets.items.${preset.id}.name`) }}</div>
                <div class="mt-0.5 line-clamp-2 text-gray-500 dark:text-dark-400">{{ t(`imageWorkbench.presets.items.${preset.id}.prompt`) }}</div>
              </button>
            </div>
          </div>

          <!-- 参考图 -->
          <div v-if="form.mode === 'edit'">
            <label class="input-label">{{ t('imageWorkbench.form.references') }}</label>
            <div
              class="rounded-xl border-2 border-dashed p-3 transition"
              :class="dragging ? 'border-primary-400 bg-primary-50/50 dark:bg-primary-900/10' : 'border-gray-200 dark:border-dark-600'"
              @dragover.prevent="dragging = true"
              @dragleave.prevent="dragging = false"
              @drop.prevent="onDrop"
            >
              <div v-if="references.length" class="mb-2 grid grid-cols-4 gap-2">
                <div v-for="(item, idx) in references" :key="item.id" class="group relative aspect-square overflow-hidden rounded-lg bg-gray-100 dark:bg-dark-800">
                  <img :src="item.preview" :alt="item.file.name" class="h-full w-full object-cover" />
                  <button
                    type="button"
                    class="absolute right-1 top-1 rounded-full bg-black/60 px-1.5 text-xs text-white opacity-0 transition group-hover:opacity-100 focus:opacity-100"
                    :aria-label="t('imageWorkbench.actions.remove')"
                    @click="removeReference(idx)"
                  >
                    ×
                  </button>
                </div>
              </div>
              <button type="button" class="w-full text-center text-xs text-gray-500 dark:text-dark-400" @click="fileInput?.click()">
                {{ t('imageWorkbench.form.referencesHint', { max: MAX_REFERENCES }) }}
              </button>
              <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp" multiple class="hidden" @change="onPickFiles" />
            </div>
            <div v-if="platform === 'openai'" class="mt-2 flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
              <span>{{ t('imageWorkbench.form.mask') }}</span>
              <template v-if="mask">
                <span class="truncate text-gray-700 dark:text-dark-200">{{ mask.name }}</span>
                <button type="button" class="text-red-500 hover:underline" @click="mask = null">{{ t('imageWorkbench.actions.remove') }}</button>
              </template>
              <button v-else type="button" class="text-primary-600 hover:underline dark:text-primary-400" @click="maskInput?.click()">
                {{ t('imageWorkbench.form.maskPick') }}
              </button>
              <input ref="maskInput" type="file" accept="image/png" class="hidden" @change="onPickMask" />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div v-if="platform === 'openai'" class="col-span-2">
              <label class="input-label">{{ t('imageWorkbench.form.size') }}</label>
              <Select v-model="form.size" :options="sizeOptions" :aria-label="t('imageWorkbench.form.size')" />
            </div>
            <div v-else class="col-span-2">
              <label class="input-label">{{ t('imageWorkbench.form.aspectRatio') }}</label>
              <Select v-model="form.aspectRatio" :options="aspectOptions" :aria-label="t('imageWorkbench.form.aspectRatio')" />
            </div>
            <div v-if="currentModel?.supportsQuality">
              <label class="input-label">{{ t('imageWorkbench.form.quality') }}</label>
              <Select v-model="form.quality" :options="qualityOptions" :aria-label="t('imageWorkbench.form.quality')" />
            </div>
            <div v-if="currentModel?.supportsBackground">
              <label class="input-label">{{ t('imageWorkbench.form.background') }}</label>
              <Select v-model="form.background" :options="backgroundOptions" :aria-label="t('imageWorkbench.form.background')" />
            </div>
            <div class="col-span-2">
              <label class="input-label">{{ t('imageWorkbench.form.count') }}</label>
              <div class="grid grid-cols-4 gap-1 rounded-xl bg-gray-100 p-1 dark:bg-dark-800">
                <button
                  v-for="n in maxCount"
                  :key="n"
                  type="button"
                  class="rounded-lg py-1 text-sm transition"
                  :class="form.n === n ? 'bg-white font-medium text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white' : 'text-gray-500 dark:text-dark-400'"
                  @click="form.n = n"
                >
                  {{ n }}
                </button>
              </div>
            </div>
          </div>

          <div class="rounded-xl bg-gray-50 p-3 text-xs text-gray-600 dark:bg-dark-800/60 dark:text-dark-300">
            <div class="flex items-center justify-between">
              <span>{{ t('imageWorkbench.cost.label') }}</span>
              <span class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ estimate.total === null ? t('imageWorkbench.cost.default') : `≈ ${formatUsd(estimate.total)}` }}
              </span>
            </div>
            <p v-if="estimate.total !== null" class="mt-1 text-gray-400 dark:text-dark-500">
              {{ t('imageWorkbench.cost.detail', { tier: estimate.tier, unit: formatUsd(estimate.unitPrice ?? 0), count: form.n, multiplier: estimate.multiplier }) }}
            </p>
            <p class="mt-1 text-gray-400 dark:text-dark-500">{{ t('imageWorkbench.cost.note') }}</p>
          </div>

          <p v-if="submitError" class="rounded-lg bg-red-50 p-2 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-400" role="alert">{{ submitError }}</p>

          <button type="button" class="btn btn-primary btn-lg w-full" :disabled="!canSubmit" @click="submit">
            {{ submitting ? t('imageWorkbench.actions.submitting') : t('imageWorkbench.actions.generate') }}
          </button>
          <p class="text-center text-xs text-gray-400 dark:text-dark-500">
            {{ t(store.asyncSupported === false ? 'imageWorkbench.hints.syncMode' : 'imageWorkbench.hints.asyncMode') }}
          </p>
        </section>

        <!-- 历史 -->
        <section class="space-y-4">
          <div class="flex flex-wrap items-center gap-2">
            <div class="flex gap-1 rounded-xl bg-gray-100 p-1 dark:bg-dark-800">
              <button
                v-for="option in filterOptions"
                :key="option.value"
                type="button"
                class="rounded-lg px-3 py-1 text-xs font-medium transition"
                :class="filter === option.value ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white' : 'text-gray-500 dark:text-dark-400'"
                @click="filter = option.value"
              >
                {{ option.label }}
              </button>
            </div>
            <input v-model.trim="search" type="search" class="input !w-auto min-w-[180px] flex-1 !py-1.5 text-sm" :placeholder="t('imageWorkbench.history.search')" />
          </div>

          <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-dark-400">
            <span>
              {{ t('imageWorkbench.storage.summary', { records: store.records.length, images: store.imageCount }) }}
              <template v-if="storage"> · {{ t('imageWorkbench.storage.usage', { used: formatBytes(storage.usage), quota: formatBytes(storage.quota) }) }}</template>
              <template v-if="storage && !storage.persisted"> · {{ t('imageWorkbench.storage.notPersisted') }}</template>
            </span>
            <button v-if="store.records.length > 0" type="button" class="text-red-500 hover:underline" @click="confirmClearOldest">
              {{ t('imageWorkbench.storage.clearOldest', { count: CLEAR_BATCH }) }}
            </button>
          </div>
          <div v-if="overSoftLimit" class="rounded-lg bg-amber-50 p-3 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300" role="status">
            {{ t('imageWorkbench.storage.overLimit', { images: SOFT_LIMIT_IMAGES, size: formatBytes(SOFT_LIMIT_BYTES) }) }}
          </div>

          <div v-if="!store.loaded && dbAvailable" class="flex justify-center py-16"><LoadingSpinner /></div>
          <div v-else-if="filteredRecords.length === 0" class="card p-10 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t(store.records.length ? 'imageWorkbench.history.noMatch' : 'imageWorkbench.history.empty') }}
          </div>

          <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <article v-for="record in visibleRecords" :key="record.id" class="card flex flex-col overflow-hidden">
              <div
                class="relative grid aspect-square gap-0.5 bg-gray-100 dark:bg-dark-800"
                :class="record.images.length > 1 ? 'grid-cols-2' : 'grid-cols-1'"
              >
                <template v-if="record.status === 'completed'">
                  <button
                    v-for="(image, idx) in record.images.slice(0, 4)"
                    :key="idx"
                    type="button"
                    class="overflow-hidden focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
                    :aria-label="t('imageWorkbench.lightbox.open')"
                    @click="openLightbox(record, idx)"
                  >
                    <WorkbenchImage :image="image" :alt="record.prompt" img-class="h-full w-full object-cover transition hover:scale-[1.02]" />
                  </button>
                </template>
                <div v-else-if="record.status === 'pending'" class="flex flex-col items-center justify-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <LoadingSpinner />
                  <span>{{ t('imageWorkbench.history.generating', { seconds: elapsedSeconds(record) }) }}</span>
                  <span v-if="record.transport === 'async'" class="px-4 text-center text-gray-400 dark:text-dark-500">{{ t('imageWorkbench.hints.canLeave') }}</span>
                  <button v-if="store.canCancel(record)" type="button" class="btn btn-secondary btn-sm" @click="store.cancel(record)">
                    {{ t('imageWorkbench.actions.stopWaiting') }}
                  </button>
                </div>
                <div v-else class="flex flex-col items-center justify-center gap-2 p-4 text-center text-xs text-red-500 dark:text-red-400">
                  <span class="font-medium">{{ t(`imageWorkbench.status.${record.status}`) }}</span>
                  <span class="line-clamp-4 break-all">{{ record.error }}</span>
                  <span v-if="record.requestId" class="text-gray-400 dark:text-dark-500">Request ID: {{ record.requestId }}</span>
                </div>
              </div>
              <div class="flex flex-1 flex-col gap-2 p-3">
                <p class="line-clamp-2 text-sm text-gray-700 dark:text-dark-200" :title="record.prompt">{{ record.prompt }}</p>
                <div class="flex flex-wrap items-center gap-1.5 text-[11px] text-gray-400 dark:text-dark-500">
                  <span class="badge badge-gray">{{ t(`imageWorkbench.modes.${record.mode}`) }}</span>
                  <span>{{ record.model }}</span>
                  <span>· {{ record.params.size || record.params.aspect_ratio || 'auto' }}</span>
                  <span>· {{ formatTime(record.createdAt) }}</span>
                </div>
                <div class="mt-auto flex flex-wrap gap-2 pt-1 text-xs">
                  <button type="button" class="text-primary-600 hover:underline dark:text-primary-400" @click="reuse(record)">{{ t('imageWorkbench.actions.reuse') }}</button>
                  <button v-if="record.status === 'completed'" type="button" class="text-primary-600 hover:underline dark:text-primary-400" @click="downloadAll(record)">
                    {{ t('imageWorkbench.actions.download') }}
                  </button>
                  <button type="button" class="ml-auto text-gray-400 hover:text-red-500 dark:text-dark-500" @click="removeRecord(record)">{{ t('imageWorkbench.actions.delete') }}</button>
                </div>
              </div>
            </article>
          </div>
          <div v-if="visibleRecords.length < filteredRecords.length" class="flex justify-center">
            <button type="button" class="btn btn-secondary" @click="pageSize += PAGE_STEP">{{ t('imageWorkbench.history.loadMore') }}</button>
          </div>
        </section>
      </div>
    </div>

    <WorkbenchLightbox
      :record="lightbox.record"
      :initial-index="lightbox.index"
      @close="lightbox.record = null"
      @reuse="(record) => { lightbox.record = null; reuse(record) }"
      @use-reference="useAsReference"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import WorkbenchImage from '@/components/imageWorkbench/WorkbenchImage.vue'
import WorkbenchLightbox from '@/components/imageWorkbench/WorkbenchLightbox.vue'
import { PROMPT_PRESETS, STYLE_PRESETS, type PromptPreset, type StylePreset } from '@/components/imageWorkbench/presets'
import { keysAPI } from '@/api/keys'
import { userGroupsAPI } from '@/api/groups'
import { useAppStore, useAuthStore } from '@/stores'
import { describeGatewayError, useImageWorkbenchStore } from '@/stores/imageWorkbench'
import {
  GROK_ASPECT_RATIOS,
  estimateImageCost,
  findWorkbenchModel,
  formatUsd,
  keySupportsWorkbench,
  modelsForPlatform,
  resolveWorkbenchPlatform,
  type WorkbenchMode,
} from '@/utils/imageWorkbench'
import {
  downloadWorkbenchImage,
  getBlob,
  getStorageUsage,
  isIndexedDbAvailable,
  requestPersistentStorage,
  type StorageUsage,
  type WorkbenchImage as WorkbenchImageData,
  type WorkbenchRecord,
} from '@/utils/imageWorkbenchDb'
import type { ApiKey } from '@/types'

const MAX_REFERENCES = 10
const MAX_REFERENCE_BYTES = 20 * 1024 * 1024
const SOFT_LIMIT_IMAGES = 1000
const SOFT_LIMIT_BYTES = 1024 * 1024 * 1024
const CLEAR_BATCH = 50
const PAGE_STEP = 24
const MAX_PENDING = 4
const FORM_STORAGE_KEY = 'image-workbench:form'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const store = useImageWorkbenchStore()

const dbAvailable = isIndexedDbAvailable()
const loadingKeys = ref(true)
const eligibleKeys = ref<ApiKey[]>([])
const userRates = ref<Record<number, number>>({})
const storage = ref<StorageUsage | null>(null)
const submitting = ref(false)
const submitError = ref('')
const showPresets = ref(false)
const dragging = ref(false)
const filter = ref<'all' | 'completed' | 'pending' | 'failed'>('all')
const search = ref('')
const pageSize = ref(PAGE_STEP)
const now = ref(Date.now())
const fileInput = ref<HTMLInputElement | null>(null)
const maskInput = ref<HTMLInputElement | null>(null)

interface ReferenceItem {
  id: string
  file: File
  preview: string
}
const references = ref<ReferenceItem[]>([])
const mask = ref<File | null>(null)
const lightbox = reactive<{ record: WorkbenchRecord | null; index: number }>({ record: null, index: 0 })

const form = reactive({
  mode: 'generate' as WorkbenchMode,
  apiKeyId: null as number | null,
  model: '',
  prompt: '',
  size: 'auto',
  aspectRatio: '1:1',
  quality: 'auto',
  background: 'auto',
  n: 1,
})

try {
  const saved = JSON.parse(localStorage.getItem(FORM_STORAGE_KEY) || '{}')
  if (saved && typeof saved === 'object') {
    for (const key of ['mode', 'apiKeyId', 'model', 'size', 'aspectRatio', 'quality', 'background', 'n'] as const) {
      if (saved[key] !== undefined) (form as Record<string, unknown>)[key] = saved[key]
    }
  }
} catch {
  // ignore corrupted cache
}

watch(
  () => ({ ...form, prompt: undefined }),
  (value) => localStorage.setItem(FORM_STORAGE_KEY, JSON.stringify(value)),
  { deep: true },
)

const selectedKey = computed(() => eligibleKeys.value.find((key) => key.id === form.apiKeyId))
const platform = computed(() => resolveWorkbenchPlatform(selectedKey.value?.group))
const availableModels = computed(() => modelsForPlatform(platform.value, form.mode))
const currentModel = computed(() => findWorkbenchModel(form.model))
const maxCount = computed(() => currentModel.value?.maxCount ?? 4)

const modeOptions = computed(() => [
  { value: 'generate' as const, label: t('imageWorkbench.modes.generate') },
  { value: 'edit' as const, label: t('imageWorkbench.modes.edit') },
])
const keyOptions = computed<SelectOption[]>(() =>
  eligibleKeys.value.map((key) => ({ value: key.id, label: `${key.name} · ${key.group?.name ?? ''}` })),
)
const modelOptions = computed<SelectOption[]>(() => availableModels.value.map((model) => ({ value: model.id, label: model.label })))
const sizeOptions = computed<SelectOption[]>(() =>
  (currentModel.value?.sizes ?? []).map((size) => ({ value: size, label: size === 'auto' ? t('imageWorkbench.form.auto') : size.replace('x', ' × ') })),
)
const aspectOptions = computed<SelectOption[]>(() => GROK_ASPECT_RATIOS.map((ratio) => ({ value: ratio, label: ratio })))
const qualityOptions = computed<SelectOption[]>(() =>
  ['auto', 'low', 'medium', 'high'].map((value) => ({ value, label: t(`imageWorkbench.quality.${value}`) })),
)
const backgroundOptions = computed<SelectOption[]>(() =>
  ['auto', 'opaque', 'transparent'].map((value) => ({ value, label: t(`imageWorkbench.background.${value}`) })),
)
const filterOptions = computed(() => [
  { value: 'all' as const, label: t('imageWorkbench.history.filterAll') },
  { value: 'completed' as const, label: t('imageWorkbench.status.completed') },
  { value: 'pending' as const, label: t('imageWorkbench.status.pending') },
  { value: 'failed' as const, label: t('imageWorkbench.status.failed') },
])

// 选择变化时保持参数合法
watch([selectedKey, () => form.mode], () => {
  if (!availableModels.value.some((model) => model.id === form.model)) {
    form.model = availableModels.value[0]?.id ?? ''
  }
})
watch(currentModel, (model) => {
  if (model && model.sizes.length && !model.sizes.includes(form.size)) form.size = model.sizes[0]
  if (model && form.n > model.maxCount) form.n = model.maxCount
})

const estimate = computed(() => {
  const group = selectedKey.value?.group
  const override = group ? userRates.value[group.id] : undefined
  const size = platform.value === 'openai' ? form.size : undefined
  return estimateImageCost(group, size, form.n, override)
})

const canSubmit = computed(
  () =>
    dbAvailable &&
    store.loaded &&
    !submitting.value &&
    !!selectedKey.value &&
    !!form.model &&
    form.prompt.trim().length > 0 &&
    (form.mode === 'generate' || references.value.length > 0) &&
    store.pendingRecords.length < MAX_PENDING,
)

async function loadKeys() {
  loadingKeys.value = true
  try {
    const all: ApiKey[] = []
    let page = 1
    while (page <= 10) {
      const response = await keysAPI.list(page, 100, { status: 'active', sort_by: 'created_at', sort_order: 'desc' })
      all.push(...(response.items || []))
      if (page >= response.pages || (response.items || []).length === 0) break
      page += 1
    }
    eligibleKeys.value = all.filter(keySupportsWorkbench)
    if (!eligibleKeys.value.some((key) => key.id === form.apiKeyId)) {
      form.apiKeyId = eligibleKeys.value[0]?.id ?? null
    }
    userRates.value = await userGroupsAPI.getUserGroupRates().catch(() => ({}))
  } catch (error) {
    appStore.showError(describeGatewayError(error))
  } finally {
    loadingKeys.value = false
  }
}

async function refreshStorage() {
  storage.value = await getStorageUsage().catch(() => null)
}

const overSoftLimit = computed(
  () => store.imageCount >= SOFT_LIMIT_IMAGES || (storage.value?.usage ?? 0) >= SOFT_LIMIT_BYTES,
)

const filteredRecords = computed(() => {
  const keyword = search.value.toLowerCase()
  return store.records.filter((record) => {
    if (filter.value === 'completed' && record.status !== 'completed') return false
    if (filter.value === 'pending' && record.status !== 'pending') return false
    if (filter.value === 'failed' && record.status !== 'failed' && record.status !== 'expired') return false
    if (keyword && !record.prompt.toLowerCase().includes(keyword) && !record.model.toLowerCase().includes(keyword)) return false
    return true
  })
})
const visibleRecords = computed(() => filteredRecords.value.slice(0, pageSize.value))

function appendStyle(style: StylePreset) {
  const suffix = t(`imageWorkbench.styles.${style}.prompt`)
  form.prompt = form.prompt.trim() ? `${form.prompt.trim()}${t('imageWorkbench.styles.separator')}${suffix}` : suffix
}

function applyPreset(preset: PromptPreset) {
  form.prompt = t(`imageWorkbench.presets.items.${preset.id}.prompt`)
  if (preset.size && currentModel.value?.sizes.includes(preset.size)) form.size = preset.size
  if (preset.aspectRatio && GROK_ASPECT_RATIOS.includes(preset.aspectRatio)) form.aspectRatio = preset.aspectRatio
  showPresets.value = false
}

function addReferenceFiles(files: File[]) {
  for (const file of files) {
    if (!file.type.startsWith('image/')) continue
    if (file.size > MAX_REFERENCE_BYTES) {
      appStore.showError(t('imageWorkbench.errors.fileTooLarge', { name: file.name }))
      continue
    }
    if (references.value.length >= MAX_REFERENCES) {
      appStore.showError(t('imageWorkbench.errors.tooManyReferences', { max: MAX_REFERENCES }))
      break
    }
    references.value.push({ id: `${Date.now()}-${Math.random()}`, file, preview: URL.createObjectURL(file) })
  }
}

function removeReference(index: number) {
  const [removed] = references.value.splice(index, 1)
  if (removed) URL.revokeObjectURL(removed.preview)
}

function onPickFiles(event: Event) {
  const input = event.target as HTMLInputElement
  addReferenceFiles(Array.from(input.files || []))
  input.value = ''
}

function onPickMask(event: Event) {
  const input = event.target as HTMLInputElement
  mask.value = input.files?.[0] ?? null
  input.value = ''
}

function onDrop(event: DragEvent) {
  dragging.value = false
  addReferenceFiles(Array.from(event.dataTransfer?.files || []))
}

function onPaste(event: ClipboardEvent) {
  if (form.mode !== 'edit') return
  const files = Array.from(event.clipboardData?.files || []).filter((file) => file.type.startsWith('image/'))
  if (files.length) {
    event.preventDefault()
    addReferenceFiles(files)
  }
}

async function submit() {
  if (!canSubmit.value || !selectedKey.value) return
  submitting.value = true
  submitError.value = ''
  try {
    const isOpenAI = platform.value === 'openai'
    await store.submit({
      apiKey: selectedKey.value,
      mode: form.mode,
      model: form.model,
      prompt: form.prompt.trim(),
      params: {
        n: form.n,
        size: isOpenAI ? form.size : undefined,
        quality: currentModel.value?.supportsQuality ? form.quality : undefined,
        background: currentModel.value?.supportsBackground ? form.background : undefined,
        aspect_ratio: isOpenAI ? undefined : form.aspectRatio,
      },
      references: form.mode === 'edit' ? references.value.map((item) => item.file) : [],
      mask: form.mode === 'edit' && isOpenAI ? mask.value : null,
      estimatedCost: estimate.value.total,
    })
    filter.value = 'all'
  } catch (error) {
    submitError.value = describeGatewayError(error)
  } finally {
    submitting.value = false
  }
}

function reuse(record: WorkbenchRecord) {
  form.mode = record.mode
  if (eligibleKeys.value.some((key) => key.id === record.apiKeyId)) form.apiKeyId = record.apiKeyId
  form.prompt = record.prompt
  // 等模型列表按 key/mode 刷新后再回填模型与参数
  setTimeout(() => {
    if (availableModels.value.some((model) => model.id === record.model)) form.model = record.model
    if (record.params.size) form.size = record.params.size
    if (record.params.aspect_ratio) form.aspectRatio = record.params.aspect_ratio
    if (record.params.quality) form.quality = record.params.quality
    if (record.params.background) form.background = record.params.background
    form.n = record.params.n || 1
  })
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function useAsReference(record: WorkbenchRecord, image: WorkbenchImageData) {
  const blob = image.blobId ? await getBlob(image.blobId) : undefined
  if (!blob) {
    appStore.showError(t('imageWorkbench.errors.referenceUnavailable'))
    return
  }
  const ext = blob.type.split('/')[1]?.replace('jpeg', 'jpg') || 'png'
  addReferenceFiles([new File([blob], `reference-${record.createdAt}.${ext}`, { type: blob.type || 'image/png' })])
  form.mode = 'edit'
  lightbox.record = null
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function openLightbox(record: WorkbenchRecord, index: number) {
  lightbox.record = record
  lightbox.index = index
}

async function downloadAll(record: WorkbenchRecord) {
  for (const [idx, image] of record.images.entries()) {
    await downloadWorkbenchImage(image, `${record.model}-${record.createdAt}-${idx + 1}`)
  }
}

async function removeRecord(record: WorkbenchRecord) {
  const message = record.status === 'pending' ? t('imageWorkbench.confirm.deletePending') : t('imageWorkbench.confirm.delete')
  if (!window.confirm(message)) return
  await store.remove([record.id])
  await refreshStorage()
}

async function confirmClearOldest() {
  if (!window.confirm(t('imageWorkbench.confirm.clearOldest', { count: CLEAR_BATCH }))) return
  await store.removeOldest(CLEAR_BATCH)
  await refreshStorage()
}

function elapsedSeconds(record: WorkbenchRecord): number {
  return Math.max(0, Math.round((now.value - record.createdAt) / 1000))
}

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function formatBytes(bytes: number): string {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(1)} MB`
  return `${Math.max(0, Math.round(bytes / 1024))} KB`
}

let clock: ReturnType<typeof setInterval> | null = null

watch(
  () => store.records.map((record) => `${record.id}:${record.status}`).join(','),
  () => void refreshStorage(),
)

onMounted(async () => {
  store.viewActive = true
  clock = setInterval(() => (now.value = Date.now()), 1000)
  document.addEventListener('paste', onPaste)
  if (dbAvailable && authStore.user) {
    void store.init(authStore.user.id)
    void requestPersistentStorage().then(refreshStorage)
    store.pollNow()
  }
  await loadKeys()
})

onBeforeUnmount(() => {
  store.viewActive = false
  if (clock) clearInterval(clock)
  document.removeEventListener('paste', onPaste)
  for (const item of references.value) URL.revokeObjectURL(item.preview)
})
</script>
