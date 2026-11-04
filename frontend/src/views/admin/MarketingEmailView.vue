<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-64">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.marketingEmail.search')"
              class="input"
              @input="handleSearch"
            />
          </div>
          <Select v-model="filters.status" :options="statusFilterOptions" class="w-40" @change="reload" />
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadCampaigns()">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="openCreate">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.marketingEmail.createCampaign') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="campaigns" :loading="loading">
          <template #cell-name="{ row }">
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
              <div class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ row.subject }}</div>
            </div>
          </template>

          <template #cell-status="{ row }">
            <span :class="['badge', statusBadge(row.status)]">{{ t(`admin.marketingEmail.status.${row.status}`) }}</span>
            <div v-if="row.error_message" class="mt-1 max-w-48 truncate text-xs text-red-500" :title="row.error_message">
              {{ row.error_message }}
            </div>
          </template>

          <template #cell-audience="{ row }">
            <span class="text-sm text-gray-600 dark:text-gray-300">{{ audienceSummary(row) }}</span>
          </template>

          <template #cell-progress="{ row }">
            <div v-if="row.total_count > 0" class="w-44">
              <div class="flex justify-between text-xs text-gray-600 dark:text-gray-300">
                <span>{{ row.sent_count }} / {{ row.total_count }}</span>
                <span>{{ progressPercent(row) }}%</span>
              </div>
              <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div class="h-full bg-primary-500" :style="{ width: progressPercent(row) + '%' }"></div>
              </div>
              <div v-if="row.failed_count || row.skipped_count" class="mt-1 text-xs text-gray-500">
                {{ t('admin.marketingEmail.progressDetail', { failed: row.failed_count, skipped: row.skipped_count }) }}
              </div>
            </div>
            <span v-else class="text-sm text-gray-400">-</span>
          </template>

          <template #cell-created_at="{ row }">
            <div class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(row.created_at) }}</div>
            <div v-if="row.finished_at" class="text-xs text-gray-400">
              {{ t('admin.marketingEmail.finishedAt') }} {{ formatDateTime(row.finished_at) }}
            </div>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center space-x-1">
              <button v-if="row.status === 'draft'" :class="actionClass" :title="t('common.edit')" @click="openEdit(row)">
                <Icon name="edit" size="sm" />
              </button>
              <button
                v-if="row.status === 'draft'"
                :class="actionClass"
                :title="t('admin.marketingEmail.actions.send')"
                @click="requestSend(row)"
              >
                <Icon name="mail" size="sm" />
              </button>
              <button
                v-if="row.status === 'pending' || row.status === 'running'"
                :class="actionClass"
                :title="t('admin.marketingEmail.actions.cancel')"
                @click="requestCancel(row)"
              >
                <Icon name="ban" size="sm" />
              </button>
              <button v-if="row.total_count > 0" :class="actionClass" :title="t('admin.marketingEmail.actions.recipients')" @click="openRecipients(row)">
                <Icon name="users" size="sm" />
              </button>
              <button :class="actionClass" :title="t('admin.marketingEmail.actions.duplicate')" @click="openDuplicate(row)">
                <Icon name="copy" size="sm" />
              </button>
              <button
                v-if="row.status !== 'pending' && row.status !== 'running'"
                :class="dangerActionClass"
                :title="t('common.delete')"
                @click="requestDelete(row)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('empty.noData')"
              :description="t('admin.marketingEmail.empty')"
              :action-text="t('admin.marketingEmail.createCampaign')"
              @action="openCreate"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="(p: number) => { pagination.page = p; loadCampaigns() }"
          @update:pageSize="(s: number) => { pagination.page_size = s; reload() }"
        />
      </template>
    </TablePageLayout>

    <CampaignEditorDialog
      :show="showEditor"
      :source="editorSource"
      :editing-id="editingId"
      @close="showEditor = false"
      @saved="handleSaved"
    />

    <CampaignRecipientsDialog :show="showRecipients" :campaign="recipientsCampaign" @close="showRecipients = false" />

    <ConfirmDialog
      :show="!!sendTarget"
      :title="t('admin.marketingEmail.confirmSend.title')"
      :message="sendMessage"
      :confirm-text="t('admin.marketingEmail.actions.send')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmSend"
      @cancel="sendTarget = null"
    />

    <ConfirmDialog
      :show="!!cancelTarget"
      :title="t('admin.marketingEmail.actions.cancel')"
      :message="t('admin.marketingEmail.confirmCancel')"
      :confirm-text="t('common.confirm')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmCancel"
      @cancel="cancelTarget = null"
    />

    <ConfirmDialog
      :show="!!deleteTarget"
      :title="t('common.delete')"
      :message="t('admin.marketingEmail.confirmDelete')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="deleteTarget = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import type { EmailCampaign } from '@/api/admin/emailCampaigns'
