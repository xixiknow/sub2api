<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full md:w-80">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="filters.search"
              type="text"
              class="input pl-10"
              :placeholder="t('admin.affiliates.withdrawals.searchPlaceholder')"
              @input="debounceLoad"
            />
          </div>
          <select v-model="filters.status" class="input w-full sm:w-36" data-test="withdraw-status-filter" @change="reloadFromFirstPage">
            <option value="">{{ t('admin.affiliates.withdrawals.allStatus') }}</option>
            <option v-for="status in statuses" :key="status" :value="status">
              {{ t(`admin.affiliates.withdrawals.statuses.${status}`) }}
            </option>
          </select>
          <input v-model="filters.start_at" type="date" class="input w-full sm:w-44" :title="t('admin.affiliates.records.startAt')" @change="reloadFromFirstPage" />
          <input v-model="filters.end_at" type="date" class="input w-full sm:w-44" :title="t('admin.affiliates.records.endAt')" @change="reloadFromFirstPage" />
          <button class="btn btn-secondary px-2 md:px-3" :disabled="loading" :title="t('common.refresh')" @click="loadRecords">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="records" :loading="loading">
          <template #cell-user="{ row }">
            <div class="min-w-0">
              <div class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ row.user_email || '-' }}</div>
              <div class="truncate text-xs text-gray-500 dark:text-dark-400">#{{ row.user_id }} {{ row.username }}</div>
            </div>
          </template>
          <template #cell-amount="{ row }">
            <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ formatCurrency(row.amount) }}</span>
          </template>
          <template #cell-alipay_account="{ row }">
            <span class="font-mono text-sm text-gray-700 dark:text-gray-300">{{ row.alipay_account }}</span>
          </template>
          <template #cell-status="{ row }">
            <AffiliateWithdrawStatusBadge :status="row.status" namespace="admin.affiliates.withdrawals.statuses" />
          </template>
          <template #cell-admin_note="{ row }">
            <span class="block max-w-56 truncate text-sm text-gray-600 dark:text-dark-300" :title="row.admin_note">{{ row.admin_note || '-' }}</span>
          </template>
          <template #cell-created_at="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(row.created_at) }}</span>
          </template>
          <template #cell-reviewed_at="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ row.reviewed_at ? formatDateTime(row.reviewed_at) : '-' }}</span>
          </template>
          <template #cell-actions="{ row }">
            <button
              type="button"
              class="btn btn-sm"
              :class="row.status === 'pending' ? 'btn-primary' : 'btn-secondary'"
              data-test="withdraw-view"
              @click="openDetail(row)"
            >
              {{ row.status === 'pending' ? t('admin.affiliates.withdrawals.markPaid') + ' / ' + t('admin.affiliates.withdrawals.reject') : t('admin.affiliates.withdrawals.view') }}
            </button>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="detailOpen"
      :title="t('admin.affiliates.withdrawals.detailTitle', { id: selected?.id ?? '' })"
      width="wide"
      @close="closeDetail"
    >
      <div v-if="detailLoading" class="flex justify-center py-10">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>
      <div v-else-if="selected" class="grid gap-6 md:grid-cols-[260px_minmax(0,1fr)]">
        <div>
          <p class="input-label">{{ t('admin.affiliates.withdrawals.qrImage') }}</p>
          <a
            v-if="selected.qr_image"
            :href="selected.qr_image"
            target="_blank"
            rel="noopener"
            class="block overflow-hidden rounded-xl border border-gray-200 bg-white p-2 dark:border-dark-600"
          >
            <img :src="selected.qr_image" alt="" class="mx-auto max-h-80 w-full object-contain" data-test="withdraw-qr" />
          </a>
          <p v-else class="rounded-xl border border-dashed border-gray-200 p-6 text-center text-sm text-gray-400 dark:border-dark-600">
            {{ t('admin.affiliates.withdrawals.qrMissing') }}
          </p>
        </div>

        <div class="space-y-4">
          <dl class="grid gap-3 sm:grid-cols-2">
            <div class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.withdrawals.user') }}</dt>
              <dd class="mt-1 truncate text-sm font-medium text-gray-900 dark:text-white">{{ selected.user_email || '-' }}</dd>
              <dd class="text-xs text-gray-500 dark:text-dark-400">#{{ selected.user_id }} {{ selected.username }}</dd>
            </div>
            <div class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.withdrawals.status') }}</dt>
              <dd class="mt-1"><AffiliateWithdrawStatusBadge :status="selected.status" namespace="admin.affiliates.withdrawals.statuses" /></dd>
            </div>
            <div class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.withdrawals.amount') }}</dt>
              <dd class="mt-1 flex items-center justify-between gap-2">
                <span class="text-xl font-semibold text-gray-900 dark:text-white">{{ formatCurrency(selected.amount) }}</span>
                <button type="button" class="btn btn-secondary btn-sm" @click="copyAmount">
                  <Icon name="copy" size="sm" />
                </button>
              </dd>
            </div>
            <div class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.withdrawals.alipayAccount') }}</dt>
              <dd class="mt-1 flex items-center justify-between gap-2">
                <span class="break-all font-mono text-sm text-gray-900 dark:text-white" data-test="withdraw-account">{{ selected.alipay_account }}</span>
                <button type="button" class="btn btn-secondary btn-sm" :title="t('admin.affiliates.withdrawals.copyAccount')" @click="copyAccount">
                  <Icon name="copy" size="sm" />
                </button>
              </dd>
            </div>
            <div class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.withdrawals.createdAt') }}</dt>
              <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ formatDateTime(selected.created_at) }}</dd>
            </div>
            <div class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.withdrawals.reviewedAt') }}</dt>
              <dd class="mt-1 text-sm text-gray-900 dark:text-white">{{ selected.reviewed_at ? formatDateTime(selected.reviewed_at) : '-' }}</dd>
            </div>
          </dl>

          <p v-if="selected.admin_note" class="rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:bg-dark-800 dark:text-dark-200">
            {{ t('admin.affiliates.withdrawals.note') }}：{{ selected.admin_note }}
          </p>

          <template v-if="selected.status === 'pending'">
            <p class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200">
              {{ t('admin.affiliates.withdrawals.payHint', { amount: formatCurrency(selected.amount) }) }}
            </p>
            <div>
              <label class="input-label">{{ action === 'reject' ? t('admin.affiliates.withdrawals.rejectReason') : t('admin.affiliates.withdrawals.paidNote') }}</label>
              <textarea
                v-model="note"
                rows="2"
                maxlength="500"
                class="input"
                :placeholder="action === 'reject' ? t('admin.affiliates.withdrawals.rejectReasonPlaceholder') : t('admin.affiliates.withdrawals.paidNotePlaceholder')"
                data-test="withdraw-note"
              ></textarea>
            </div>
          </template>
        </div>
      </div>

      <template v-if="selected?.status === 'pending' && !detailLoading" #footer>
        <div class="flex flex-wrap justify-end gap-3">
          <template v-if="action === null">
            <button type="button" class="btn btn-secondary text-red-600 dark:text-red-400" data-test="withdraw-reject" @click="action = 'reject'">
              {{ t('admin.affiliates.withdrawals.reject') }}
            </button>
            <button type="button" class="btn btn-primary" data-test="withdraw-paid" @click="action = 'paid'">
              <Icon name="checkCircle" size="sm" />
              {{ t('admin.affiliates.withdrawals.markPaid') }}
            </button>
          </template>
          <template v-else>
            <p class="mr-auto self-center text-sm text-gray-600 dark:text-dark-300">
              {{ action === 'reject'
                ? t('admin.affiliates.withdrawals.rejectMessage', { amount: formatCurrency(selected.amount) })
                : t('admin.affiliates.withdrawals.markPaidMessage', { amount: formatCurrency(selected.amount), account: selected.alipay_account }) }}
            </p>
            <button type="button" class="btn btn-secondary" :disabled="submitting" @click="action = null">
              {{ t('common.cancel') }}
            </button>
            <button
              type="button"
              class="btn"
              :class="action === 'reject' ? 'btn-danger' : 'btn-primary'"
              :disabled="submitting"
              data-test="withdraw-confirm"
              @click="submitAction"
            >
              <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
              {{ action === 'reject' ? t('admin.affiliates.withdrawals.rejectTitle') : t('admin.affiliates.withdrawals.markPaidTitle') }}
            </button>
          </template>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import AffiliateWithdrawStatusBadge from '@/components/user/affiliate/AffiliateWithdrawStatusBadge.vue'
