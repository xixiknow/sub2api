<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Filters -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" @change="loadAll" />
          <Select v-model="groupFilter" :options="groupOptions" class="w-44" @change="loadAll" />
          <Select v-model="platformFilter" :options="platformOptions" class="w-44" @change="loadAll" />
          <Select v-model="granularity" :options="granularityOptions" class="w-28" @change="loadTrend" />
          <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <input v-model="includeSubscription" type="checkbox" class="rounded" @change="loadAll" />
            {{ t('admin.profit.includeSubscription') }}
          </label>
          <div class="ml-auto flex gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadAll">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
        <p class="mt-2 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.profit.formulaHint') }}</p>
      </div>

      <!-- KPI -->
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-5">
        <div v-for="kpi in kpis" :key="kpi.key" class="card p-4">
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ kpi.label }}</p>
          <p class="mt-1 text-xl font-semibold" :class="kpi.class">{{ kpi.value }}</p>
          <p v-if="kpi.sub" class="mt-0.5 text-xs text-gray-400">{{ kpi.sub }}</p>
        </div>
      </div>

      <!-- Trend -->
      <div class="card p-4">
        <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.profit.trend') }}</h3>
        <div v-if="trendLoading" class="flex h-64 items-center justify-center"><LoadingSpinner /></div>
        <div v-else-if="trend.length && chartData" class="h-64">
          <Line :data="chartData" :options="chartOptions" />
        </div>
        <div v-else class="flex h-64 items-center justify-center text-sm text-gray-500">{{ t('admin.profit.noData') }}</div>
      </div>

      <!-- Breakdown -->
      <div class="card p-4">
        <div class="mb-4 flex flex-wrap items-center gap-3">
          <div class="flex rounded-lg border border-gray-200 p-0.5 dark:border-dark-600">
            <button
              v-for="dim in dimensions"
              :key="dim"
              class="rounded-md px-3 py-1 text-sm"
              :class="dimension === dim ? 'bg-primary-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
              @click="switchDimension(dim)"
            >
              {{ t(`admin.profit.dimensions.${dim}`) }}
            </button>
          </div>
          <button class="btn btn-secondary ml-auto" :disabled="!sortedBreakdown.length" @click="exportCSV">
            <Icon name="download" size="sm" class="mr-1" />
            {{ t('admin.profit.export') }}
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-left text-xs text-gray-500 dark:border-dark-600 dark:text-dark-400">
                <th class="px-3 py-2">{{ t(`admin.profit.dimensions.${dimension}`) }}</th>
                <th v-if="showRate" class="px-3 py-2">{{ rateLabel }}</th>
                <th
                  v-for="col in metricColumns"
                  :key="col.key"
                  class="cursor-pointer select-none px-3 py-2 text-right"
                  @click="toggleSort(col.key)"
                >
                  {{ col.label }}
                  <span v-if="sortKey === col.key">{{ sortDesc ? '↓' : '↑' }}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="breakdownLoading">
                <td :colspan="10" class="py-8 text-center"><LoadingSpinner /></td>
              </tr>
              <tr v-else-if="!sortedBreakdown.length">
                <td :colspan="10" class="py-8 text-center text-gray-500">{{ t('admin.profit.noData') }}</td>
              </tr>
              <tr
                v-for="row in sortedBreakdown"
                v-else
                :key="row.key"
                class="border-b border-gray-100 dark:border-dark-700"
              >
                <td class="px-3 py-2">
                  <span class="font-medium text-gray-900 dark:text-white">{{ rowName(row) }}</span>
                  <span v-if="row.platform && dimension === 'account'" class="ml-2 text-xs text-gray-400">{{ row.platform }}</span>
                </td>
                <td v-if="showRate" class="px-3 py-2 text-gray-600 dark:text-gray-300">
                  {{ row.rate_multiplier !== undefined ? `×${trimNumber(row.rate_multiplier)}` : '-' }}
                </td>
                <td class="px-3 py-2 text-right">{{ formatNumber(row.requests) }}</td>
                <td class="px-3 py-2 text-right">{{ formatTokensK(row.total_tokens) }}</td>
                <td class="px-3 py-2 text-right">${{ money(row.revenue) }}</td>
                <td class="px-3 py-2 text-right">${{ money(row.cost) }}</td>
                <td class="px-3 py-2 text-right font-medium" :class="profitClass(row.profit)">${{ money(row.profit) }}</td>
                <td class="px-3 py-2 text-right" :class="profitClass(row.profit)">{{ percent(row.margin) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler,
  type TooltipItem
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { adminAPI } from '@/api/admin'
import type { ProfitBreakdownItem, ProfitDimension, ProfitQuery, ProfitSummary, ProfitTrendPoint } from '@/api/admin/profit'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatNumber, formatTokensK } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const { t } = useI18n()
const appStore = useAppStore()

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

const today = new Date()
const startDate = ref(formatLocalDate(new Date(today.getTime() - 6 * 86400000)))
const endDate = ref(formatLocalDate(today))
const granularity = ref<'day' | 'hour'>('day')
const includeSubscription = ref(false)
const groupFilter = ref<string>('')
const platformFilter = ref<string>('')

const groups = ref<AdminGroup[]>([])
const summary = ref<ProfitSummary | null>(null)
const trend = ref<ProfitTrendPoint[]>([])
const breakdown = ref<ProfitBreakdownItem[]>([])
const dimension = ref<ProfitDimension>('group')
const dimensions: ProfitDimension[] = ['group', 'account', 'platform', 'model']

const summaryLoading = ref(false)
const trendLoading = ref(false)
const breakdownLoading = ref(false)
const loading = computed(() => summaryLoading.value || trendLoading.value || breakdownLoading.value)

type SortKey = 'requests' | 'total_tokens' | 'revenue' | 'cost' | 'profit' | 'margin'
const sortKey = ref<SortKey>('revenue')
const sortDesc = ref(true)

const groupOptions = computed(() => [
  { value: '', label: t('admin.profit.allGroups') },
  ...groups.value.map((g) => ({ value: String(g.id), label: g.name }))
])
const platformOptions = computed(() => [
  { value: '', label: t('admin.profit.allPlatforms') },
  ...CONCRETE_PLATFORM_OPTIONS
])
const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
])

