<template>
  <section class="space-y-3">
    <!-- Row 1: platform tabs + search -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex min-w-0 flex-wrap items-center gap-1.5" role="tablist" :aria-label="t('channelStatusBoard.platformAria')">
        <button
          v-for="tab in platformTabs"
          :key="tab"
          type="button"
          role="tab"
          :aria-selected="platform === tab"
          class="inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-sm transition-colors"
          :class="platform === tab
            ? 'bg-primary-50 font-semibold text-primary-600 ring-1 ring-primary-200 dark:bg-primary-500/15 dark:text-primary-300 dark:ring-primary-500/30'
            : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
          @click="emit('update:platform', tab)"
        >
          <span v-if="tab !== 'all' && tab !== 'other'" class="inline-flex text-gray-700 dark:text-gray-200">
            <ProviderIcon :provider="tab as Provider" :size="15" />
          </span>
          <span v-else-if="tab === 'other'" class="text-[10px] text-gray-400">◇</span>
          {{ t(`channelStatusBoard.platforms.${tab}`) }}
          <span class="text-xs tabular-nums" :class="platform === tab ? 'text-primary-500' : 'text-gray-400'">
            {{ counts[tab] ?? 0 }}
          </span>
        </button>
      </div>

      <label
        class="relative flex w-full items-center rounded-xl border border-gray-200 bg-white px-3 py-1.5 text-sm focus-within:border-primary-300 focus-within:ring-2 focus-within:ring-primary-100 dark:border-dark-700 dark:bg-dark-800 dark:focus-within:ring-primary-500/20 sm:w-72"
      >
        <Icon name="search" size="sm" class="mr-2 flex-shrink-0 text-gray-400" />
        <input
          ref="searchRef"
          :value="search"
          type="search"
          class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-white"
          :placeholder="t('channelStatusBoard.searchPlaceholder')"
          @input="emit('update:search', ($event.target as HTMLInputElement).value)"
          @keydown.esc="($event.target as HTMLInputElement).blur()"
        />
        <kbd
          class="ml-2 flex-shrink-0 rounded border border-gray-200 bg-gray-50 px-1.5 text-[11px] font-medium text-gray-400 dark:border-dark-600 dark:bg-dark-700"
        >/</kbd>
      </label>
    </div>

    <!-- Row 2: range + sort -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('channelStatusBoard.timeRange') }}</span>
        <div class="inline-flex rounded-xl bg-gray-100/80 p-0.5 dark:bg-dark-800" role="group">
          <button
            v-for="option in ranges"
            :key="option.value"
            type="button"
            class="rounded-lg px-3 py-1 text-xs transition-colors"
            :class="range === option.value
              ? 'bg-white font-semibold text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
            @click="emit('update:range', option.value)"
          >
            {{ option.label }}
          </button>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-1">
        <span class="mr-1 text-xs text-gray-500 dark:text-gray-400">{{ t('channelStatusBoard.sortLabel') }}</span>
        <button
          v-for="key in sortKeys"
          :key="key"
          type="button"
          class="inline-flex items-center gap-0.5 rounded-lg px-2.5 py-1 text-xs transition-colors"
          :class="sort.key === key
            ? 'bg-primary-50 font-semibold text-primary-600 ring-1 ring-primary-200 dark:bg-primary-500/15 dark:text-primary-300 dark:ring-primary-500/30'
            : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-700'"
          @click="emit('update:sort', nextBoardSort(sort, key))"
        >
          {{ t(`channelStatusBoard.sort.${key}`) }}
          <span v-if="key !== 'default'" aria-hidden="true">{{ arrow(key) }}</span>
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts" generic="R extends string">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Provider } from '@/api/admin/channelMonitor'
import Icon from '@/components/icons/Icon.vue'
import ProviderIcon from '../ProviderIcon.vue'
import { nextBoardSort } from './useStatusBoard'
import {
  BOARD_PLATFORM_TABS,
  BOARD_SORT_DEFAULT_DIR,
  type BoardPlatformFilter,
  type BoardSort,
  type BoardSortKey,
} from './types'

const props = defineProps<{
  platform: BoardPlatformFilter
  counts: Record<BoardPlatformFilter, number>
  search: string
  range: R
  ranges: Array<{ value: R; label: string }>
  sort: BoardSort
  sortKeys: BoardSortKey[]
}>()

const emit = defineEmits<{
  (e: 'update:platform', value: BoardPlatformFilter): void
  (e: 'update:search', value: string): void
  (e: 'update:range', value: R): void
  (e: 'update:sort', value: BoardSort): void
}>()

const { t } = useI18n()
const searchRef = ref<HTMLInputElement | null>(null)
const platformTabs: BoardPlatformFilter[] = ['all', ...BOARD_PLATFORM_TABS]

function arrow(key: BoardSortKey): string {
  const dir = props.sort.key === key ? props.sort.dir : BOARD_SORT_DEFAULT_DIR[key]
  return dir === 'asc' ? '↑' : '↓'
}

// "/" focuses the search box unless the user is already typing somewhere.
function onKeydown(event: KeyboardEvent) {
  if (event.key !== '/' || event.ctrlKey || event.metaKey || event.altKey) return
  const target = event.target as HTMLElement | null
  const tag = target?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target?.isContentEditable) return
  event.preventDefault()
  searchRef.value?.focus()
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>
