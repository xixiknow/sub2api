<template>
  <BaseDialog :show="show" :title="t('affiliate.withdraw.title')" width="normal" @close="handleClose">
    <form id="affiliate-withdraw-form" class="space-y-5" @submit.prevent="submit">
      <p class="rounded-lg border border-terra-line bg-terra-surface px-4 py-3 text-sm leading-6 text-terra-muted dark:border-dark-700 dark:bg-dark-900/70 dark:text-dark-300">
        {{ t('affiliate.withdraw.description') }}
      </p>

      <div>
        <label for="affiliate-withdraw-amount" class="input-label">{{ t('affiliate.withdraw.amount') }}</label>
        <div class="flex gap-2">
          <input
            id="affiliate-withdraw-amount"
            v-model.number="amount"
            type="number"
            step="0.01"
            :min="minAmount > 0 ? minAmount : 0.01"
            :max="available"
            class="input flex-1"
            data-test="withdraw-amount"
          />
          <button type="button" class="btn btn-secondary shrink-0" @click="amount = roundDown(available)">
            {{ t('affiliate.withdraw.fillAll') }}
          </button>
        </div>
        <p class="input-hint">
          {{ minAmount > 0
            ? t('affiliate.withdraw.amountHint', { available: formatCurrency(available), min: formatCurrency(minAmount) })
            : t('affiliate.withdraw.amountHintNoMin', { available: formatCurrency(available) }) }}
        </p>
      </div>

      <div>
        <label for="affiliate-withdraw-account" class="input-label">{{ t('affiliate.withdraw.account') }}</label>
        <input
          id="affiliate-withdraw-account"
          v-model="account"
          type="text"
          maxlength="128"
          autocomplete="off"
          class="input"
          :placeholder="t('affiliate.withdraw.accountPlaceholder')"
          data-test="withdraw-account"
        />
        <p class="input-hint">{{ t('affiliate.withdraw.accountHint') }}</p>
      </div>

      <div>
        <p class="input-label">{{ t('affiliate.withdraw.qr') }}</p>
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
          <label
            class="group relative flex h-40 w-40 shrink-0 cursor-pointer items-center justify-center overflow-hidden rounded-xl border-2 border-dashed transition"
            :class="qrImage
              ? 'border-solid border-terra-line bg-white dark:border-dark-600'
              : 'border-terra-line bg-terra-surface hover:border-primary-400 dark:border-dark-600 dark:bg-dark-900/70'"
          >
            <input
              ref="fileInput"
              type="file"
              accept="image/png,image/jpeg,image/webp"
              class="hidden"
              data-test="withdraw-qr-input"
              @change="handleFile"
            />
            <img v-if="qrImage" :src="qrImage" alt="" class="h-full w-full object-contain" />
            <span v-else class="flex flex-col items-center gap-2 px-3 text-center text-xs text-terra-muted dark:text-dark-400">
              <Icon :name="processingImage ? 'refresh' : 'upload'" size="md" :class="processingImage ? 'animate-spin' : ''" />
              {{ processingImage ? t('affiliate.withdraw.qrProcessing') : t('affiliate.withdraw.qrUpload') }}
            </span>
          </label>
          <div class="space-y-2">
            <p class="text-xs leading-5 text-terra-muted dark:text-dark-400">{{ t('affiliate.withdraw.qrHint') }}</p>
            <div v-if="qrImage" class="flex gap-2">
              <button type="button" class="btn btn-secondary btn-sm" @click="fileInput?.click()">
                {{ t('affiliate.withdraw.qrReplace') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm text-red-600 dark:text-red-400" @click="clearImage">
                {{ t('affiliate.withdraw.qrRemove') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="affiliate-withdraw-form"
          class="btn btn-primary"
          :disabled="submitting || processingImage"
          data-test="withdraw-submit"
        >
          <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
          <span>{{ submitting ? t('affiliate.withdraw.submitting') : t('affiliate.withdraw.submit') }}</span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { AffiliateWithdrawRequest } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatCurrency } from '@/utils/format'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { prepareQRImage, QRImageError } from './qrImage'

const props = defineProps<{
  show: boolean
  available: number
  minAmount: number
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submitted', request: AffiliateWithdrawRequest): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const amount = ref<number | ''>('')
const account = ref('')
const qrImage = ref('')
const submitting = ref(false)
const processingImage = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

watch(() => props.show, (open) => {
  if (open) {
    amount.value = roundDown(props.available)
    account.value = ''
    qrImage.value = ''
  }
})

function roundDown(value: number): number {
  return Math.floor((Number(value) || 0) * 100) / 100
}

function clearImage(): void {
  qrImage.value = ''
  if (fileInput.value) fileInput.value.value = ''
}

async function handleFile(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  processingImage.value = true
  try {
    qrImage.value = await prepareQRImage(file)
  } catch (error) {
    const code = error instanceof QRImageError ? error.code : 'readFailed'
    const key = code === 'invalidType' ? 'qrInvalidType' : code === 'tooLarge' ? 'qrTooLarge' : 'qrReadFailed'
    appStore.showError(t(`affiliate.withdraw.${key}`))
  } finally {
    processingImage.value = false
    input.value = ''
  }
}

function validate(): string | null {
  const value = Number(amount.value)
  if (!Number.isFinite(value) || value <= 0) return t('affiliate.withdraw.amountRequired')
  if (props.minAmount > 0 && value < props.minAmount) {
    return t('affiliate.withdraw.amountBelowMin', { amount: formatCurrency(props.minAmount) })
  }
  if (value > props.available + 1e-9) return t('affiliate.withdraw.amountExceeds')
  if (!account.value.trim()) return t('affiliate.withdraw.accountRequired')
  if (!qrImage.value) return t('affiliate.withdraw.qrRequired')
  return null
}

async function submit(): Promise<void> {
  if (submitting.value) return
  const error = validate()
  if (error) {
    appStore.showError(error)
    return
  }
  submitting.value = true
  try {
    const request = await userAPI.submitAffiliateWithdraw({
      amount: Number(amount.value),
      alipay_account: account.value.trim(),
      qr_image: qrImage.value,
    })
    appStore.showSuccess(t('affiliate.withdraw.success', { amount: formatCurrency(request.amount) }))
    emit('submitted', request)
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'affiliate.errors', t('affiliate.withdraw.failed')))
  } finally {
    submitting.value = false
  }
}

function handleClose(): void {
  if (submitting.value) return
  emit('close')
}
</script>