import type { Column } from '@/components/common/types'
import { affiliatesAPI } from '@/api/admin/affiliates'
import type { AffiliateWithdrawRequest, AffiliateWithdrawStatus } from '@/types'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateTime } from '@/utils/format'

const statuses: AffiliateWithdrawStatus[] = ['pending', 'paid', 'rejected', 'cancelled']

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const records = ref<AffiliateWithdrawRequest[]>([])
const filters = reactive<{ search: string; status: AffiliateWithdrawStatus | ''; start_at: string; end_at: string }>({
  search: '',
  status: 'pending',
  start_at: '',
  end_at: '',
})
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const detailOpen = ref(false)
const detailLoading = ref(false)
const selected = ref<AffiliateWithdrawRequest | null>(null)
const action = ref<'paid' | 'reject' | null>(null)
const note = ref('')
const submitting = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.affiliates.withdrawals.user') },
  { key: 'amount', label: t('admin.affiliates.withdrawals.amount') },
  { key: 'alipay_account', label: t('admin.affiliates.withdrawals.alipayAccount') },
  { key: 'status', label: t('admin.affiliates.withdrawals.status') },
  { key: 'admin_note', label: t('admin.affiliates.withdrawals.note') },
  { key: 'created_at', label: t('admin.affiliates.withdrawals.createdAt') },
  { key: 'reviewed_at', label: t('admin.affiliates.withdrawals.reviewedAt') },
  { key: 'actions', label: t('admin.affiliates.withdrawals.actions') },
])

