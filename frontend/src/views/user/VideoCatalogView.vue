<template>
  <component :is="embedded ? 'div' : AppLayout">
    <div :class="embedded ? 'space-y-6' : 'mx-auto max-w-6xl space-y-6'">
      <div class="sticky top-0 z-10 -mx-1 flex flex-wrap items-center gap-2 rounded-xl border border-gray-200 bg-white/90 px-3 py-2 backdrop-blur dark:border-dark-700 dark:bg-dark-900/90">
        <a href="#video-prices" class="btn btn-secondary btn-sm">{{ t('videoCatalog.prices') }}</a>
        <a href="#video-docs" class="btn btn-secondary btn-sm">{{ t('videoCatalog.docs') }}</a>
      </div>

      <section id="video-prices" class="space-y-4 scroll-mt-24">
        <p class="text-sm text-gray-600 dark:text-dark-300">{{ t('videoCatalog.billingNote') }}</p>

        <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex flex-wrap gap-2">
            <button
              v-for="option in billingFilters"
              :key="option.value"
              type="button"
              class="btn btn-sm"
              :class="billingFilter === option.value ? 'btn-primary' : 'btn-secondary'"
              @click="billingFilter = option.value"
            >
              {{ option.label }}
            </button>
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="option in seriesFilters"
              :key="option.value"
              type="button"
              class="btn btn-sm"
              :class="seriesFilter === option.value ? 'btn-primary' : 'btn-secondary'"
              @click="seriesFilter = option.value"
            >
              {{ option.label }}
            </button>
          </div>
        </div>

        <div v-if="loading" class="card px-5 py-8 text-sm text-gray-500 dark:text-dark-400">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="families.length === 0" class="card px-5 py-8 text-sm text-gray-500 dark:text-dark-400">
          {{ t('videoCatalog.empty') }}
        </div>
        <div v-else class="grid gap-4">
          <article
            v-for="family in families"
            :key="family.family"
            class="card overflow-hidden"
          >
            <div class="flex flex-col gap-4 p-5 sm:flex-row sm:items-start sm:justify-between">
              <div class="min-w-0 space-y-3">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="font-mono text-base font-semibold text-gray-900 dark:text-white">{{ family.family }}</h2>
                  <span
                    class="rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="family.billing_unit === 'per_second'
                      ? 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300'
                      : 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'"
                  >
                    {{ family.billing_unit === 'per_second' ? t('videoCatalog.perSecond') : t('videoCatalog.perClip') }}
                  </span>
                  <span
                    v-for="tag in family.tags || []"
                    :key="tag"
                    class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300"
                  >
                    {{ tag }}
                  </span>
                </div>
                <div class="flex flex-wrap gap-2">
                  <span
                    v-for="price in family.prices"
                    :key="price.resolution"
                    class="rounded-lg bg-gray-50 px-2.5 py-1 font-mono text-sm text-gray-800 dark:bg-dark-800 dark:text-gray-200"
                  >
                    {{ formatPriceChip(family, price) }}
                  </span>
                </div>
                <p class="text-sm text-gray-600 dark:text-dark-300">
                  {{ durationLabel(family) }} · {{ t('videoCatalog.aspect') }} {{ family.aspect_ratios.join(' / ') }}
                  · {{ refsLabel(family) }}
                  · {{ t('videoCatalog.firstLast') }} {{ family.allow_first_last ? t('videoCatalog.supported') : t('videoCatalog.unsupported') }}
                </p>
              </div>
              <div class="flex shrink-0 flex-wrap gap-2">
                <button type="button" class="btn btn-secondary btn-sm" @click="toggleExpanded(family.family)">
                  {{ expanded === family.family ? t('videoCatalog.collapse') : t('videoCatalog.details') }}
                </button>
                <button type="button" class="btn btn-primary btn-sm" @click="scrollToDocs(family.family)">
                  {{ t('videoCatalog.viewCall') }}
                </button>
              </div>
            </div>
            <div
              v-if="expanded === family.family"
              class="border-t border-gray-100 bg-gray-50 px-5 py-4 text-sm dark:border-dark-700 dark:bg-dark-800/60"
            >
              <dl class="grid gap-3 sm:grid-cols-2">
                <div>
                  <dt class="text-xs uppercase tracking-wide text-gray-500 dark:text-dark-400">{{ t('videoCatalog.createPath') }}</dt>
                  <dd class="mt-1 font-mono">POST {{ family.create_path }}</dd>
                </div>
                <div>
                  <dt class="text-xs uppercase tracking-wide text-gray-500 dark:text-dark-400">{{ t('videoCatalog.duration') }}</dt>
                  <dd class="mt-1">{{ durationLabel(family) }}</dd>
                </div>
                <div class="sm:col-span-2">
                  <dt class="text-xs uppercase tracking-wide text-gray-500 dark:text-dark-400">{{ t('videoCatalog.refs') }}</dt>
                  <dd class="mt-1">{{ refsLabel(family) }}</dd>
                </div>
              </dl>
            </div>
          </article>
        </div>
      </section>

      <VideoCatalogDocs ref="docsRef" :families="allFamilies" />
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getVideoCatalog, type VideoCatalogFamily, type VideoCatalogPrice } from '@/api/videoCatalog'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import VideoCatalogDocs from './VideoCatalogDocs.vue'

