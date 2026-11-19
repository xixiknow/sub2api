<template>
  <section class="card overflow-hidden">
    <div class="grid lg:grid-cols-[1.35fr_1fr]">
      <!-- 资产 -->
      <div class="border-b border-terra-line p-6 dark:border-dark-700 lg:border-b-0 lg:border-r">
        <p class="flex items-center gap-1.5 text-sm font-medium text-terra-muted dark:text-dark-400">
          <Icon name="dollar" size="sm" class="text-primary-500" />
          {{ t('affiliate.assets.available') }}
        </p>
        <p class="mt-2 text-4xl font-semibold tracking-tight text-emerald-600 dark:text-emerald-400" data-test="assets-available">
          {{ formatCurrency(detail.aff_quota) }}
        </p>
        <p class="mt-1 text-xs text-terra-muted dark:text-dark-400">{{ t('affiliate.assets.availableHint') }}</p>

        <div class="mt-5 flex flex-col gap-2 sm:flex-row">
          <button
            type="button"
            class="btn btn-secondary justify-center"
            :disabled="transferring || detail.aff_quota <= 0"
            data-test="assets-transfer"
            @click="emit('transfer')"
          >
            <Icon :name="transferring ? 'refresh' : 'dollar'" size="sm" :class="transferring ? 'animate-spin' : ''" />
            <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.assets.transferButton') }}</span>
          </button>
          <button
            type="button"
            class="btn btn-primary justify-center"
            :disabled="!canWithdraw"
            :title="withdrawDisabledReason"
            data-test="assets-withdraw"
            @click="emit('withdraw')"
          >
            <Icon name="upload" size="sm" />
            <span>{{ t('affiliate.assets.withdrawButton') }}</span>
          </button>
        </div>
        <p v-if="withdrawDisabledReason" class="mt-2 text-xs text-terra-muted dark:text-dark-400" data-test="assets-withdraw-hint">
          {{ withdrawDisabledReason }}
        </p>

        <div
          v-if="detail.pending_withdraw"
          class="mt-4 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200"
          data-test="assets-pending"
        >
          <Icon name="clock" size="sm" class="mt-0.5 shrink-0" />
          <span>
            {{ t('affiliate.assets.pendingNotice', {
              amount: formatCurrency(detail.pending_withdraw.amount),
              time: formatDateTime(detail.pending_withdraw.created_at),
            }) }}
          </span>
        </div>
      </div>

      <!-- 统计 -->
      <dl class="grid grid-cols-2 gap-px bg-terra-line dark:bg-dark-700">
        <div class="bg-terra-elevated p-5 dark:bg-dark-900">
          <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.assets.frozen') }}</dt>
          <dd class="mt-1 text-xl font-semibold text-amber-600 dark:text-amber-400">{{ formatCurrency(detail.aff_frozen_quota) }}</dd>
          <dd class="mt-0.5 text-[11px] leading-4 text-terra-muted dark:text-dark-500">{{ t('affiliate.assets.frozenHint') }}</dd>
        </div>
        <div class="bg-terra-elevated p-5 dark:bg-dark-900">
          <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.assets.history') }}</dt>
          <dd class="mt-1 text-xl font-semibold text-terra-ink dark:text-white">{{ formatCurrency(detail.aff_history_quota) }}</dd>
        </div>
        <div class="bg-terra-elevated p-5 dark:bg-dark-900">
          <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.assets.rate') }}</dt>
          <dd class="mt-1 text-xl font-semibold text-primary-600 dark:text-primary-400">{{ rebateRate }}%</dd>
        </div>
        <div class="bg-terra-elevated p-5 dark:bg-dark-900">
          <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.stats.invitedUsers') }}</dt>
          <dd class="mt-1 text-xl font-semibold text-terra-ink dark:text-white">{{ formatCount(detail.aff_count) }}</dd>
          <dd class="mt-0.5 text-[11px] leading-4 text-terra-muted dark:text-dark-500">
            {{ t('affiliate.assets.invited', { qualified: formatCount(detail.qualified_aff_count ?? 0), total: formatCount(detail.aff_count) }) }}
          </dd>
        </div>
      </dl>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserAffiliateDetail } from '@/types'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { formatCount } from './useAffiliateLevels'

const props = defineProps<{
  detail: UserAffiliateDetail
  rebateRate: string
  transferring: boolean
}>()

const emit = defineEmits<{
  (e: 'transfer'): void
  (e: 'withdraw'): void
}>()

const { t } = useI18n()

const minAmount = computed(() => props.detail.withdraw_min_amount ?? 0)

const withdrawDisabledReason = computed(() => {
  if (!props.detail.withdraw_enabled) return t('affiliate.assets.withdrawDisabled')
  if (props.detail.pending_withdraw) return ''
  if (minAmount.value > 0 && props.detail.aff_quota < minAmount.value) {
    return t('affiliate.assets.withdrawMinHint', { amount: formatCurrency(minAmount.value) })
  }
  return ''
})

const canWithdraw = computed(() => (
  Boolean(props.detail.withdraw_enabled)
  && !props.detail.pending_withdraw
  && props.detail.aff_quota > 0
  && (minAmount.value <= 0 || props.detail.aff_quota >= minAmount.value)
))
</script>
