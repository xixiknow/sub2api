<template>
  <section class="card p-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h3 class="text-base font-semibold text-terra-ink dark:text-white">{{ t('affiliate.registrationSeats.title') }}</h3>
        <p class="mt-1 text-sm text-terra-muted dark:text-dark-400">{{ t('affiliate.registrationSeats.description') }}</p>
      </div>
      <div
        class="inline-flex w-fit items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold"
        :class="linkUsable
          ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-900/25 dark:text-emerald-200 dark:ring-emerald-800/50'
          : 'bg-amber-50 text-amber-700 ring-1 ring-amber-200 dark:bg-amber-900/25 dark:text-amber-200 dark:ring-amber-800/50'"
      >
        <Icon :name="linkUsable ? 'checkCircle' : 'exclamationTriangle'" size="sm" />
        <span>{{ linkUsable ? t('affiliate.registrationSeats.statusAvailable') : t('affiliate.registrationSeats.statusEmpty') }}</span>
      </div>
    </div>

    <div v-if="free" class="mt-5 rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-900/50 dark:bg-emerald-900/20">
      <p class="text-sm font-semibold text-emerald-800 dark:text-emerald-100">{{ t('affiliate.registrationSeats.freeModeTitle') }}</p>
      <p class="mt-1 text-sm text-emerald-700 dark:text-emerald-200">{{ t('affiliate.registrationSeats.freeModeDescription') }}</p>
    </div>

    <div v-else class="mt-5 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
      <dl class="grid grid-cols-3 gap-3">
        <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-3 py-3 dark:border-dark-700 dark:bg-dark-900/70">
          <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.registrationSeats.available') }}</dt>
          <dd class="mt-1 text-xl font-semibold text-emerald-700 dark:text-emerald-300">{{ formatCount(detail.registration_seat_available) }}</dd>
        </div>
        <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-3 py-3 dark:border-dark-700 dark:bg-dark-900/70">
          <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.registrationSeats.used') }}</dt>
          <dd class="mt-1 text-xl font-semibold text-terra-ink dark:text-dark-100">{{ formatCount(detail.registration_seat_used) }}</dd>
        </div>
        <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-3 py-3 dark:border-dark-700 dark:bg-dark-900/70">
          <dt class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.registrationSeats.total') }}</dt>
          <dd class="mt-1 text-xl font-semibold text-terra-ink dark:text-dark-100">{{ formatCount(detail.registration_seat_total) }}</dd>
        </div>
      </dl>

      <div class="rounded-lg border border-terra-line bg-terra-surface p-4 dark:border-dark-700 dark:bg-dark-900/70">
        <div class="grid gap-3 sm:grid-cols-[120px_minmax(0,1fr)_auto] sm:items-end">
          <div>
            <label for="registration-seat-quantity" class="text-xs font-semibold text-terra-muted dark:text-dark-400">
              {{ t('affiliate.registrationSeats.quantity') }}
            </label>
            <input
              id="registration-seat-quantity"
              v-model.number="quantity"
              type="number"
              min="1"
              max="1000"
              step="1"
              class="input mt-2"
              @blur="normalize"
            />
          </div>
          <div class="text-sm">
            <p class="text-xs font-semibold text-terra-muted dark:text-dark-400">{{ t('affiliate.registrationSeats.expectedCost') }}</p>
            <p class="mt-2 font-semibold text-primary-700 dark:text-primary-300">
              {{ formatCurrency(expectedCost) }}
              <span class="ml-1 text-xs font-normal text-terra-muted dark:text-dark-400">({{ t('affiliate.registrationSeats.cost') }} {{ formatCurrency(cost) }})</span>
            </p>
          </div>
          <button type="button" class="btn btn-primary justify-center" :disabled="purchasing || normalizedQuantity <= 0" @click="purchase">
            <Icon :name="purchasing ? 'refresh' : 'plus'" size="sm" :class="purchasing ? 'animate-spin' : ''" />
            <span>{{ purchasing ? t('affiliate.registrationSeats.purchasing') : t('affiliate.registrationSeats.purchase') }}</span>
          </button>
        </div>
        <p class="mt-3 text-xs text-terra-muted dark:text-dark-400">
          {{ t('affiliate.registrationSeats.balanceHint', { balance: formatCurrency(balance) }) }}
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserAffiliateDetail } from '@/types'
import { formatCurrency } from '@/utils/format'
import { formatCount } from './useAffiliateLevels'

const props = defineProps<{
  detail: UserAffiliateDetail
  balance: number
  purchasing: boolean
}>()

const emit = defineEmits<{ (e: 'purchase', quantity: number, expectedCost: number): void }>()

const { t } = useI18n()
const quantity = ref<number>(1)

const cost = computed(() => props.detail.registration_seat_cost ?? 0)
const free = computed(() => cost.value <= 0)
const linkUsable = computed(() => free.value || (props.detail.registration_seat_available ?? 0) > 0)
const normalizedQuantity = computed(() => {
  const parsed = Math.floor(Number(quantity.value))
  return Number.isFinite(parsed) && parsed > 0 ? Math.min(parsed, 1000) : 0
})
const expectedCost = computed(() => normalizedQuantity.value * cost.value)

function normalize(): void {
  quantity.value = normalizedQuantity.value > 0 ? normalizedQuantity.value : 1
}

function purchase(): void {
  normalize()
  if (normalizedQuantity.value <= 0 || props.purchasing) return
  emit('purchase', normalizedQuantity.value, expectedCost.value)
}
</script>
