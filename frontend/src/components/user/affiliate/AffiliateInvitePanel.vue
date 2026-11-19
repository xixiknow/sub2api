<template>
  <section class="card overflow-hidden">
    <div class="p-6">
      <div class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h3 class="text-base font-semibold text-terra-ink dark:text-white">{{ t('affiliate.title') }}</h3>
          <p class="mt-1 text-sm text-terra-muted dark:text-dark-400">
            {{ t('affiliate.tips.line2', { rate: `${rebateRate}%` }) }}
          </p>
        </div>
      </div>

      <div class="mt-5 grid gap-4 md:grid-cols-2">
        <div class="space-y-2">
          <p class="text-sm font-medium text-terra-ink dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
          <div class="flex flex-col items-stretch gap-2 rounded-xl border border-terra-line bg-terra-surface px-3 py-2 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
            <code class="min-w-0 break-all text-sm font-semibold text-terra-ink dark:text-white sm:flex-1 sm:truncate">{{ code }}</code>
            <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="emit('copy-code')">
              <Icon name="copy" size="sm" />
              <span>{{ t('affiliate.copyCode') }}</span>
            </button>
          </div>
        </div>

        <div class="space-y-2">
          <p class="text-sm font-medium text-terra-ink dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
          <div class="flex flex-col items-stretch gap-2 rounded-xl border border-terra-line bg-terra-surface px-3 py-2 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
            <code class="min-w-0 break-all text-sm text-terra-muted dark:text-gray-300 sm:flex-1 sm:truncate">{{ link }}</code>
            <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="emit('copy-link')">
              <Icon name="copy" size="sm" />
              <span>{{ t('affiliate.copyLink') }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="border-t border-terra-line bg-terra-surface/60 p-6 dark:border-dark-700 dark:bg-dark-900/40">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <h4 class="text-sm font-semibold text-terra-ink dark:text-white">{{ t('affiliate.rebateGuide.progressTitle') }}</h4>
        <span class="inline-flex w-fit rounded-md bg-primary-50 px-2.5 py-1 text-xs font-semibold text-primary-800 ring-1 ring-primary-100 dark:bg-primary-900/25 dark:text-primary-200 dark:ring-primary-800/50">
          <template v-if="nextLocked">
            {{ t('affiliate.rebateGuide.nextGoal', { level: nextLocked.level, count: formatCount(nextLocked.remaining) }) }}
          </template>
          <template v-else>{{ t('affiliate.rebateGuide.allUnlocked') }}</template>
        </span>
      </div>

      <div class="mt-4 grid gap-3 md:grid-cols-3">
        <div
          v-for="row in rows"
          :key="row.level"
          class="rounded-lg border bg-terra-elevated p-4 dark:bg-dark-800"
          :class="row.unlocked ? 'border-terra-line dark:border-dark-700' : 'border-dashed border-terra-line dark:border-dark-600'"
        >
          <div class="flex items-center justify-between gap-2">
            <p class="text-sm font-semibold text-terra-ink dark:text-white">{{ row.title }}</p>
            <span
              class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[11px] font-semibold"
              :class="row.unlocked
                ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-900/25 dark:text-emerald-200 dark:ring-emerald-800/50'
                : 'bg-amber-50 text-amber-700 ring-1 ring-amber-200 dark:bg-amber-900/25 dark:text-amber-200 dark:ring-amber-800/50'"
            >
              <Icon :name="row.unlocked ? 'checkCircle' : 'lock'" size="xs" />
              {{ row.statusLabel }}
            </span>
          </div>
          <div class="mt-3 flex items-end justify-between gap-2">
            <div>
              <p class="text-2xl font-semibold text-primary-700 dark:text-primary-300">{{ row.rateLabel }}</p>
              <p class="text-[11px] text-terra-muted dark:text-dark-400">{{ sourceLabel(row.source) }}</p>
            </div>
            <div class="text-right">
              <p class="text-[11px] text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.total') }}</p>
              <p class="text-sm font-semibold text-terra-ink dark:text-dark-100">{{ formatCurrency(row.rebateAmount) }}</p>
            </div>
          </div>
          <div class="mt-3">
            <div class="flex items-center justify-between text-[11px] text-terra-muted dark:text-dark-400">
              <span class="truncate">{{ row.level <= 1 ? t('affiliate.rebateGuide.alwaysOnProgress') : t('affiliate.rebateGuide.inviteProgress') }}</span>
              <span class="shrink-0">{{ row.progressText }}</span>
            </div>
            <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
              <div
                class="h-full rounded-full transition-all"
                :class="row.unlocked ? 'bg-emerald-500' : 'bg-primary-500'"
                :style="{ width: `${row.progressPercent}%` }"
              ></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatCurrency } from '@/utils/format'
import { formatCount, type UnlockProgressRow } from './useAffiliateLevels'

defineProps<{
  code: string
  link: string
  rebateRate: string
  rows: UnlockProgressRow[]
  nextLocked: UnlockProgressRow | null
  sourceLabel: (source: string) => string
}>()

const emit = defineEmits<{
  (e: 'copy-code'): void
  (e: 'copy-link'): void
}>()

const { t } = useI18n()
</script>
