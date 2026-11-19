<template>
  <div class="card p-6">
    <div class="flex items-start justify-between gap-3">
      <div>
        <h3 class="text-base font-semibold text-terra-ink dark:text-white">{{ t('affiliate.withdrawals.title') }}</h3>
        <p class="mt-1 text-sm text-terra-muted dark:text-dark-400">{{ t('affiliate.withdrawals.subtitle') }}</p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="reload">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
      </button>
    </div>

    <div v-if="!loading && items.length === 0" class="mt-5 rounded-lg border border-dashed border-terra-line p-6 text-center text-sm text-terra-muted dark:border-dark-700 dark:text-dark-400">
      {{ t('affiliate.withdrawals.empty') }}
    </div>

    <ul v-else class="mt-5 divide-y divide-terra-line/70 dark:divide-dark-800">
      <li
        v-for="item in items"
        :key="item.id"
        class="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between"
        data-test="withdraw-row"
      >
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-base font-semibold text-terra-ink dark:text-white">{{ formatCurrency(item.amount) }}</span>
            <AffiliateWithdrawStatusBadge :status="item.status" />
          </div>
          <p class="mt-1 truncate text-xs text-terra-muted dark:text-dark-400">
            {{ maskAlipayAccount(item.alipay_account) }} · {{ formatDateTime(item.created_at) }}
          </p>
          <p v-if="item.status === 'rejected' && item.admin_note" class="mt-1 text-xs text-red-600 dark:text-red-400">
            {{ t('affiliate.withdrawals.rejectReason', { reason: item.admin_note }) }}
          </p>
          <p v-else-if="item.status === 'paid' && item.admin_note" class="mt-1 text-xs text-terra-muted dark:text-dark-400">
            {{ t('affiliate.withdrawals.note') }}：{{ item.admin_note }}
          </p>
        </div>
        <button
          v-if="item.status === 'pending'"
          type="button"
          class="btn btn-secondary btn-sm self-start sm:self-center"
          :disabled="cancellingId === item.id"
          @click="pendingCancel = item"
        >
          {{ t('affiliate.withdrawals.cancel') }}
        </button>
      </li>
    </ul>

    <div v-if="items.length < total" class="mt-3 text-center">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="loadMore">
        {{ t('affiliate.withdrawals.loadMore') }}
      </button>
    </div>

    <ConfirmDialog
      :show="pendingCancel !== null"
      :title="t('affiliate.withdrawals.cancelTitle')"
      :message="t('affiliate.withdrawals.cancelMessage', { amount: formatCurrency(pendingCancel?.amount ?? 0) })"
      danger
      @confirm="confirmCancel"
      @cancel="pendingCancel = null"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { AffiliateWithdrawRequest } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractI18nErrorMessage } from '@/utils/apiError'
import AffiliateWithdrawStatusBadge from './AffiliateWithdrawStatusBadge.vue'
import { maskAlipayAccount } from './qrImage'

const emit = defineEmits<{ (e: 'changed'): void }>()

const PAGE_SIZE = 10
const { t } = useI18n()
const appStore = useAppStore()

const items = ref<AffiliateWithdrawRequest[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
const cancellingId = ref<number | null>(null)
const pendingCancel = ref<AffiliateWithdrawRequest | null>(null)

async function fetchPage(target: number, append: boolean): Promise<void> {
  loading.value = true
  try {
    const resp = await userAPI.listAffiliateWithdraws(target, PAGE_SIZE)
    items.value = append ? [...items.value, ...(resp.items ?? [])] : (resp.items ?? [])
    total.value = resp.total ?? items.value.length
    page.value = target
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.errors', t('affiliate.withdrawals.loadFailed')))
  } finally {
    loading.value = false
  }
}

function reload(): Promise<void> {
  return fetchPage(1, false)
}

function loadMore(): Promise<void> {
  return fetchPage(page.value + 1, true)
}

async function confirmCancel(): Promise<void> {
  const target = pendingCancel.value
  pendingCancel.value = null
  if (!target) return
  cancellingId.value = target.id
  try {
    await userAPI.cancelAffiliateWithdraw(target.id)
    appStore.showSuccess(t('affiliate.withdrawals.cancelSuccess'))
    emit('changed')
    await reload()
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.errors', t('affiliate.withdrawals.cancelFailed')))
  } finally {
    cancellingId.value = null
  }
}

onMounted(() => {
  void reload()
})

defineExpose({ reload })
</script>
