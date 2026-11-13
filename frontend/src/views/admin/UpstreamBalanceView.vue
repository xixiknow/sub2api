<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Filters -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <input
            v-model="search"
            type="text"
            class="input w-56"
            :placeholder="t('admin.upstreamBalanceOps.search')"
            @keyup.enter="loadOverview"
          />
          <Select v-model="statusFilter" :options="statusOptions" class="w-36" @change="loadOverview" />
          <Select v-model="platformFilter" :options="platformOptions" class="w-40" @change="loadOverview" />
          <Select v-model="groupFilter" :options="groupOptions" class="w-40" @change="loadOverview" />
          <div class="ml-auto flex gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadOverview">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-secondary" :disabled="refreshingAll" @click="refreshAll">
              <Icon name="refresh" size="sm" class="mr-1" :class="refreshingAll ? 'animate-spin' : ''" />
              {{ t('admin.upstreamBalanceOps.refreshAll') }}
            </button>
            <button class="btn btn-primary" @click="showSettings = true">
              <Icon name="cog" size="sm" class="mr-1" />
              {{ t('admin.upstreamBalanceOps.settings') }}
            </button>
          </div>
        </div>
      </div>

      <!-- KPI -->
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div v-for="kpi in kpis" :key="kpi.key" class="card p-4">
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ kpi.label }}</p>
          <p class="mt-1 text-xl font-semibold" :class="kpi.class">{{ kpi.value }}</p>
          <p v-if="kpi.sub" class="mt-0.5 text-xs text-gray-400">{{ kpi.sub }}</p>
        </div>
      </div>

      <div class="card p-4">
        <div class="mb-4 flex rounded-lg border border-gray-200 p-0.5 dark:border-dark-600" style="width: fit-content">
          <button
            v-for="tab in tabs"
            :key="tab"
            class="rounded-md px-3 py-1 text-sm"
            :class="activeTab === tab ? 'bg-primary-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
            @click="switchTab(tab)"
          >
            {{ t(`admin.upstreamBalanceOps.tabs.${tab}`) }}
          </button>
          <button v-if="activeTab === 'wallets'" class="btn btn-secondary btn-sm ml-3" :disabled="!wallets.length" @click="exportCSV">
            <Icon name="download" size="sm" />
          </button>
        </div>

        <!-- Wallets -->
        <div v-if="activeTab === 'wallets'" class="overflow-x-auto">
          <table class="min-w-full whitespace-nowrap text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-left text-xs text-gray-500 dark:border-dark-600 dark:text-dark-400">
                <th class="px-3 py-2">{{ t('admin.upstreamBalanceOps.columns.status') }}</th>
                <th class="px-3 py-2">{{ t('admin.upstreamBalanceOps.columns.upstream') }}</th>
                <th class="px-3 py-2 text-right">{{ t('admin.upstreamBalanceOps.columns.balance') }}</th>
                <th class="px-3 py-2 text-right">{{ t('admin.upstreamBalanceOps.columns.burn24h') }}</th>
                <th class="px-3 py-2 text-right">{{ t('admin.upstreamBalanceOps.columns.daysLeft') }}</th>
                <th class="px-3 py-2 text-right">{{ t('admin.upstreamBalanceOps.columns.suggest') }}</th>
                <th class="px-3 py-2 text-right">{{ t('admin.upstreamBalanceOps.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && !wallets.length">
                <td :colspan="COLS" class="py-8 text-center"><LoadingSpinner /></td>
              </tr>
              <tr v-else-if="!wallets.length">
                <td :colspan="COLS" class="py-8 text-center text-gray-500">{{ t('admin.upstreamBalanceOps.noData') }}</td>
              </tr>
              <template v-for="w in wallets" v-else :key="w.key">
                <tr
                  class="cursor-pointer border-b border-gray-100 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800"
                  @click="toggleExpand(w)"
                >
                  <td class="px-3 py-2">
                    <span class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(w.status)">
                      <span class="h-1.5 w-1.5 rounded-full" :class="statusDot(w.status)" />
                      {{ t(`admin.upstreamBalanceOps.status.${w.status}`) }}
                    </span>
                    <span v-if="w.idle" class="ml-1 rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400" :title="t('admin.upstreamBalanceOps.idleHint')">
                      {{ t('admin.upstreamBalanceOps.idle') }}
                    </span>
                  </td>
                  <td class="max-w-xs px-3 py-2">
                    <div class="flex items-center gap-1.5 truncate font-medium text-gray-900 dark:text-white" :title="w.members.map((m) => m.name).join('\n')">
                      <Icon :name="expandedKey === w.key ? 'chevronDown' : 'chevronRight'" size="xs" class="shrink-0 text-gray-400" />
                      <span class="truncate">{{ walletName(w) }}</span>
                    </div>
                    <div class="flex flex-wrap items-center gap-x-2 pl-5 text-xs text-gray-400">
                      <span v-if="w.label && w.site">{{ w.site }}</span>
                      <span>{{ w.members.length > 1 ? t('admin.upstreamBalanceOps.sharedWallet', { count: w.members.length }) : w.members[0].name }}</span>
                      <span v-if="w.members.some((m) => m.paused_by_balance)" class="text-red-500">{{ t('admin.upstreamBalanceOps.paused') }}</span>
                      <span v-if="w.stale" class="text-amber-500">{{ t('admin.upstreamBalanceOps.stale') }}</span>
                      <span v-if="w.alert_disabled">{{ t('admin.upstreamBalanceOps.alertOff') }}</span>
                      <span v-if="w.fetch_status && w.fetch_status !== 'ok'" class="text-red-500">{{ w.fetch_error || w.fetch_status }}</span>
                    </div>
                  </td>
                  <td class="px-3 py-2 text-right font-medium" :title="w.fetched_at ? t('admin.upstreamBalanceOps.fetchedAt', { time: formatTime(w.fetched_at) }) : ''">
                    <template v-if="w.unlimited">{{ t('admin.upstreamBalanceOps.status.unlimited') }}</template>
                    <template v-else-if="w.balance !== undefined">${{ money(w.balance) }}</template>
                    <template v-else>-</template>
                  </td>
                  <td class="px-3 py-2 text-right" :title="w.daily_burn_source ? t(`admin.upstreamBalanceOps.source.${w.daily_burn_source}`) : ''">
                    {{ w.daily_burn !== undefined ? `$${money(w.daily_burn)}` : '-' }}
                    <span v-if="w.daily_burn_source === 'usage'" class="text-xs text-gray-400">*</span>
                  </td>
                  <td
                    class="px-3 py-2 text-right font-medium"
                    :class="daysClass(w)"
                    :title="w.exhaust_at ? t('admin.upstreamBalanceOps.exhaustAtHint', { time: formatTime(w.exhaust_at) }) : ''"
                  >
                    {{ daysText(w) }}
                  </td>
                  <td class="px-3 py-2 text-right" :title="t('admin.upstreamBalanceOps.suggestHint', { days: w.thresholds.target_days })">
                    {{ w.suggest_recharge ? `$${money(w.suggest_recharge)}` : '-' }}
                  </td>
                  <td class="whitespace-nowrap px-3 py-2 text-right" @click.stop>
                    <button class="btn btn-ghost btn-sm" :title="t('admin.upstreamBalanceOps.actions.refresh')" :disabled="refreshingKey === w.key" @click="refreshWallet(w)">
                      <Icon name="refresh" size="xs" :class="refreshingKey === w.key ? 'animate-spin' : ''" />
                    </button>
                    <button class="btn btn-ghost btn-sm" @click="openMerge(w)">{{ t('admin.upstreamBalanceOps.actions.merge') }}</button>
                    <button class="btn btn-ghost btn-sm" @click="openOverride(w)">{{ t('admin.upstreamBalanceOps.actions.threshold') }}</button>
                    <router-link class="btn btn-ghost btn-sm" :to="{ path: '/admin/accounts', query: { search: w.members[0].name } }">
                      {{ t('admin.upstreamBalanceOps.actions.account') }}
                    </router-link>
                  </td>
                </tr>
                <tr v-if="expandedKey === w.key" class="border-b border-gray-100 bg-gray-50/60 dark:border-dark-700 dark:bg-dark-800/60">
                  <td :colspan="COLS" class="space-y-4 px-4 py-3">
                    <!-- Metrics -->
                    <div>
                      <div class="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.upstreamBalanceOps.detail.metrics') }}</div>
                      <div class="flex flex-wrap gap-x-8 gap-y-1 text-xs text-gray-600 dark:text-gray-300">
                        <span :title="w.burn_7d_source ? t(`admin.upstreamBalanceOps.source.${w.burn_7d_source}`) : ''">
                          {{ t('admin.upstreamBalanceOps.columns.burn7d') }}：{{ w.burn_7d !== undefined ? `$${money(w.burn_7d)}` : '-' }}
                        </span>
                        <span>{{ t('admin.upstreamBalanceOps.columns.exhaustAt') }}：{{ w.exhaust_at ? formatTime(w.exhaust_at) : '-' }}</span>
                        <span>{{ t('admin.upstreamBalanceOps.columns.share') }}：{{ percent(w.traffic_share) }}</span>
                        <span v-if="w.fetched_at">{{ t('admin.upstreamBalanceOps.fetchedAt', { time: formatTime(w.fetched_at) }) }}</span>
                      </div>
                    </div>
                    <!-- Accounts -->
                    <div>
                      <div class="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.upstreamBalanceOps.detail.accounts') }}</div>
                      <table class="min-w-full text-xs">
                        <thead>
                          <tr class="text-left text-gray-500 dark:text-dark-400">
                            <th class="py-1 pr-4 font-normal">{{ t('admin.upstreamBalanceOps.detail.account') }}</th>
                            <th class="py-1 pr-4 font-normal">{{ t('admin.upstreamBalanceOps.detail.platform') }}</th>
                            <th class="py-1 pr-4 font-normal">{{ t('admin.upstreamBalanceOps.detail.schedule') }}</th>
                            <th class="py-1 pr-4 text-right font-normal">{{ t('admin.upstreamBalanceOps.detail.cost24h') }}</th>
                            <th class="py-1 pr-4 text-right font-normal">{{ t('admin.upstreamBalanceOps.detail.cost7d') }}</th>
                            <th v-if="w.label" class="py-1" />
                          </tr>
                        </thead>
                        <tbody>
                          <tr v-for="m in w.members" :key="m.account_id" class="text-gray-700 dark:text-gray-300">
                            <td class="py-1 pr-4">{{ m.name }}</td>
                            <td class="py-1 pr-4">{{ m.platform }}</td>
                            <td class="py-1 pr-4">
                              <span v-if="m.paused_by_balance" class="text-red-500">{{ t('admin.upstreamBalanceOps.paused') }}</span>
                              <span v-else-if="m.schedulable" class="text-emerald-600">{{ t('admin.upstreamBalanceOps.detail.schedulable') }}</span>
                              <span v-else class="text-gray-400">{{ t('admin.upstreamBalanceOps.detail.unschedulable') }}</span>
                            </td>
                            <td class="py-1 pr-4 text-right">${{ money(m.cost_24h) }}</td>
                            <td class="py-1 pr-4 text-right">${{ money(m.cost_7d) }}</td>
                            <td v-if="w.label" class="py-1 text-right">
                              <button class="text-primary-600 hover:underline dark:text-primary-400" :disabled="mergeSaving" @click.stop="removeFromWallet(m.account_id)">
                                {{ t('admin.upstreamBalanceOps.detail.remove') }}
                              </button>
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                    <!-- History -->
                    <div>
                      <div class="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.upstreamBalanceOps.history') }}</div>
                      <div v-if="historyLoading" class="flex h-48 items-center justify-center"><LoadingSpinner /></div>
                      <div v-else-if="historyChart" class="h-56"><Line :data="historyChart" :options="historyOptions" /></div>
                      <div v-else class="flex h-24 items-center justify-center text-xs text-gray-500">{{ t('admin.upstreamBalanceOps.noHistory') }}</div>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
          <p class="mt-2 text-xs text-gray-400">* {{ t('admin.upstreamBalanceOps.source.usage') }}</p>
        </div>

        <!-- Alerts -->
        <div v-else class="overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-left text-xs text-gray-500 dark:border-dark-600 dark:text-dark-400">
                <th class="px-3 py-2">{{ t('admin.upstreamBalanceOps.alerts.time') }}</th>
                <th class="px-3 py-2">{{ t('admin.upstreamBalanceOps.alerts.level') }}</th>
                <th class="px-3 py-2">{{ t('admin.upstreamBalanceOps.alerts.content') }}</th>
                <th class="px-3 py-2">{{ t('admin.upstreamBalanceOps.alerts.phones') }}</th>
                <th class="px-3 py-2">{{ t('admin.upstreamBalanceOps.alerts.result') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="alertsLoading">
                <td colspan="5" class="py-8 text-center"><LoadingSpinner /></td>
              </tr>
              <tr v-else-if="!alerts.length">
                <td colspan="5" class="py-8 text-center text-gray-500">{{ t('admin.upstreamBalanceOps.alerts.empty') }}</td>
              </tr>
              <tr v-for="a in alerts" v-else :key="a.id" class="border-b border-gray-100 dark:border-dark-700">
                <td class="whitespace-nowrap px-3 py-2 text-gray-600 dark:text-gray-300">{{ formatTime(a.created_at) }}</td>
                <td class="px-3 py-2">
                  <span v-if="a.level === 'test'" class="text-xs text-gray-500">{{ t('admin.upstreamBalanceOps.alerts.test') }}</span>
                  <span v-else class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs" :class="statusClass(a.level)">
                    {{ t(`admin.upstreamBalanceOps.status.${a.level}`) }}
                  </span>
                </td>
                <td class="px-3 py-2 text-gray-900 dark:text-white">{{ a.content }}</td>
                <td class="px-3 py-2 text-gray-600 dark:text-gray-300">{{ a.provider }} · {{ a.phones }}</td>
                <td class="px-3 py-2">
                  <span v-if="a.status === 'sent'" class="text-emerald-600">{{ t('admin.upstreamBalanceOps.alerts.sent') }}</span>
                  <span v-else class="text-red-600" :title="a.error">{{ t('admin.upstreamBalanceOps.alerts.failed') }}: {{ a.error }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <UpstreamBalanceSettingsDialog :show="showSettings" @close="showSettings = false" @saved="loadOverview" />

    <!-- Account override -->
    <BaseDialog :show="!!overrideWallet" :title="t('admin.upstreamBalanceOps.override.title')" @close="overrideWallet = null">
      <div v-if="overrideWallet" class="space-y-4">
        <p class="text-xs text-gray-500">{{ t('admin.upstreamBalanceOps.override.hint') }}</p>
        <div>
          <label class="input-label">{{ t('admin.upstreamBalanceOps.actions.account') }}</label>
          <select v-model.number="overrideAccountId" class="input">
            <option v-for="m in overrideWallet.members" :key="m.account_id" :value="m.account_id">{{ m.name }}</option>
          </select>
        </div>
        <div class="grid grid-cols-3 gap-3">
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.warnDays') }}</label>
            <input v-model="overrideForm.warn_days" type="number" min="0" step="0.5" class="input" :placeholder="String(overview?.thresholds.warn_days ?? '')" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.criticalDays') }}</label>
            <input v-model="overrideForm.critical_days" type="number" min="0" step="0.5" class="input" :placeholder="String(overview?.thresholds.critical_days ?? '')" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.columns.balance') }} &lt; $</label>
            <input v-model="overrideForm.min_balance" type="number" min="0" step="1" class="input" :placeholder="String(overview?.thresholds.min_balance ?? '')" />
          </div>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="overrideForm.disabled" type="checkbox" class="rounded" />
          {{ t('admin.upstreamBalanceOps.override.disabled') }}
        </label>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" @click="overrideWallet = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="overrideSaving" @click="saveOverride">{{ t('admin.upstreamBalanceOps.config.save') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Merge wallet -->
    <BaseDialog :show="!!mergeWallet" :title="t('admin.upstreamBalanceOps.merge.title')" @close="mergeWallet = null">
      <div v-if="mergeWallet" class="space-y-4">
        <p class="text-xs text-gray-500">{{ t('admin.upstreamBalanceOps.merge.hint') }}</p>
        <div>
          <label class="input-label">{{ t('admin.upstreamBalanceOps.merge.label') }}</label>
          <input v-model="mergeLabel" type="text" maxlength="64" class="input" :placeholder="t('admin.upstreamBalanceOps.merge.placeholder')" />
          <div v-if="existingLabels.length" class="mt-2 flex flex-wrap items-center gap-1.5 text-xs">
            <span class="text-gray-500">{{ t('admin.upstreamBalanceOps.merge.existing') }}</span>
            <button
              v-for="label in existingLabels"
              :key="label"
              type="button"
              class="rounded-full border px-2 py-0.5"
              :class="mergeLabel === label ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-gray-200 text-gray-600 hover:bg-gray-100 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'"
              @click="mergeLabel = label"
            >
              {{ label }}
            </button>
          </div>
        </div>
        <div class="text-xs text-gray-500">
          <div class="mb-1">{{ t('admin.upstreamBalanceOps.merge.members') }}</div>
          <div class="text-gray-700 dark:text-gray-300">{{ mergeWallet.members.map((m) => m.name).join('、') }}</div>
        </div>
      </div>
      <template #footer>
        <div class="flex items-center justify-between gap-2">
          <button v-if="mergeWallet?.label" class="btn btn-secondary" :disabled="mergeSaving" @click="saveMerge('')">
            {{ t('admin.upstreamBalanceOps.merge.clear') }}
          </button>
          <span v-else />
          <div class="flex gap-2">
            <button class="btn btn-secondary" @click="mergeWallet = null">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="mergeSaving || !mergeLabel.trim()" @click="saveMerge(mergeLabel)">
              {{ t('admin.upstreamBalanceOps.config.save') }}
            </button>
          </div>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { adminAPI } from '@/api/admin'
import type {
  UpstreamBalanceAlert,
  UpstreamBalanceHistory,
  UpstreamBalanceOverview,
  UpstreamWallet,
  UpstreamWalletStatus
} from '@/api/admin/upstreamBalance'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import UpstreamBalanceSettingsDialog from '@/components/admin/upstream-balance/UpstreamBalanceSettingsDialog.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const { t } = useI18n()
const appStore = useAppStore()

type Tab = 'wallets' | 'alerts'
const tabs: Tab[] = ['wallets', 'alerts']
const activeTab = ref<Tab>('wallets')

const search = ref('')
const statusFilter = ref('')
const platformFilter = ref('')
const groupFilter = ref('')
const groups = ref<AdminGroup[]>([])

const overview = ref<UpstreamBalanceOverview | null>(null)
const wallets = computed(() => overview.value?.wallets ?? [])
const loading = ref(false)
const refreshingAll = ref(false)
const refreshingKey = ref('')
const showSettings = ref(false)

const alerts = ref<UpstreamBalanceAlert[]>([])
const alertsLoading = ref(false)

const expandedKey = ref('')
const history = ref<UpstreamBalanceHistory | null>(null)
const historyLoading = ref(false)

const statusOptions = computed(() => [
  { value: '', label: t('admin.upstreamBalanceOps.allStatus') },
  { value: 'alerting', label: t('admin.upstreamBalanceOps.alertingOnly') },
  ...(['exhausted', 'critical', 'warn', 'unknown', 'ok', 'unlimited'] as UpstreamWalletStatus[]).map((s) => ({
    value: s,
    label: t(`admin.upstreamBalanceOps.status.${s}`)
  }))
])
const platformOptions = computed(() => [{ value: '', label: t('admin.upstreamBalanceOps.allPlatforms') }, ...CONCRETE_PLATFORM_OPTIONS])
const groupOptions = computed(() => [
  { value: '', label: t('admin.upstreamBalanceOps.allGroups') },
  ...groups.value.map((g) => ({ value: String(g.id), label: g.name }))
])

async function loadOverview() {
  loading.value = true
  try {
    overview.value = await adminAPI.upstreamBalance.getOverview({
      search: search.value.trim() || undefined,
      status: statusFilter.value || undefined,
      platform: platformFilter.value || undefined,
      group_id: groupFilter.value ? Number(groupFilter.value) : undefined
    })
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalanceOps.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function loadAlerts() {
  alertsLoading.value = true
  try {
    alerts.value = await adminAPI.upstreamBalance.listAlerts(30)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalanceOps.loadFailed')))
  } finally {
    alertsLoading.value = false
  }
}

function switchTab(tab: Tab) {
  activeTab.value = tab
  if (tab === 'alerts') void loadAlerts()
}

async function refreshAll() {
  refreshingAll.value = true
  try {
    await adminAPI.upstreamBalance.refreshAll()
    appStore.showSuccess(t('admin.upstreamBalanceOps.refreshAllDone'))
    await loadOverview()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalanceOps.loadFailed')))
  } finally {
    refreshingAll.value = false
  }
}

async function refreshWallet(w: UpstreamWallet) {
  refreshingKey.value = w.key
  try {
    await adminAPI.accountInsights.batchRefreshAccountBalance(w.members.map((m) => m.account_id))
    await loadOverview()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalance.refreshFailed')))
  } finally {
    refreshingKey.value = ''
  }
}

async function toggleExpand(w: UpstreamWallet) {
  if (expandedKey.value === w.key) {
    expandedKey.value = ''
    return
  }
  expandedKey.value = w.key
  history.value = null
  historyLoading.value = true
  try {
    history.value = await adminAPI.upstreamBalance.getHistory(w.primary_account_id, 7)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalanceOps.loadFailed')))
  } finally {
    historyLoading.value = false
  }
}

const isDark = computed(() => document.documentElement.classList.contains('dark'))

// 余额曲线按快照时间展示；每日成本挂到当天最后一个快照所在的标签上，避免引入时间轴适配器。
const historyChart = computed(() => {
  const h = history.value
  if (!h || !h.points.length) return null
  const labels = h.points.map((p) => formatTime(p.t))
  const costByDay = new Map(h.daily_cost.map((d) => [d.date, d.cost]))
  const dayOf = (iso: string) => {
    const d = new Date(iso)
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  }
  const cost = h.points.map((p, i) => {
    const day = dayOf(p.t)
    const next = h.points[i + 1]
    return !next || dayOf(next.t) !== day ? costByDay.get(day) ?? null : null
  })
  return {
    labels,
    datasets: [
      { label: t('admin.upstreamBalanceOps.historyBalance'), data: h.points.map((p) => p.balance), borderColor: '#3b82f6', backgroundColor: '#3b82f620', tension: 0.2, fill: true, pointRadius: 1, yAxisID: 'y' },
      { label: t('admin.upstreamBalanceOps.historyCost'), data: cost, borderColor: '#f59e0b', backgroundColor: '#f59e0b', showLine: false, pointRadius: 4, yAxisID: 'y1' }
    ]
  }
})

const historyOptions = computed(() => {
  const tick = isDark.value ? '#9ca3af' : '#6b7280'
  const grid = isDark.value ? '#374151' : '#e5e7eb'
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: { legend: { labels: { color: isDark.value ? '#e5e7eb' : '#374151' } } },
    scales: {
      x: { ticks: { color: tick, maxTicksLimit: 8 }, grid: { color: grid } },
      y: { position: 'left' as const, ticks: { color: tick }, grid: { color: grid } },
      y1: { position: 'right' as const, ticks: { color: tick }, grid: { drawOnChartArea: false } }
    }
  }
})

// ---- account override ----
const overrideWallet = ref<UpstreamWallet | null>(null)
const overrideAccountId = ref(0)
const overrideSaving = ref(false)
const overrideForm = reactive<{ warn_days: string; critical_days: string; min_balance: string; disabled: boolean }>({
  warn_days: '',
  critical_days: '',
  min_balance: '',
  disabled: false
})

function openOverride(w: UpstreamWallet) {
  overrideWallet.value = w
  overrideAccountId.value = w.members[0].account_id
}

watch(overrideAccountId, (id) => {
  const member = overrideWallet.value?.members.find((m) => m.account_id === id)
  const o = member?.override
  overrideForm.warn_days = o?.warn_days != null ? String(o.warn_days) : ''
  overrideForm.critical_days = o?.critical_days != null ? String(o.critical_days) : ''
  overrideForm.min_balance = o?.min_balance != null ? String(o.min_balance) : ''
  overrideForm.disabled = !!o?.disabled
})

function optionalNumber(v: string | number): number | null {
  if (v === '' || v === null || v === undefined) return null
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

async function saveOverride() {
  overrideSaving.value = true
  try {
    await adminAPI.upstreamBalance.updateAccountAlert(overrideAccountId.value, {
      warn_days: optionalNumber(overrideForm.warn_days),
      critical_days: optionalNumber(overrideForm.critical_days),
      min_balance: optionalNumber(overrideForm.min_balance),
      disabled: overrideForm.disabled
    })
    appStore.showSuccess(t('admin.upstreamBalanceOps.override.saved'))
    overrideWallet.value = null
    await loadOverview()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalanceOps.config.saveFailed')))
  } finally {
    overrideSaving.value = false
  }
}

// ---- wallet merge ----
const mergeWallet = ref<UpstreamWallet | null>(null)
const mergeLabel = ref('')
const mergeSaving = ref(false)
const existingLabels = computed(() => [...new Set(wallets.value.map((w) => w.label).filter((l): l is string => !!l))].sort())

function openMerge(w: UpstreamWallet) {
  mergeWallet.value = w
  mergeLabel.value = w.label ?? ''
}

async function updateLabel(accountIds: number[], label: string, successKey: string): Promise<boolean> {
  mergeSaving.value = true
  try {
    await adminAPI.upstreamBalance.updateWalletLabel(accountIds, label.trim())
    appStore.showSuccess(t(successKey))
    await loadOverview()
    return true
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalanceOps.merge.failed')))
    return false
  } finally {
    mergeSaving.value = false
  }
}

async function saveMerge(label: string) {
  if (!mergeWallet.value) return
  const ids = mergeWallet.value.members.map((m) => m.account_id)
  if (await updateLabel(ids, label, 'admin.upstreamBalanceOps.merge.saved')) {
    mergeWallet.value = null
    expandedKey.value = ''
  }
}

async function removeFromWallet(accountId: number) {
  if (await updateLabel([accountId], '', 'admin.upstreamBalanceOps.merge.removed')) {
    expandedKey.value = ''
  }
}

// ---- formatting ----
const COLS = 7
function walletName(w: UpstreamWallet): string {
  return w.label || w.site || w.members[0]?.name || '-'
}
function money(v: number): string {
  return v.toFixed(Math.abs(v) >= 100 ? 2 : Math.abs(v) >= 1 ? 2 : 4)
}
function percent(v: number): string {
  return `${(v * 100).toFixed(1)}%`
}
function formatTime(iso: string): string {
  const d = new Date(iso)
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}
function daysText(w: UpstreamWallet): string {
  if (w.unlimited) return '∞'
  if (w.idle && w.balance !== undefined && w.balance > 0) return t('admin.upstreamBalanceOps.idle')
  if (w.days_left !== undefined) return t('admin.upstreamBalanceOps.days', { value: w.days_left.toFixed(1) })
  if (w.balance !== undefined && w.balance > 0) return '∞'
  return '-'
}
function daysClass(w: UpstreamWallet): string {
  switch (w.status) {
    case 'exhausted':
    case 'critical':
      return 'text-red-600 dark:text-red-400'
    case 'warn':
      return 'text-amber-600 dark:text-amber-400'
    default:
      return 'text-gray-900 dark:text-white'
  }
}
function statusClass(status: string): string {
  switch (status) {
    case 'exhausted':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
    case 'critical':
      return 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300'
    case 'warn':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'ok':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
}
function statusDot(status: string): string {
  switch (status) {
    case 'exhausted':
      return 'bg-red-500'
    case 'critical':
      return 'bg-orange-500'
    case 'warn':
      return 'bg-amber-500'
    case 'ok':
      return 'bg-emerald-500'
    default:
      return 'bg-gray-400'
  }
}

const kpis = computed(() => {
  const s = overview.value?.summary
  const alerting = s ? s.exhausted + s.critical + s.warn : 0
  return [
    {
      key: 'balance',
      label: t('admin.upstreamBalanceOps.kpi.totalBalance'),
      value: s ? `$${money(s.total_balance)}` : '-',
      class: 'text-gray-900 dark:text-white',
      sub: s ? t('admin.upstreamBalanceOps.kpi.wallets', { count: s.wallets }) : ''
    },
    { key: 'burn', label: t('admin.upstreamBalanceOps.kpi.dailyBurn'), value: s ? `$${money(s.daily_burn)}` : '-', class: 'text-gray-900 dark:text-white', sub: '' },
    {
      key: 'days',
      label: t('admin.upstreamBalanceOps.kpi.daysLeft'),
      value: s ? (s.days_left !== undefined ? t('admin.upstreamBalanceOps.days', { value: s.days_left.toFixed(1) }) : '∞') : '-',
      class: 'text-gray-900 dark:text-white',
      sub: ''
    },
    {
      key: 'alerting',
      label: t('admin.upstreamBalanceOps.kpi.alerting'),
      value: s ? `${alerting} / ${s.exhausted}` : '-',
      class: alerting > 0 ? 'text-red-600 dark:text-red-400' : 'text-gray-900 dark:text-white',
      sub: s && s.unknown ? t('admin.upstreamBalanceOps.kpi.unknown', { count: s.unknown }) : ''
    }
  ]
})

function exportCSV() {
  const header = ['status', 'wallet', 'accounts', 'site', 'balance', 'daily_burn', 'daily_burn_source', 'burn_7d', 'days_left', 'exhaust_at', 'suggest_recharge', 'traffic_share']
  const lines = [header.join(',')]
  const q = (s: string) => `"${s.replace(/"/g, '""')}"`
  for (const w of wallets.value) {
    lines.push([
      w.status,
      q(walletName(w)),
      q(w.members.map((m) => m.name).join(' / ')),
      q(w.site),
      w.balance ?? '',
      w.daily_burn ?? '',
      w.daily_burn_source ?? '',
      w.burn_7d ?? '',
      w.days_left ?? '',
      w.exhaust_at ?? '',
      w.suggest_recharge ?? '',
      w.traffic_share.toFixed(4)
    ].join(','))
  }
  const blob = new Blob(['﻿' + lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `upstream-balance-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(async () => {
  void loadOverview()
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch {
    // 过滤项加载失败不影响主数据
  }
})
</script>
