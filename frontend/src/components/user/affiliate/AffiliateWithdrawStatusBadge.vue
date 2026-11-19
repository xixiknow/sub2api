<template>
  <span
    class="inline-flex items-center whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium"
    :class="statusClass"
  >
    {{ statusLabel }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AffiliateWithdrawStatus } from '@/types'

const props = withDefaults(defineProps<{
  status: AffiliateWithdrawStatus | string
  /** i18n 前缀，用户端与管理端文案分开维护 */
  namespace?: string
}>(), {
  namespace: 'affiliate.withdrawals.statuses',
})

const { t } = useI18n()

const classMap: Record<string, string> = {
  pending: 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300',
  paid: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300',
  rejected: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
  cancelled: 'bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-dark-300',
}

const statusClass = computed(() => classMap[props.status] ?? classMap.cancelled)
const statusLabel = computed(() => {
  const key = `${props.namespace}.${props.status}`
  const label = t(key)
  return label === key ? props.status : label
})
</script>
