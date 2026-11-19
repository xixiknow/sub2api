<template>
  <section class="card overflow-hidden">
    <button
      type="button"
      class="flex w-full items-center justify-between gap-3 px-6 py-4 text-left transition hover:bg-terra-surface/70 dark:hover:bg-dark-800/60"
      :aria-expanded="open"
      data-test="rules-toggle"
      @click="open = !open"
    >
      <span class="inline-flex items-center gap-2 text-sm font-semibold text-terra-ink dark:text-white">
        <Icon name="lightbulb" size="sm" class="text-primary-500" />
        {{ t('affiliate.rebateGuide.title') }}
      </span>
      <span class="inline-flex items-center gap-1 text-xs font-medium text-terra-muted dark:text-dark-400">
        {{ open ? t('affiliate.rules.toggleHide') : t('affiliate.rules.toggleShow') }}
        <Icon name="chevronDown" size="sm" class="transition-transform" :class="open ? 'rotate-180' : ''" />
      </span>
    </button>

    <div v-if="open" class="border-t border-terra-line p-6 dark:border-dark-700">
      <div class="grid gap-6 lg:grid-cols-2">
        <div>
          <div class="flex items-center justify-between gap-3">
            <h4 class="text-sm font-semibold text-terra-ink dark:text-white">{{ t('affiliate.rebateGuide.mechanicsTitle') }}</h4>
            <span class="text-xs text-terra-muted dark:text-dark-400">{{ t('affiliate.rebateGuide.autoSettle') }}</span>
          </div>
          <ol class="mt-4 space-y-4">
            <li v-for="row in rows" :key="row.level" class="flex gap-3">
              <span
                class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-sm font-bold"
                :class="row.unlocked
                  ? 'bg-primary-50 text-primary-700 ring-1 ring-primary-100 dark:bg-primary-900/25 dark:text-primary-200 dark:ring-primary-800/50'
                  : 'bg-gray-100 text-gray-500 ring-1 ring-gray-200 dark:bg-dark-800 dark:text-dark-400 dark:ring-dark-700'"
              >{{ row.level }}</span>
              <div class="min-w-0">
                <p class="text-sm font-medium text-terra-ink dark:text-white">{{ mechanismTitle(row.level) }}</p>
                <p class="mt-1 text-sm leading-6 text-terra-muted dark:text-dark-400">{{ mechanismDescription(row.level) }}</p>
                <p class="mt-1 text-xs text-terra-muted dark:text-dark-500">{{ row.requirementLabel }}</p>
              </div>
            </li>
          </ol>
        </div>

        <div class="space-y-4">
          <div class="rounded-lg border border-terra-line bg-terra-surface p-4 dark:border-dark-700 dark:bg-dark-900/70">
            <p class="text-sm font-semibold text-terra-ink dark:text-white">{{ t('affiliate.rebateGuide.flowTitle') }}</p>
            <ol class="mt-3 grid gap-2 sm:grid-cols-4">
              <li
                v-for="(step, index) in flowSteps"
                :key="step"
                class="rounded-md bg-terra-elevated px-3 py-2 text-xs font-medium text-terra-muted shadow-glass-sm dark:bg-dark-800 dark:text-dark-300"
              >
                <span class="mr-1 font-semibold text-primary-600 dark:text-primary-300">{{ index + 1 }}.</span>{{ step }}
              </li>
            </ol>
          </div>
          <ul class="space-y-2 text-sm leading-6 text-terra-muted dark:text-dark-400">
            <li class="flex gap-2"><Icon name="checkCircle" size="sm" class="mt-1 shrink-0 text-emerald-500" />{{ t('affiliate.tips.line1') }}</li>
            <li class="flex gap-2"><Icon name="checkCircle" size="sm" class="mt-1 shrink-0 text-emerald-500" />{{ t('affiliate.rebateGuide.progressSubtitle') }}</li>
            <li class="flex gap-2"><Icon name="checkCircle" size="sm" class="mt-1 shrink-0 text-emerald-500" />{{ t('affiliate.tips.line3') }}</li>
            <li class="flex gap-2"><Icon name="checkCircle" size="sm" class="mt-1 shrink-0 text-emerald-500" />{{ t('affiliate.tips.line4') }}</li>
          </ul>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UnlockProgressRow } from './useAffiliateLevels'

defineProps<{ rows: UnlockProgressRow[] }>()

const { t } = useI18n()
const open = ref(false)

const flowSteps = computed(() => [
  t('affiliate.rebateGuide.flowInvite'),
  t('affiliate.rebateGuide.flowRecharge'),
  t('affiliate.rebateGuide.flowFreeze'),
  t('affiliate.rebateGuide.flowTransfer'),
])

function mechanismTitle(level: number): string {
  return level >= 1 && level <= 3
    ? t(`affiliate.rebateGuide.level${level}Title`)
    : t('affiliate.levelRules.levelTitle', { level })
}

function mechanismDescription(level: number): string {
  return level >= 1 && level <= 3 ? t(`affiliate.rebateGuide.level${level}Description`) : ''
}
</script>