import type { Column } from '@/components/common/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import CampaignEditorDialog from '@/components/admin/marketingEmail/CampaignEditorDialog.vue'
import CampaignRecipientsDialog from '@/components/admin/marketingEmail/CampaignRecipientsDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()

const actionClass =
  'flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-600 dark:hover:text-gray-300'
const dangerActionClass =
  'flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400'

const campaigns = ref<EmailCampaign[]>([])
const loading = ref(false)
const filters = reactive({ status: '' })
const searchQuery = ref('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })

const statusKeys = ['draft', 'pending', 'running', 'completed', 'canceled', 'failed'] as const
const statusFilterOptions = computed(() => [
  { value: '', label: t('admin.marketingEmail.allStatus') },
  ...statusKeys.map((s) => ({ value: s, label: t(`admin.marketingEmail.status.${s}`) }))
])

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.marketingEmail.columns.name') },
  { key: 'status', label: t('admin.marketingEmail.columns.status') },
  { key: 'audience', label: t('admin.marketingEmail.columns.audience') },
  { key: 'progress', label: t('admin.marketingEmail.columns.progress') },
  { key: 'created_at', label: t('admin.marketingEmail.columns.createdAt') },
  { key: 'actions', label: t('admin.marketingEmail.columns.actions') }
])

function statusBadge(status: string) {
  switch (status) {
    case 'completed':
      return 'badge-success'
    case 'running':
    case 'pending':
      return 'badge-primary'
    case 'failed':
      return 'badge-danger'
    case 'canceled':
      return 'badge-warning'
    default:
      return 'badge-gray'
  }
}

function progressPercent(row: EmailCampaign) {
  if (!row.total_count) return 0
  const done = row.sent_count + row.failed_count + row.skipped_count
  return Math.min(100, Math.round((done / row.total_count) * 100))
}

function audienceSummary(row: EmailCampaign) {
  const a = row.audience || {}
  const parts: string[] = []
  if (a.inactive_days) parts.push(t('admin.marketingEmail.summary.inactive', { days: a.inactive_days }))
  if (a.never_used_api) parts.push(t('admin.marketingEmail.summary.neverUsedApi'))
  if (a.balance_min !== undefined || a.balance_max !== undefined) {
    parts.push(t('admin.marketingEmail.summary.balance', { min: a.balance_min ?? '-', max: a.balance_max ?? '-' }))
  }
  if (a.registered_from || a.registered_to) parts.push(t('admin.marketingEmail.summary.registered'))
  const explicit = (a.user_ids?.length || 0) + (a.emails?.length || 0)
  if (explicit) parts.push(t('admin.marketingEmail.summary.explicit', { count: explicit }))
  return parts.length ? parts.join(' · ') : t('admin.marketingEmail.summary.all')
}

// ===== list =====
let controller: AbortController | null = null
async function loadCampaigns(silent = false) {
  controller?.abort()
  const current = new AbortController()
  controller = current
  if (!silent) loading.value = true
  try {
    const res = await adminAPI.emailCampaigns.list(
      pagination.page,
      pagination.page_size,
      { status: filters.status || undefined, search: searchQuery.value || undefined },
      { signal: current.signal }
    )
    if (controller !== current) return
    campaigns.value = res.items
    pagination.total = res.total
  } catch (error: any) {
    if (current.signal.aborted || error?.name === 'AbortError' || error?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(error, t('admin.marketingEmail.failed')))
  } finally {
    if (controller === current) {
      loading.value = false
      controller = null
    }
  }
}

