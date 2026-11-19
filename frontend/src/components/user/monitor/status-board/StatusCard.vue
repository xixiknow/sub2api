<template>
  <component
    :is="clickable ? 'button' : 'div'"
    :type="clickable ? 'button' : undefined"
    class="status-card flex w-full flex-col rounded-2xl border border-gray-200/70 bg-white p-5 text-left shadow-sm transition-all duration-200 dark:border-dark-700/70 dark:bg-dark-800"
    :class="clickable ? 'cursor-pointer hover:-translate-y-0.5 hover:border-primary-200 hover:shadow-md dark:hover:border-primary-500/40' : ''"
    @click="clickable && emit('click')"
  >
    <!-- Header -->
    <div class="flex items-start gap-3">
      <span
        class="grid h-11 w-11 flex-shrink-0 place-items-center rounded-xl bg-gray-50 text-gray-900 ring-1 ring-gray-900/5 dark:bg-dark-700 dark:text-gray-100 dark:ring-white/10"
      >
        <ProviderIcon :provider="card.platform as Provider" :size="22" />
      </span>
      <div class="min-w-0 flex-1">
        <div class="truncate text-[15px] font-bold text-gray-900 dark:text-white" :title="card.title">
          {{ card.title }}
        </div>
        <div class="mt-1 flex min-w-0 items-center gap-2 text-xs">
          <span class="truncate text-gray-500 dark:text-gray-400">{{ card.platformLabel }}</span>
          <span
            v-if="card.rate != null"
            class="inline-flex flex-shrink-0 items-center gap-1 rounded-md bg-violet-50 px-1.5 py-0.5 font-semibold text-violet-600 dark:bg-violet-500/15 dark:text-violet-300"
            :title="card.rateTitle"
          >
            {{ t('channelStatusBoard.rate') }}
            <span v-if="hasCustomRate" class="font-normal line-through opacity-50">x{{ formatBoardRate(card.baseRate) }}</span>
            <span>x{{ formatBoardRate(card.rate) }}</span>
          </span>
        </div>
      </div>
      <span class="inline-flex flex-shrink-0 items-center gap-1.5 text-xs font-semibold" :class="statusTextClass">
        <span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass"></span>
        {{ t(`channelStatusBoard.status.${card.status}`) }}
      </span>
    </div>

    <!-- Metrics -->
    <div class="mt-5 grid grid-cols-3 border-t border-gray-100 pt-4 dark:border-dark-700/70">
      <div
        v-for="(metric, index) in card.metrics"
        :key="metric.label"
        class="min-w-0"
        :class="index > 0 ? 'border-l border-gray-100 pl-4 dark:border-dark-700/70' : 'pr-4'"
        :title="metric.title"
      >
        <div class="truncate text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
        <div class="mt-1 flex items-baseline gap-0.5 whitespace-nowrap">
          <span class="text-2xl font-bold tabular-nums tracking-tight text-gray-900 dark:text-white">{{ metric.value }}</span>
          <span v-if="metric.unit" class="text-xs font-medium text-gray-400">{{ metric.unit }}</span>
        </div>
        <div v-if="metric.hint" class="mt-0.5 truncate text-[11px] text-gray-400 dark:text-dark-400">{{ metric.hint }}</div>
      </div>
    </div>

    <slot />

    <!-- History bars -->
    <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700/70">
      <StatusBars :bars="card.bars" :label="card.barsLabel" />
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Provider } from '@/api/admin/channelMonitor'
import ProviderIcon from '../ProviderIcon.vue'
import StatusBars from './StatusBars.vue'
import { formatBoardRate } from './format'
import type { StatusCardModel } from './types'

const props = withDefaults(
  defineProps<{
    card: StatusCardModel
    clickable?: boolean
  }>(),
  { clickable: false },
)

const emit = defineEmits<{
  (e: 'click'): void
}>()

const { t } = useI18n()

const hasCustomRate = computed(
  () => props.card.baseRate != null && props.card.rate != null && props.card.baseRate !== props.card.rate,
)

const statusTextClass = computed(() => {
  switch (props.card.status) {
    case 'operational':
      return 'text-emerald-600 dark:text-emerald-400'
    case 'degraded':
      return 'text-amber-600 dark:text-amber-400'
    case 'failed':
      return 'text-red-600 dark:text-red-400'
    default:
      return 'text-gray-400'
  }
})

const statusDotClass = computed(() => {
  switch (props.card.status) {
    case 'operational':
      return 'bg-emerald-500'
    case 'degraded':
      return 'bg-amber-500'
    case 'failed':
      return 'bg-red-500'
    default:
      return 'bg-gray-400'
  }
})
</script>
