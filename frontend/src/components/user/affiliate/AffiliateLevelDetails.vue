<template>
  <section class="card p-6">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
      <div>
        <h3 class="text-base font-semibold text-terra-ink dark:text-white">{{ t('affiliate.levelDetails.title') }}</h3>
        <p class="mt-1 text-sm text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.subtitle') }}</p>
      </div>

      <div class="grid gap-2 rounded-lg border border-terra-line bg-terra-surface p-1 dark:border-dark-700 dark:bg-dark-900 sm:grid-cols-3">
        <button
          v-for="item in levelDetails"
          :key="item.level"
          type="button"
          class="min-w-0 rounded-md px-3 py-2 text-left transition"
          :class="activeLevel === item.level
            ? 'bg-terra-elevated text-primary-700 shadow-glass-sm dark:bg-dark-800 dark:text-primary-200'
            : 'text-terra-muted hover:bg-terra-elevated/70 dark:text-dark-300 dark:hover:bg-dark-800/60'"
          @click="activeLevel = item.level"
        >
          <span class="block text-xs font-semibold">{{ t('affiliate.levelDetails.levelTab', { level: item.level }) }}</span>
          <span class="mt-0.5 block truncate text-sm font-semibold">{{ formatCurrency(item.total_rebate) }}</span>
        </button>
      </div>
    </div>

    <dl class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
        <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.invitees') }}</dt>
        <dd class="mt-1 text-lg font-semibold text-terra-ink dark:text-white">{{ formatCount(selected.invitee_count) }}</dd>
      </div>
      <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
        <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.total') }}</dt>
        <dd class="mt-1 text-lg font-semibold text-primary-700 dark:text-primary-300">{{ formatCurrency(selected.total_rebate) }}</dd>
      </div>
      <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
        <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.frozen') }}</dt>
        <dd class="mt-1 text-lg font-semibold text-amber-600 dark:text-amber-300">{{ formatCurrency(selected.frozen_rebate) }}</dd>
      </div>
      <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
        <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.available') }}</dt>
        <dd class="mt-1 text-lg font-semibold text-emerald-700 dark:text-emerald-300">{{ formatCurrency(selected.available_rebate) }}</dd>
      </div>
    </dl>

    <div v-if="selected.invitees.length === 0" class="mt-5 rounded-lg border border-dashed border-terra-line p-6 text-center text-sm text-terra-muted dark:border-dark-700 dark:text-dark-400">
      {{ t('affiliate.levelDetails.empty', { level: t('affiliate.levelDetails.levelLabel', { level: selected.level }) }) }}
    </div>

    <template v-else>
      <div class="mt-5 hidden overflow-x-auto md:block">
        <table class="w-full min-w-[920px] text-left text-sm">
          <thead>
            <tr class="border-b border-terra-line text-xs uppercase tracking-wide text-terra-muted dark:border-dark-700 dark:text-dark-400">
              <th class="px-3 py-2 font-semibold">{{ t('affiliate.levelDetails.columns.user') }}</th>
              <th class="px-3 py-2 font-semibold">{{ t('affiliate.levelDetails.columns.chain') }}</th>
              <th class="px-3 py-2 font-semibold">{{ t('affiliate.levelDetails.columns.joinedAt') }}</th>
              <th class="px-3 py-2 text-right font-semibold">{{ t('affiliate.levelDetails.columns.orders') }}</th>
              <th class="px-3 py-2 text-right font-semibold">{{ t('affiliate.levelDetails.columns.total') }}</th>
              <th class="px-3 py-2 text-right font-semibold">{{ t('affiliate.levelDetails.columns.frozen') }}</th>
              <th class="px-3 py-2 text-right font-semibold">{{ t('affiliate.levelDetails.columns.available') }}</th>
              <th class="px-3 py-2 font-semibold">{{ t('affiliate.levelDetails.columns.lastRebate') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in selected.invitees"
              :key="`${selected.level}-${item.user_id}`"
              class="border-b border-terra-line/70 last:border-b-0 dark:border-dark-800"
            >
              <td class="px-3 py-3">
                <p class="max-w-[220px] truncate font-medium text-terra-ink dark:text-white">{{ item.email || fallbackUserLabel(item) }}</p>
                <p class="max-w-[220px] truncate text-xs text-terra-muted dark:text-dark-400">{{ item.username || fallbackUserLabel(item) }}</p>
              </td>
              <td class="px-3 py-3">
                <span class="inline-flex max-w-[180px] items-center rounded-md bg-primary-50 px-2 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-200">
                  <span class="truncate">{{ chainLabel(item, selected.level) }}</span>
                </span>
              </td>
              <td class="px-3 py-3 text-terra-muted dark:text-dark-300">{{ formatDateTime(item.joined_at) || '-' }}</td>
              <td class="px-3 py-3 text-right text-terra-ink dark:text-dark-100">{{ formatOrderCount(item.order_count) }}</td>
              <td class="px-3 py-3 text-right font-semibold text-primary-700 dark:text-primary-300">{{ formatCurrency(item.total_rebate) }}</td>
              <td class="px-3 py-3 text-right font-medium text-amber-600 dark:text-amber-300">{{ formatCurrency(item.frozen_rebate) }}</td>
              <td class="px-3 py-3 text-right font-medium text-emerald-700 dark:text-emerald-300">{{ formatCurrency(item.available_rebate) }}</td>
              <td class="px-3 py-3 text-terra-muted dark:text-dark-300">{{ formatDateTime(item.last_rebate_at) || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="mt-5 space-y-3 md:hidden">
        <div
          v-for="item in selected.invitees"
          :key="`${selected.level}-mobile-${item.user_id}`"
          class="rounded-lg border border-terra-line bg-terra-surface p-4 dark:border-dark-700 dark:bg-dark-900/70"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="truncate font-medium text-terra-ink dark:text-white">{{ item.email || fallbackUserLabel(item) }}</p>
              <p class="mt-0.5 truncate text-xs text-terra-muted dark:text-dark-400">{{ chainLabel(item, selected.level) }}</p>
            </div>
            <p class="shrink-0 text-sm font-semibold text-primary-700 dark:text-primary-300">{{ formatCurrency(item.total_rebate) }}</p>
          </div>
          <div class="mt-3 grid grid-cols-2 gap-3 text-xs">
            <div>
              <p class="text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.columns.orders') }}</p>
              <p class="mt-0.5 font-medium text-terra-ink dark:text-dark-100">{{ formatOrderCount(item.order_count) }}</p>
            </div>
            <div>
              <p class="text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.columns.available') }}</p>
              <p class="mt-0.5 font-medium text-emerald-700 dark:text-emerald-300">{{ formatCurrency(item.available_rebate) }}</p>
            </div>
            <div>
              <p class="text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.columns.frozen') }}</p>
              <p class="mt-0.5 font-medium text-amber-600 dark:text-amber-300">{{ formatCurrency(item.frozen_rebate) }}</p>
            </div>
            <div>
              <p class="text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.columns.lastRebate') }}</p>
              <p class="mt-0.5 font-medium text-terra-ink dark:text-dark-100">{{ formatDateTime(item.last_rebate_at) || '-' }}</p>
            </div>
          </div>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AffiliateLevelDetail, AffiliateLevelInvitee, UserAffiliateDetail } from '@/types'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { formatCount } from './useAffiliateLevels'

const props = defineProps<{
  detail: UserAffiliateDetail
  levelRebateAmounts: number[]
}>()

const { t } = useI18n()
const activeLevel = ref(1)

const levelDetails = computed<AffiliateLevelDetail[]>(() => {
  const rows = props.detail.level_details ?? []
  return [1, 2, 3].map((level) => {
    const existing = rows.find((item) => item.level === level)
    if (existing) {
      return { ...existing, invitees: existing.invitees ?? [] }
    }
    const fallbackInvitees: AffiliateLevelInvitee[] = level === 1
      ? (props.detail.invitees ?? []).map((item) => ({
          user_id: item.user_id,
          email: item.email,
          username: item.username,
          joined_at: item.created_at,
          total_rebate: item.total_rebate,
          frozen_rebate: 0,
          available_rebate: item.total_rebate,
          order_count: 0,
        }))
      : []
    const rebateAmount = props.levelRebateAmounts[level - 1] ?? 0
    return {
      level,
      invitee_count: fallbackInvitees.length,
      total_rebate: rebateAmount,
      frozen_rebate: 0,
      available_rebate: rebateAmount,
      invitees: fallbackInvitees,
    }
  })
})

const selected = computed<AffiliateLevelDetail>(() => (
  levelDetails.value.find((item) => item.level === activeLevel.value) ?? levelDetails.value[0]
))

function formatOrderCount(value: number): string {
  return t('affiliate.levelDetails.orderCount', { count: formatCount(value) })
}

function fallbackUserLabel(item: AffiliateLevelInvitee): string {
  return `#${item.user_id}`
}

function chainLabel(item: AffiliateLevelInvitee, level: number): string {
  if (level <= 1) return t('affiliate.levelDetails.directChain')
  const parent = item.parent_email || item.parent_username || (item.parent_user_id ? `#${item.parent_user_id}` : '')
  return parent ? t('affiliate.levelDetails.viaUser', { user: parent }) : t('affiliate.levelDetails.noChain')
}
</script>
