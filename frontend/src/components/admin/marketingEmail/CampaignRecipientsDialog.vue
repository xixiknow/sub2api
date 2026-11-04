<template>
  <BaseDialog :show="show" :title="t('admin.marketingEmail.recipients.title', { name: campaign?.name || '' })" width="extra-wide" @close="emit('close')">
    <div class="mb-3 flex items-center gap-3">
      <select v-model="status" class="input w-40" @change="load(1)">
        <option value="">{{ t('admin.marketingEmail.recipients.all') }}</option>
        <option v-for="s in statuses" :key="s" :value="s">{{ t(`admin.marketingEmail.recipientStatus.${s}`) }}</option>
      </select>
      <button type="button" class="btn btn-secondary" :disabled="loading" @click="load(page)">
        <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
      </button>
    </div>
    <DataTable :columns="columns" :data="items" :loading="loading">
      <template #cell-email="{ row }">
        <div class="text-sm text-gray-900 dark:text-white">{{ row.email }}</div>
        <div class="text-xs text-gray-500">#{{ row.user_id }} {{ row.recipient_name }}</div>
      </template>
      <template #cell-status="{ value }">
        <span :class="['badge', recipientBadge(value)]">{{ t(`admin.marketingEmail.recipientStatus.${value}`) }}</span>
      </template>
      <template #cell-last_activity_at="{ value }">
        <span class="text-sm text-gray-500">{{ value ? formatDateTime(value) : '-' }}</span>
      </template>
      <template #cell-sent_at="{ value }">
        <span class="text-sm text-gray-500">{{ value ? formatDateTime(value) : '-' }}</span>
      </template>
      <template #cell-error="{ value }">
        <span class="line-clamp-2 max-w-xs text-xs text-red-500" :title="value">{{ value }}</span>
      </template>
    </DataTable>
    <Pagination
      v-if="total > 0"
      class="mt-3"
      :page="page"
      :total="total"
      :page-size="pageSize"
      @update:page="load"
      @update:pageSize="(s: number) => { pageSize = s; load(1) }"
    />
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { EmailCampaign, EmailCampaignRecipient } from '@/api/admin/emailCampaigns'
import type { Column } from '@/components/common/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ show: boolean; campaign: EmailCampaign | null }>()
const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()
const appStore = useAppStore()

const statuses = ['pending', 'sent', 'failed', 'skipped'] as const
const items = ref<EmailCampaignRecipient[]>([])
const loading = ref(false)
const status = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const columns = computed<Column[]>(() => [
  { key: 'email', label: t('admin.marketingEmail.recipients.email') },
  { key: 'status', label: t('admin.marketingEmail.columns.status') },
  { key: 'last_activity_at', label: t('admin.marketingEmail.recipients.lastActivity') },
  { key: 'sent_at', label: t('admin.marketingEmail.recipients.sentAt') },
  { key: 'error', label: t('admin.marketingEmail.recipients.error') }
])

function recipientBadge(value: string) {
  if (value === 'sent') return 'badge-success'
  if (value === 'failed') return 'badge-danger'
  if (value === 'skipped') return 'badge-warning'
  return 'badge-gray'
}

async function load(p: number) {
  if (!props.campaign) return
  page.value = p
  loading.value = true
  try {
    const res = await adminAPI.emailCampaigns.listRecipients(props.campaign.id, page.value, pageSize.value, status.value)
    items.value = res.items
    total.value = res.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.marketingEmail.failed')))
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (visible) => {
    if (!visible) return
    status.value = ''
    items.value = []
    void load(1)
  }
)
</script>
