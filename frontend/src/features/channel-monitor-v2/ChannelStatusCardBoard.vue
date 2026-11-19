<template>
  <div class="space-y-5 pb-12">
    <section
      class="card !rounded-3xl !border-0 p-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
    >
      <header class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
        <div class="min-w-0">
          <h1 class="flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
            <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400">
              <Icon name="chart" size="sm" />
            </span>
            {{ t('channelMonitorV2.title') }}
          </h1>
          <div class="mt-1.5 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
            <span class="inline-flex h-2 w-2 rounded-full" :class="loading ? 'bg-gray-400' : 'bg-green-500'"></span>
            <span v-if="loading" class="inline-flex items-center gap-1 text-primary-600 dark:text-primary-300">
              <LoadingSpinner size="sm" />
              {{ t('channelMonitorV2.updating') }}
            </span>
            <span v-else-if="matrix?.coverage.data_through">
              {{ t('channelMonitorV2.updatedTo', { time: formatTime(matrix.coverage.data_through) }) }}
            </span>
            <span v-if="bootstrapPercent != null" class="badge badge-primary inline-flex items-center gap-1">
              {{ t('channelMonitorV2.bootstrap.progress', { percent: bootstrapPercent }) }}
            </span>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <slot name="actions" />
          <button
            class="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600"
            type="button"
            :title="t('common.refresh')"
            :disabled="loading"
            @click="reload()"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </header>

      <div class="px-5 py-4 sm:px-6">
        <StatusBoardToolbar
          v-model:platform="platform"
          v-model:search="search"
          v-model:range="range"
          v-model:sort="sort"
          :counts="counts"
          :ranges="ranges"
          :sort-keys="sortKeys"
        />
      </div>
    </section>

    <StatusCardGrid
      :cards="visibleCards"
      :loading="loading && cards.length === 0"
      :empty-title="cards.length ? t('channelStatusBoard.empty.noMatchTitle') : t('channelMonitorV2.empty.title')"
      :empty-description="cards.length ? t('channelStatusBoard.empty.noMatchDescription') : t('channelMonitorV2.empty.description')"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import StatusBoardToolbar from '@/components/user/monitor/status-board/StatusBoardToolbar.vue'
import StatusCardGrid from '@/components/user/monitor/status-board/StatusCardGrid.vue'
import { useStatusBoard } from '@/components/user/monitor/status-board/useStatusBoard'
import type { BoardSortKey, StatusCardModel } from '@/components/user/monitor/status-board/types'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { userGroupsAPI } from '@/api/groups'
import * as adminGroupsAPI from '@/api/admin/groups'
import * as api from '@/api/channelMonitorV2'
import type { MonitorMatrixResponse, MonitorRange } from '@/api/channelMonitorV2'
import { buildV2Cards, type GroupRateInfo } from './statusBoardMapping'

type CardRange = Extract<MonitorRange, '24h' | '7d' | '30d'>
const CARD_RANGES: CardRange[] = ['24h', '7d', '30d']
const AUTO_REFRESH_SECONDS = 60

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const { providerLabel } = useChannelMonitorFormat()
const isAdmin = computed(() => authStore.isAdmin)

const range = ref<CardRange>(
  CARD_RANGES.includes(route.query.range as CardRange) ? (route.query.range as CardRange) : '24h',
)
const ranges = computed(() => CARD_RANGES.map((value) => ({ value, label: t(`channelMonitorV2.ranges.${value}`) })))
const sortKeys: BoardSortKey[] = ['default', 'rate', 'success', 'cache', 'latency', 'status']

const matrix = ref<MonitorMatrixResponse | null>(null)
const rates = ref<Record<number, GroupRateInfo>>({})
const loading = ref(false)
let controller: AbortController | null = null
let timer: number | null = null

const cards = computed<StatusCardModel[]>(() =>
  matrix.value
    ? buildV2Cards(matrix.value.items || [], matrix.value.coverage, {
        t: (key, params) => (params ? t(key, params) : t(key)),
        rates: rates.value,
        platformLabel: (platform) => providerLabel(platform),
      })
    : [],
)
const { platform, search, sort, counts, visibleCards } = useStatusBoard(cards)

const bootstrapPercent = computed(() => {
  const bootstrap = matrix.value?.coverage?.bootstrap
  if (!bootstrap?.active) return null
  return Math.min(100, Math.max(0, Math.round(bootstrap.progress_percent || 0)))
})

async function reload() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  try {
    const next = await api.getMatrix(
      { range: range.value, platforms: [], groupIds: [], models: [] },
      'platform_group',
      isAdmin.value,
      request.signal,
    )
    if (controller !== request) return
    matrix.value = next
  } catch (error) {
    const e = error as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.name === 'CanceledError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.loadFailed')))
  } finally {
    if (controller === request) {
      loading.value = false
      controller = null
    }
  }
}

// 倍率仅用于展示：失败时静默降级为不显示倍率。管理员取全部分组，普通用户取可用分组。
async function loadRates() {
  try {
    const [groups, userRates] = await Promise.all([
      isAdmin.value ? adminGroupsAPI.getAll() : userGroupsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch(() => ({}) as Record<number, number>),
    ])
    const next: Record<number, GroupRateInfo> = {}
    for (const group of groups) {
      if (group.rate_multiplier == null) continue
      next[group.id] = { base: group.rate_multiplier, user: userRates[group.id] ?? null }
    }
    rates.value = next
  } catch {
    rates.value = {}
  }
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat(locale.value || undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

watch(range, (value) => {
  void router.replace({ query: { ...route.query, range: value } })
  void reload()
})

onMounted(() => {
  void reload()
  void loadRates()
  timer = window.setInterval(() => {
    if (!document.hidden && !loading.value) void reload()
  }, AUTO_REFRESH_SECONDS * 1000)
})

onBeforeUnmount(() => {
  controller?.abort()
  if (timer) window.clearInterval(timer)
})

defineExpose({ reload })
</script>
