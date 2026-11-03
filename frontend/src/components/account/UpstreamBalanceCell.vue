<template>
  <div v-if="eligible" class="flex items-center gap-1.5">
    <span class="text-[11px] text-gray-400 dark:text-dark-500">{{ t('admin.upstreamBalance.label') }}</span>
    <UpstreamBalanceBadge :result="snapshot" />
    <button
      class="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-600"
      :disabled="refreshing"
      :title="t('admin.upstreamBalance.refresh')"
      @click.stop="refresh"
    >
      <Icon name="refresh" size="xs" :class="refreshing ? 'animate-spin' : ''" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { UpstreamBalanceResult } from '@/api/admin/accountInsights'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import Icon from '@/components/icons/Icon.vue'
import UpstreamBalanceBadge from './UpstreamBalanceBadge.vue'

const props = defineProps<{
  account: {
    id: number
    type: string
    parent_account_id?: number | null
    credentials?: Record<string, unknown>
    extra?: Record<string, unknown>
  }
}>()

const { t } = useI18n()
const appStore = useAppStore()

// 与后端 upstreamBillingProbeOfficialAPIDomains 对齐的常见官方域；官方 API 没有中转余额端点。
const OFFICIAL_HOST_SUFFIXES = ['anthropic.com', 'openai.com', 'googleapis.com', 'x.ai', 'moonshot.cn', 'moonshot.ai', 'deepseek.com', 'bigmodel.cn', 'minimaxi.com', 'minimax.io']

const baseURL = computed(() => {
  const raw = props.account.credentials?.base_url
  return typeof raw === 'string' ? raw.trim() : ''
})

const eligible = computed(() => {
  if (props.account.type !== 'apikey' || props.account.parent_account_id) return false
  if (!baseURL.value) return false
  try {
    const host = new URL(baseURL.value).hostname.toLowerCase()
    return !OFFICIAL_HOST_SUFFIXES.some((suffix) => host === suffix || host.endsWith(`.${suffix}`))
  } catch {
    return false
  }
})

const local = ref<UpstreamBalanceResult | null>(null)
watch(() => props.account.extra?.upstream_balance, () => { local.value = null })

const snapshot = computed<UpstreamBalanceResult | null>(() => {
  if (local.value) return local.value
  const raw = props.account.extra?.upstream_balance
  return raw && typeof raw === 'object' ? (raw as UpstreamBalanceResult) : null
})

const refreshing = ref(false)
async function refresh() {
  refreshing.value = true
  try {
    local.value = await adminAPI.accountInsights.refreshAccountBalance(props.account.id)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalance.refreshFailed')))
  } finally {
    refreshing.value = false
  }
}
</script>
