<template>
  <span v-if="!result" class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.upstreamBalance.never') }}</span>
  <span
    v-else-if="result.status === 'ok'"
    class="inline-flex items-center gap-1 text-xs"
    :title="tooltip"
  >
    <span v-if="result.unlimited" class="font-medium text-gray-700 dark:text-gray-200">{{ t('admin.upstreamBalance.unlimited') }}</span>
    <span
      v-else
      class="font-medium"
      :class="isLow ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'"
    >
      ${{ formatAmount(result.balance) }}
    </span>
    <span v-if="result.used !== undefined" class="text-gray-400 dark:text-dark-500">
      / {{ t('admin.upstreamBalance.used') }} ${{ formatAmount(result.used) }}
    </span>
  </span>
  <span
    v-else
    class="text-xs"
    :class="result.status === 'unsupported' ? 'text-gray-400 dark:text-dark-500' : 'text-amber-600 dark:text-amber-400'"
    :title="tooltip"
  >
    {{ result.status === 'unsupported' ? t('admin.upstreamBalance.unsupported') : t('admin.upstreamBalance.failed') }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UpstreamBalanceResult } from '@/api/admin/accountInsights'
import { formatDateTime } from '@/utils/format'

const props = withDefaults(defineProps<{
  result?: UpstreamBalanceResult | null
  /** 低于该值标红 */
  lowThreshold?: number
}>(), {
  result: null,
  lowThreshold: 5
})

const { t } = useI18n()

const isLow = computed(() => (props.result?.balance ?? Infinity) < props.lowThreshold)

const sourceLabel = computed(() => {
  switch (props.result?.source) {
    case 'sub2api': return 'sub2api /v1/usage'
    case 'newapi_key': return 'new-api /dashboard/billing'
    default: return ''
  }
})

const tooltip = computed(() => {
  const r = props.result
  if (!r) return ''
  const parts = [sourceLabel.value, r.fetched_at ? formatDateTime(r.fetched_at) : '']
  if (r.error) parts.push(r.error)
  if (r.http_status) parts.push(`HTTP ${r.http_status}`)
  return parts.filter(Boolean).join(' · ')
})

function formatAmount(v?: number): string {
  if (v === undefined || v === null) return '-'
  return Math.abs(v) >= 100 ? v.toFixed(2) : v.toFixed(4).replace(/0{1,2}$/, '')
}
</script>
