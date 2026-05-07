<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div class="card p-5">
            <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
              <Icon name="dollar" size="sm" class="text-primary-500" />
              {{ t('affiliate.stats.rebateRate') }}
            </p>
            <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formattedRebateRate }}<span class="ml-0.5 text-base font-medium">%</span>
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              {{ t('affiliate.stats.rebateRateHint') }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.invitedUsers') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCount(detail.aff_count) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.availableQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatCurrency(detail.aff_quota) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.totalQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(detail.aff_history_quota) }}
            </p>
            <p v-if="detail.aff_frozen_quota > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
              {{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}
            </p>
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <div
            v-for="item in levelRebateSummaries"
            :key="item.level"
            class="card p-5"
          >
            <p class="text-sm text-gray-500 dark:text-dark-400">
              {{ t('affiliate.stats.levelRebate', { level: item.level }) }}
            </p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(item.rebate_amount) }}
            </p>
          </div>
        </div>

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.title') }}</h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.description') }}</p>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
              <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ detail.aff_code }}</code>
                <button class="btn btn-secondary btn-sm" @click="copyCode">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyCode') }}</span>
                </button>
              </div>
            </div>

            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
              <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ inviteLink }}</code>
                <button class="btn btn-secondary btn-sm" @click="copyInviteLink">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyLink') }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="mt-5 rounded-xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-900/40 dark:bg-primary-900/20">
            <p class="text-sm font-medium text-primary-800 dark:text-primary-200">{{ t('affiliate.tips.title') }}</p>
            <ul class="mt-2 space-y-1 text-sm text-primary-700 dark:text-primary-300">
              <li>1. {{ t('affiliate.tips.line1') }}</li>
              <li>2. {{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
              <li>3. {{ t('affiliate.tips.line3') }}</li>
              <li v-if="detail.aff_frozen_quota > 0">4. {{ t('affiliate.tips.line4') }}</li>
            </ul>
          </div>
        </div>

        <div class="card p-6">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.transfer.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.transfer.description') }}</p>
            </div>
            <button
              class="btn btn-primary"
              :disabled="transferring || detail.aff_quota <= 0"
              @click="transferQuota"
            >
              <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="dollar" size="sm" />
              <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
            </button>
          </div>
          <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm text-amber-600 dark:text-amber-400">
            {{ t('affiliate.transfer.empty') }}
          </p>
        </div>

        <div class="card p-6">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <h3 class="text-base font-semibold text-terra-ink dark:text-white">
                {{ t('affiliate.levelDetails.title') }}
              </h3>
              <p class="mt-1 text-sm text-terra-muted dark:text-dark-400">
                {{ t('affiliate.levelDetails.subtitle') }}
              </p>
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
                <span class="block text-xs font-semibold">
                  {{ t('affiliate.levelDetails.levelTab', { level: item.level }) }}
                </span>
                <span class="mt-0.5 block truncate text-sm font-semibold">
                  {{ formatCurrency(item.total_rebate) }}
                </span>
              </button>
            </div>
          </div>

          <div class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
              <p class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.invitees') }}</p>
              <p class="mt-1 text-lg font-semibold text-terra-ink dark:text-white">{{ formatCount(selectedLevelDetail.invitee_count) }}</p>
            </div>
            <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
              <p class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.total') }}</p>
              <p class="mt-1 text-lg font-semibold text-primary-700 dark:text-primary-300">{{ formatCurrency(selectedLevelDetail.total_rebate) }}</p>
            </div>
            <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
              <p class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.frozen') }}</p>
              <p class="mt-1 text-lg font-semibold text-accent-600 dark:text-accent-300">{{ formatCurrency(selectedLevelDetail.frozen_rebate) }}</p>
            </div>
            <div class="rounded-lg border border-terra-line/80 bg-terra-surface px-4 py-3 dark:border-dark-700 dark:bg-dark-900/70">
              <p class="text-xs font-medium text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.stats.available') }}</p>
              <p class="mt-1 text-lg font-semibold text-emerald-700 dark:text-emerald-300">{{ formatCurrency(selectedLevelDetail.available_rebate) }}</p>
            </div>
          </div>

          <div v-if="selectedLevelDetail.invitees.length === 0" class="mt-5 rounded-lg border border-dashed border-terra-line p-6 text-center text-sm text-terra-muted dark:border-dark-700 dark:text-dark-400">
            {{ t('affiliate.levelDetails.empty', { level: selectedLevelLabel }) }}
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
                    v-for="item in selectedLevelDetail.invitees"
                    :key="`${selectedLevelDetail.level}-${item.user_id}`"
                    class="border-b border-terra-line/70 last:border-b-0 dark:border-dark-800"
                  >
                    <td class="px-3 py-3">
                      <p class="max-w-[220px] truncate font-medium text-terra-ink dark:text-white">
                        {{ item.email || fallbackUserLabel(item) }}
                      </p>
                      <p class="max-w-[220px] truncate text-xs text-terra-muted dark:text-dark-400">
                        {{ item.username || fallbackUserLabel(item) }}
                      </p>
                    </td>
                    <td class="px-3 py-3 text-terra-muted dark:text-dark-300">
                      <span class="inline-flex max-w-[180px] items-center rounded-md bg-primary-50 px-2 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-200">
                        <span class="truncate">{{ chainLabel(item, selectedLevelDetail.level) }}</span>
                      </span>
                    </td>
                    <td class="px-3 py-3 text-terra-muted dark:text-dark-300">{{ formatDateTime(item.joined_at) || '-' }}</td>
                    <td class="px-3 py-3 text-right text-terra-ink dark:text-dark-100">{{ formatOrderCount(item.order_count) }}</td>
                    <td class="px-3 py-3 text-right font-semibold text-primary-700 dark:text-primary-300">{{ formatCurrency(item.total_rebate) }}</td>
                    <td class="px-3 py-3 text-right font-medium text-accent-600 dark:text-accent-300">{{ formatCurrency(item.frozen_rebate) }}</td>
                    <td class="px-3 py-3 text-right font-medium text-emerald-700 dark:text-emerald-300">{{ formatCurrency(item.available_rebate) }}</td>
                    <td class="px-3 py-3 text-terra-muted dark:text-dark-300">{{ formatDateTime(item.last_rebate_at) || '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div class="mt-5 space-y-3 md:hidden">
              <div
                v-for="item in selectedLevelDetail.invitees"
                :key="`${selectedLevelDetail.level}-mobile-${item.user_id}`"
                class="rounded-lg border border-terra-line bg-terra-surface p-4 dark:border-dark-700 dark:bg-dark-900/70"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate font-medium text-terra-ink dark:text-white">
                      {{ item.email || fallbackUserLabel(item) }}
                    </p>
                    <p class="mt-0.5 truncate text-xs text-terra-muted dark:text-dark-400">
                      {{ chainLabel(item, selectedLevelDetail.level) }}
                    </p>
                  </div>
                  <p class="shrink-0 text-sm font-semibold text-primary-700 dark:text-primary-300">
                    {{ formatCurrency(item.total_rebate) }}
                  </p>
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
                    <p class="mt-0.5 font-medium text-accent-600 dark:text-accent-300">{{ formatCurrency(item.frozen_rebate) }}</p>
                  </div>
                  <div>
                    <p class="text-terra-muted dark:text-dark-400">{{ t('affiliate.levelDetails.columns.lastRebate') }}</p>
                    <p class="mt-0.5 font-medium text-terra-ink dark:text-dark-100">{{ formatDateTime(item.last_rebate_at) || '-' }}</p>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { AffiliateLevelDetail, AffiliateLevelInvitee, UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)
const activeLevel = ref(1)

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

const levelRebateSummaries = computed(() => {
  const rows = detail.value?.level_rebates ?? []
  return [1, 2, 3].map((level) => ({
    level,
    rebate_amount: rows.find((item) => item.level === level)?.rebate_amount ?? 0,
  }))
})

const levelDetails = computed<AffiliateLevelDetail[]>(() => {
  const rows = detail.value?.level_details ?? []
  return [1, 2, 3].map((level) => {
    const existing = rows.find((item) => item.level === level)
    if (existing) {
      return {
        ...existing,
        invitees: existing.invitees ?? []
      }
    }

    const fallbackInvitees: AffiliateLevelInvitee[] = level === 1
      ? (detail.value?.invitees ?? []).map((item) => ({
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
    const rebateAmount = levelRebateSummaries.value.find((item) => item.level === level)?.rebate_amount ?? 0

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

const selectedLevelDetail = computed<AffiliateLevelDetail>(() => (
  levelDetails.value.find((item) => item.level === activeLevel.value) ?? levelDetails.value[0]
))

const selectedLevelLabel = computed(() => (
  t('affiliate.levelDetails.levelLabel', { level: selectedLevelDetail.value.level })
))

function formatCount(value: number): string {
  return value.toLocaleString()
}

function formatOrderCount(value: number): string {
  return t('affiliate.levelDetails.orderCount', { count: formatCount(value) })
}

function fallbackUserLabel(item: AffiliateLevelInvitee): string {
  return `#${item.user_id}`
}

function displayParent(item: AffiliateLevelInvitee): string {
  return item.parent_email || item.parent_username || (item.parent_user_id ? `#${item.parent_user_id}` : '')
}

function chainLabel(item: AffiliateLevelInvitee, level: number): string {
  if (level <= 1) {
    return t('affiliate.levelDetails.directChain')
  }
  const parent = displayParent(item)
  return parent
    ? t('affiliate.levelDetails.viaUser', { user: parent })
    : t('affiliate.levelDetails.noChain')
}

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
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

onMounted(() => {
  void loadAffiliateDetail()
})
</script>