function errorMessage(error: unknown, fallback: string): string {
  return extractI18nErrorMessage(error, t, 'admin.affiliates.errors', fallback)
}

async function loadRecords(): Promise<void> {
  loading.value = true
  try {
    const resp = await affiliatesAPI.listWithdrawRequests({
      page: pagination.page,
      page_size: pagination.page_size,
      status: filters.status,
      search: filters.search.trim(),
      start_at: filters.start_at,
      end_at: filters.end_at,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    })
    records.value = resp.items ?? []
    pagination.total = resp.total ?? 0
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.affiliates.errors.loadFailed')))
  } finally {
    loading.value = false
  }
}

function reloadFromFirstPage(): void {
  pagination.page = 1
  void loadRecords()
}

function debounceLoad(): void {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(reloadFromFirstPage, 300)
}

function handlePageChange(page: number): void {
  pagination.page = page
  void loadRecords()
}

function handlePageSizeChange(size: number): void {
  pagination.page_size = size
  reloadFromFirstPage()
}

async function openDetail(row: AffiliateWithdrawRequest): Promise<void> {
  selected.value = { ...row }
  action.value = null
  note.value = ''
  detailOpen.value = true
  detailLoading.value = true
  try {
    selected.value = await affiliatesAPI.getWithdrawRequest(row.id)
  } catch (error) {
    appStore.showError(errorMessage(error, t('admin.affiliates.errors.loadFailed')))
  } finally {
    detailLoading.value = false
  }
}

function closeDetail(): void {
  if (submitting.value) return
  detailOpen.value = false
  selected.value = null
  action.value = null
}

async function copyAccount(): Promise<void> {
  if (selected.value) await copyToClipboard(selected.value.alipay_account, t('admin.affiliates.withdrawals.accountCopied'))
}

async function copyAmount(): Promise<void> {
  if (selected.value) await copyToClipboard(Number(selected.value.amount).toFixed(2), t('admin.affiliates.withdrawals.amountCopied'))
}

async function submitAction(): Promise<void> {
  if (!selected.value || !action.value || submitting.value) return
  const trimmed = note.value.trim()
  if (action.value === 'reject' && !trimmed) {
    appStore.showError(t('admin.affiliates.withdrawals.rejectReasonRequired'))
    return
  }
  submitting.value = true
  try {
    if (action.value === 'paid') {
      await affiliatesAPI.markWithdrawPaid(selected.value.id, trimmed)
      appStore.showSuccess(t('admin.affiliates.withdrawals.paidSuccess'))
    } else {
      await affiliatesAPI.rejectWithdrawRequest(selected.value.id, trimmed)
      appStore.showSuccess(t('admin.affiliates.withdrawals.rejectSuccess'))
    }
    submitting.value = false
    closeDetail()
    await loadRecords()
  } catch (error) {
    appStore.showError(errorMessage(error, t('common.error')))
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void loadRecords()
})
</script>
