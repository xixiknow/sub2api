<template>
  <div class="space-y-5 pb-12">
    <section
      class="rounded-3xl bg-white px-5 py-4 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:px-6"
    >
      <StatusBoardToolbar
        v-model:platform="platform"
        v-model:search="search"
        v-model:sort="sort"
        :range="window"
        :ranges="windowOptions"
        :counts="counts"
        :sort-keys="sortKeys"
        @update:range="emit('update:window', $event)"
      />
    </section>

    <StatusCardGrid
      :cards="visibleCards"
      :loading="loading && items.length === 0"
      clickable
      :empty-title="items.length ? t('channelStatusBoard.empty.noMatchTitle') : t('channelStatus.empty.title')"
      :empty-description="items.length ? t('channelStatusBoard.empty.noMatchDescription') : t('channelStatus.empty.description')"
      @card-click="onCardClick"
    >
      <template #extra="{ card }">
        <MonitorQuotaView
          v-if="quotaFor(card.key)"
          :snapshot="quotaFor(card.key)"
          class="mt-3"
        />
      </template>
    </StatusCardGrid>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView, UserMonitorDetail } from '@/api/channelMonitor'
import type { Group } from '@/types'
import MonitorQuotaView from '@/components/common/MonitorQuotaView.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { PROVIDER_ORDER } from '@/constants/channelMonitor'
import { isChannelMonitorQuotaVisible } from '@/utils/featureFlags'
import StatusBoardToolbar from './status-board/StatusBoardToolbar.vue'
import StatusCardGrid from './status-board/StatusCardGrid.vue'
import { useStatusBoard } from './status-board/useStatusBoard'
import { buildV1Cards } from './status-board/v1Mapping'
import type { BoardSortKey, StatusCardModel } from './status-board/types'

type MonitorWindow = '7d' | '15d' | '30d'

const props = defineProps<{
  items: UserMonitorView[]
  window: MonitorWindow
  countdownSeconds: number
  loading: boolean
  detailCache: Record<number, UserMonitorDetail>
  groupsById?: Record<number, Group>
  userGroupRates?: Record<number, number>
}>()

const emit = defineEmits<{
  (e: 'cardClick', item: UserMonitorView): void
  (e: 'update:window', value: MonitorWindow): void
}>()

const { t } = useI18n()
const { providerLabel, formatMonitorModel } = useChannelMonitorFormat()

// V1 没有缓存命中率数据，排序中不提供该项。
const sortKeys: BoardSortKey[] = ['default', 'rate', 'success', 'latency', 'status']
const windowOptions = computed(() =>
  (['7d', '15d', '30d'] as MonitorWindow[]).map((value) => ({ value, label: t(`channelStatus.windowTab.${value}`) })),
)

// 只解析出当前用户可用的分组；不可用或已删除的分组不展示。
function resolveGroups(item: UserMonitorView): Group[] {
  const byId = props.groupsById
  if (!byId) return []
  return (item.group_ids || []).map((id) => byId[id]).filter((g): g is Group => Boolean(g))
}

function resolveAvailability(item: UserMonitorView): number | null {
  if (props.window === '7d') {
    return item.availability_7d ?? null
  }
  const detail = props.detailCache[item.id]
  if (!detail) return null
  const primary = detail.models.find((m) => m.model === item.primary_model)
  if (!primary) return null
  return props.window === '15d' ? primary.availability_15d ?? null : primary.availability_30d ?? null
}

// 默认排序沿用原先的提供商分组顺序（PROVIDER_ORDER），未知 provider 排在末尾。
const orderedItems = computed(() => {
  const rank = (provider: string) => {
    const index = PROVIDER_ORDER.indexOf(provider as (typeof PROVIDER_ORDER)[number])
    return index === -1 ? PROVIDER_ORDER.length : index
  }
  return props.items
    .map((item, index) => ({ item, index }))
    .sort((a, b) => rank(a.item.provider) - rank(b.item.provider) || a.index - b.index)
    .map((entry) => entry.item)
})

const itemsById = computed(() => new Map(props.items.map((item) => [String(item.id), item])))

const cards = computed<StatusCardModel[]>(() =>
  buildV1Cards(orderedItems.value, {
    t: (key, params) => (params ? t(key, params) : t(key)),
    windowLabel: t(`channelStatus.windowTab.${props.window}`),
    availability: resolveAvailability,
    groups: resolveGroups,
    userGroupRates: props.userGroupRates || {},
    platformLabel: (provider) => providerLabel(provider),
    modelLabel: (model) => formatMonitorModel(model),
  }),
)
const { platform, search, sort, counts, visibleCards } = useStatusBoard(cards)

function quotaFor(key: string) {
  if (!isChannelMonitorQuotaVisible()) return null
  return itemsById.value.get(key)?.latest_quota ?? null
}

function onCardClick(card: StatusCardModel) {
  const item = itemsById.value.get(card.key)
  if (item) emit('cardClick', item)
}
</script>