function reload() {
  pagination.page = 1
  loadCampaigns()
}

let searchTimer: number | null = null
function handleSearch() {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(reload, 300)
}

// Refresh progress while any campaign is still sending.
let pollTimer: number | null = null
onMounted(() => {
  loadCampaigns()
  pollTimer = window.setInterval(() => {
    if (campaigns.value.some((c) => c.status === 'pending' || c.status === 'running')) loadCampaigns(true)
  }, 5000)
})
onUnmounted(() => {
  if (pollTimer) window.clearInterval(pollTimer)
  if (searchTimer) window.clearTimeout(searchTimer)
  controller?.abort()
})

// ===== editor =====
const showEditor = ref(false)
const editorSource = ref<EmailCampaign | null>(null)
const editingId = ref<number | null>(null)

function openCreate() {
  editorSource.value = null
  editingId.value = null
  showEditor.value = true
}

function openEdit(row: EmailCampaign) {
  editorSource.value = row
  editingId.value = row.id
  showEditor.value = true
}

function openDuplicate(row: EmailCampaign) {
  editorSource.value = row
  editingId.value = null
  showEditor.value = true
}

function handleSaved(campaign: EmailCampaign, send: boolean) {
  showEditor.value = false
  appStore.showSuccess(t('admin.marketingEmail.saved'))
  loadCampaigns()
  if (send) requestSend(campaign)
}

// ===== recipients =====
const showRecipients = ref(false)
const recipientsCampaign = ref<EmailCampaign | null>(null)
function openRecipients(row: EmailCampaign) {
  recipientsCampaign.value = row
  showRecipients.value = true
}

// ===== send =====
const sendTarget = ref<EmailCampaign | null>(null)
const sendCount = ref<number | null>(null)
const sendMessage = computed(() => {
  const c = sendTarget.value
  if (!c) return ''
  if (sendCount.value === null) return t('admin.marketingEmail.confirmSend.counting')
  const minutes = Math.max(1, Math.ceil(sendCount.value / Math.max(1, c.throttle_per_minute)))
  return t('admin.marketingEmail.confirmSend.message', {
    count: sendCount.value,
    subject: c.subject,
    rate: c.throttle_per_minute,
    minutes
  })
})

async function requestSend(row: EmailCampaign) {
  sendTarget.value = row
  sendCount.value = null
  try {
    const res = await adminAPI.emailCampaigns.previewAudience(row.audience || {})
    if (sendTarget.value?.id === row.id) sendCount.value = res.count
  } catch (error) {
    sendTarget.value = null
    appStore.showError(extractApiErrorMessage(error, t('admin.marketingEmail.failed')))
  }
}

async function confirmSend() {
  const target = sendTarget.value
  if (!target || sendCount.value === null) return
  sendTarget.value = null
  if (sendCount.value === 0) {
    appStore.showError(t('admin.marketingEmail.noAudience'))
    return
  }
  try {
    await adminAPI.emailCampaigns.send(target.id)
    appStore.showSuccess(t('admin.marketingEmail.sendQueued'))
    loadCampaigns()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.marketingEmail.failed')))
  }
}

// ===== cancel / delete =====
const cancelTarget = ref<EmailCampaign | null>(null)
function requestCancel(row: EmailCampaign) {
  cancelTarget.value = row
}
async function confirmCancel() {
  const target = cancelTarget.value
  cancelTarget.value = null
  if (!target) return
  try {
    await adminAPI.emailCampaigns.cancel(target.id)
    loadCampaigns()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.marketingEmail.failed')))
  }
}

const deleteTarget = ref<EmailCampaign | null>(null)
function requestDelete(row: EmailCampaign) {
  deleteTarget.value = row
}
async function confirmDelete() {
  const target = deleteTarget.value
  deleteTarget.value = null
  if (!target) return
  try {
    await adminAPI.emailCampaigns.delete(target.id)
    loadCampaigns()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.marketingEmail.failed')))
  }
}
</script>