const metricColumns = computed<{ key: SortKey; label: string }[]>(() => [
  { key: 'requests', label: t('admin.profit.columns.requests') },
  { key: 'total_tokens', label: t('admin.profit.columns.tokens') },
  { key: 'revenue', label: t('admin.profit.columns.revenue') },
  { key: 'cost', label: t('admin.profit.columns.cost') },
  { key: 'profit', label: t('admin.profit.columns.profit') },
  { key: 'margin', label: t('admin.profit.columns.margin') }
])

const showRate = computed(() => dimension.value === 'group' || dimension.value === 'account')
const rateLabel = computed(() => {
  switch (dimension.value) {
    case 'group': return t('admin.profit.columns.groupRate')
    default: return t('admin.profit.columns.accountRate')
  }
})

function query(): ProfitQuery {
  return {
    start_date: startDate.value,
    end_date: endDate.value,
    group_id: groupFilter.value ? Number(groupFilter.value) : undefined,
    platform: platformFilter.value || undefined,
    include_subscription: includeSubscription.value || undefined
  }
}

async function loadSummary() {
  summaryLoading.value = true
  try {
    summary.value = await adminAPI.profit.getSummary(query())
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.profit.loadFailed')))
  } finally {
    summaryLoading.value = false
  }
}

async function loadTrend() {
  trendLoading.value = true
  try {
    trend.value = await adminAPI.profit.getTrend(query(), granularity.value)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.profit.loadFailed')))
  } finally {
    trendLoading.value = false
  }
}

async function loadBreakdown() {
  breakdownLoading.value = true
  try {
    breakdown.value = await adminAPI.profit.getBreakdown(query(), dimension.value)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.profit.loadFailed')))
  } finally {
    breakdownLoading.value = false
  }
}

function loadAll() {
  void loadSummary()
  void loadTrend()
  void loadBreakdown()
}

function switchDimension(dim: ProfitDimension) {
  if (dimension.value === dim) return
  dimension.value = dim
  void loadBreakdown()
}

function toggleSort(key: SortKey) {
  if (sortKey.value === key) {
    sortDesc.value = !sortDesc.value
  } else {
    sortKey.value = key
    sortDesc.value = true
  }
}

const sortedBreakdown = computed(() => {
  const rows = [...breakdown.value]
  const dir = sortDesc.value ? -1 : 1
  rows.sort((a, b) => (a[sortKey.value] - b[sortKey.value]) * dir)
  return rows
})

