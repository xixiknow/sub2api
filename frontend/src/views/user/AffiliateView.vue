<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <template v-else-if="detail">
        <AffiliateAssetsPanel
          :detail="detail"
          :rebate-rate="formattedRebateRate"
          :transferring="transferring"
          @transfer="confirmTransferOpen = true"
          @withdraw="withdrawDialogOpen = true"
        />

        <AffiliateInvitePanel
          :code="detail.aff_code"
          :link="inviteLink"
          :rebate-rate="formattedRebateRate"
          :rows="unlockProgressRows"
          :next-locked="nextLockedProgress"
          :source-label="levelRateSourceLabel"
          @copy-code="copyCode"
          @copy-link="copyInviteLink"
        />

        <div class="grid gap-6" :class="showWithdrawHistory ? 'xl:grid-cols-2' : ''">
          <AffiliateSeatsCard
            :detail="detail"
            :balance="currentBalance"
            :purchasing="purchasingSeats"
            @purchase="purchaseRegistrationSeats"
          />
          <AffiliateWithdrawHistory
            v-if="showWithdrawHistory"
            ref="historyRef"
            @changed="loadAffiliateDetail(true)"
          />
        </div>

        <AffiliateLevelDetails :detail="detail" :level-rebate-amounts="levelRebateAmounts" />

        <AffiliateRulesCollapse :rows="unlockProgressRows" />
      </template>
    </div>

    <AffiliateWithdrawDialog
      v-if="detail"
      :show="withdrawDialogOpen"
      :available="detail.aff_quota"
      :min-amount="detail.withdraw_min_amount ?? 0"
      @close="withdrawDialogOpen = false"
      @submitted="handleWithdrawSubmitted"
    />

    <ConfirmDialog
      :show="confirmTransferOpen"
      :title="t('affiliate.transfer.confirmTitle')"
      :message="t('affiliate.transfer.confirmMessage', { amount: formatCurrency(detail?.aff_quota ?? 0) })"
      @confirm="transferQuota"
      @cancel="confirmTransferOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import AffiliateAssetsPanel from '@/components/user/affiliate/AffiliateAssetsPanel.vue'
import AffiliateInvitePanel from '@/components/user/affiliate/AffiliateInvitePanel.vue'
import AffiliateSeatsCard from '@/components/user/affiliate/AffiliateSeatsCard.vue'
import AffiliateWithdrawHistory from '@/components/user/affiliate/AffiliateWithdrawHistory.vue'
import AffiliateLevelDetails from '@/components/user/affiliate/AffiliateLevelDetails.vue'
import AffiliateRulesCollapse from '@/components/user/affiliate/AffiliateRulesCollapse.vue'
import AffiliateWithdrawDialog from '@/components/user/affiliate/AffiliateWithdrawDialog.vue'
import { formatCount, useAffiliateLevels } from '@/components/user/affiliate/useAffiliateLevels'
import userAPI from '@/api/user'
import type { UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const purchasingSeats = ref(false)
const withdrawDialogOpen = ref(false)
const confirmTransferOpen = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)
const historyRef = ref<InstanceType<typeof AffiliateWithdrawHistory> | null>(null)

const {
  formattedRebateRate,
  levelRebateAmounts,
  unlockProgressRows,
  nextLockedProgress,
  levelRateSourceLabel,
} = useAffiliateLevels(detail)

const inviteLink = computed(() => {
  if (!detail.value) return ''
  const path = `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return typeof window === 'undefined' ? path : `${window.location.origin}${path}`
})

const currentBalance = computed(() => authStore.user?.balance ?? 0)
const showWithdrawHistory = computed(() => Boolean(detail.value?.withdraw_enabled || detail.value?.pending_withdraw))

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) loading.value = true
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) loading.value = false
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  confirmTransferOpen.value = false
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

async function handleWithdrawSubmitted(): Promise<void> {
  withdrawDialogOpen.value = false
  await loadAffiliateDetail(true)
  await nextTick()
  await historyRef.value?.reload()
}

async function purchaseRegistrationSeats(quantity: number, expectedCost: number): Promise<void> {
  if (quantity <= 0 || purchasingSeats.value) {
    appStore.showError(t('affiliate.registrationSeats.invalidQuantity'))
    return
  }
  purchasingSeats.value = true
  try {
    const resp = await userAPI.purchaseAffiliateRegistrationSeats(quantity)
    if (detail.value) {
      detail.value.registration_seat_cost = resp.registration_seat_cost
      detail.value.registration_seat_total = resp.registration_seat_total
      detail.value.registration_seat_used = resp.registration_seat_used
      detail.value.registration_seat_available = resp.registration_seat_available
    }
    appStore.showSuccess(t('affiliate.registrationSeats.purchaseSuccess', {
      count: formatCount(quantity),
      amount: formatCurrency(expectedCost),
    }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.registrationSeats.purchaseFailed')))
  } finally {
    purchasingSeats.value = false
  }
}

onMounted(() => {
  void loadAffiliateDetail()
})
</script>