defineProps<{
  embedded?: boolean
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const allFamilies = ref<VideoCatalogFamily[]>([])
const billingFilter = ref<'all' | 'per_second' | 'per_clip'>('all')
const seriesFilter = ref('all')
const expanded = ref<string | null>(null)
const docsRef = ref<{ scrollToFamily: (family: string) => void } | null>(null)

const billingFilters = computed(() => [
  { value: 'all' as const, label: t('videoCatalog.filterAll') },
  { value: 'per_second' as const, label: t('videoCatalog.filterPerSecond') },
  { value: 'per_clip' as const, label: t('videoCatalog.filterPerClip') }
])

const seriesFilters = computed(() => [
  { value: 'all', label: t('videoCatalog.seriesAll') },
  { value: 'minimax', label: 'minimax' },
  { value: 'A', label: 'A' },
  { value: 'B', label: 'B' },
  { value: 'C', label: 'C' },
  { value: 'E', label: 'E' },
  { value: 'F', label: 'F' },
  { value: '2.5', label: '2.5' }
])

const families = computed(() =>
  allFamilies.value.filter((family) => {
    if (billingFilter.value !== 'all' && family.billing_unit !== billingFilter.value) return false
    if (seriesFilter.value !== 'all' && familySeries(family.family) !== seriesFilter.value) return false
    return true
  })
)

onMounted(async () => {
  try {
    const catalog = await getVideoCatalog()
    allFamilies.value = catalog?.families || []
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('videoCatalog.loadFailed')))
  } finally {
    loading.value = false
  }
})

function familySeries(name: string): string {
  if (name.startsWith('minimax')) return 'minimax'
  if (name.includes('2.5')) return '2.5'
  if (name.includes('2.0-C')) return 'C'
  if (name.includes('2.0-E')) return 'E'
  if (name.includes('2.0-F') || name.includes('fast-F')) return 'F'
  if (name.includes('2.0-B') || name.includes('fast-B')) return 'B'
  return 'A'
}

function formatPriceChip(family: VideoCatalogFamily, price: VideoCatalogPrice): string {
  const amount = Number.isInteger(price.price) ? String(price.price) : price.price.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')
  const unit = family.billing_unit === 'per_second' ? '/s' : t('videoCatalog.perClipUnit')
  const label = price.resolution === '4k' ? '4K' : price.resolution
  return `${label} $${amount}${unit}`
}

function durationLabel(family: VideoCatalogFamily): string {
  if (family.fixed_duration) {
    return `${t('videoCatalog.duration')} ${t('videoCatalog.durationFixed', { value: family.fixed_duration })}`
  }
  const range = t('videoCatalog.durationRange', { min: family.min_duration, max: family.max_duration })
  const extras: string[] = []
  if (family.duration_required) extras.push(t('videoCatalog.durationRequired'))
  if (family.default_duration) extras.push(t('videoCatalog.durationDefault', { value: family.default_duration }))
  return extras.length
    ? `${t('videoCatalog.duration')} ${range} (${extras.join(', ')})`
    : `${t('videoCatalog.duration')} ${range}`
}

function refsLabel(family: VideoCatalogFamily): string {
  return t('videoCatalog.refsSummary', {
    images: family.max_images,
    videos: family.max_videos,
    audios: family.max_audios,
    total: family.max_references
  })
}

function toggleExpanded(family: string) {
  expanded.value = expanded.value === family ? null : family
}

function scrollToDocs(family: string) {
  document.getElementById('video-docs')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  window.setTimeout(() => docsRef.value?.scrollToFamily(family), 50)
}
</script>