function rowName(row: ProfitBreakdownItem): string {
  if (dimension.value === 'platform') {
    return CONCRETE_PLATFORM_OPTIONS.find((option) => option.value === row.name)?.label ?? (row.name || '-')
  }
  if (row.name) return row.name
  if (dimension.value === 'group') return t('admin.profit.noGroup')
  return row.key || '-'
}

function money(v: number): string {
  return v.toFixed(Math.abs(v) >= 100 ? 2 : 4)
}
function percent(v: number): string {
  return `${(v * 100).toFixed(1)}%`
}
function trimNumber(v: number): string {
  return Number(v).toFixed(4).replace(/\.?0+$/, '')
}
function profitClass(v: number): string {
  if (v > 0) return 'text-emerald-600 dark:text-emerald-400'
  if (v < 0) return 'text-red-600 dark:text-red-400'
  return 'text-gray-600 dark:text-gray-300'
}

const kpis = computed(() => {
  const s = summary.value
  return [
    { key: 'revenue', label: t('admin.profit.kpi.revenue'), value: s ? `$${money(s.revenue)}` : '-', class: 'text-gray-900 dark:text-white', sub: s ? t('admin.profit.kpi.standardCost', { value: `$${money(s.standard_cost)}` }) : '' },
    { key: 'cost', label: t('admin.profit.kpi.cost'), value: s ? `$${money(s.cost)}` : '-', class: 'text-gray-900 dark:text-white', sub: '' },
    { key: 'profit', label: t('admin.profit.kpi.profit'), value: s ? `$${money(s.profit)}` : '-', class: s ? profitClass(s.profit) : '', sub: '' },
    { key: 'margin', label: t('admin.profit.kpi.margin'), value: s ? percent(s.margin) : '-', class: s ? profitClass(s.profit) : '', sub: s ? t('admin.profit.kpi.requests', { value: formatNumber(s.requests) }) : '' },
    {
      key: 'subscription',
      label: t('admin.profit.kpi.subscription'),
      value: s ? `$${money(s.subscription_consumption)}` : '-',
      class: 'text-gray-900 dark:text-white',
      sub: s ? t('admin.profit.kpi.subscriptionCost', { value: `$${money(s.subscription_cost)}` }) : ''
    }
  ]
})

const isDark = computed(() => document.documentElement.classList.contains('dark'))

const chartData = computed(() => {
  if (!trend.value.length) return null
  return {
    labels: trend.value.map((p) => p.date),
    datasets: [
      { label: t('admin.profit.kpi.revenue'), data: trend.value.map((p) => p.revenue), borderColor: '#3b82f6', backgroundColor: '#3b82f620', tension: 0.3, fill: false },
      { label: t('admin.profit.kpi.cost'), data: trend.value.map((p) => p.cost), borderColor: '#f59e0b', backgroundColor: '#f59e0b20', tension: 0.3, fill: false },
      { label: t('admin.profit.kpi.profit'), data: trend.value.map((p) => p.profit), borderColor: '#10b981', backgroundColor: '#10b98120', tension: 0.3, fill: true }
    ]
  }
})

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { intersect: false, mode: 'index' as const },
  plugins: {
    legend: { labels: { color: isDark.value ? '#e5e7eb' : '#374151' } },
    tooltip: {
      callbacks: {
        label: (ctx: TooltipItem<'line'>) => `${ctx.dataset.label}: $${money(ctx.parsed.y ?? 0)}`
      }
    }
  },
  scales: {
    x: { ticks: { color: isDark.value ? '#9ca3af' : '#6b7280' }, grid: { color: isDark.value ? '#374151' : '#e5e7eb' } },
    y: { ticks: { color: isDark.value ? '#9ca3af' : '#6b7280' }, grid: { color: isDark.value ? '#374151' : '#e5e7eb' } }
  }
}))

function exportCSV() {
  const header = [t(`admin.profit.dimensions.${dimension.value}`), 'requests', 'tokens', 'revenue', 'cost', 'profit', 'margin']
  const lines = [header.join(',')]
  for (const row of sortedBreakdown.value) {
    const name = `"${rowName(row).replace(/"/g, '""')}"`
    lines.push([name, row.requests, row.total_tokens, row.revenue.toFixed(6), row.cost.toFixed(6), row.profit.toFixed(6), row.margin.toFixed(4)].join(','))
  }
  const blob = new Blob(['﻿' + lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `profit-${dimension.value}-${startDate.value}_${endDate.value}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(async () => {
  loadAll()
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch {
    // 过滤项加载失败不影响主数据
  }
})
</script>
