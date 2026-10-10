<template>
  <div>
    <div
      v-if="loading && items.length === 0"
      class="grid gap-5 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
    >
      <div
        v-for="i in 6"
        :key="i"
        class="p-5 rounded-2xl min-h-[280px] bg-white/70 dark:bg-dark-800/60 border border-gray-200/80 dark:border-dark-700/70 animate-pulse"
      >
        <div class="flex items-start gap-3">
          <div class="w-9 h-9 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
          <div class="flex-1 space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
            <div class="h-3 w-1/2 rounded bg-gray-200 dark:bg-dark-700"></div>
          </div>
          <div class="h-6 w-16 rounded-full bg-gray-200 dark:bg-dark-700"></div>
        </div>
        <div class="mt-5 grid grid-cols-2 gap-2">
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
        </div>
        <div class="mt-6 h-5 w-full rounded bg-gray-100 dark:bg-dark-900/40"></div>
      </div>
    </div>

    <EmptyState
      v-else-if="items.length === 0"
      :title="t('channelStatus.empty.title')"
      :description="t('channelStatus.empty.description')"
    />

    <div v-else class="space-y-7">
      <section v-for="group in groupedItems" :key="group.provider">
        <div class="mb-3 flex items-center gap-2">
          <span
            class="inline-flex h-7 w-7 items-center justify-center rounded-lg ring-1 ring-black/5 dark:ring-white/10"
            :class="providerGradient(group.provider)"
          >
            <ProviderIcon :provider="group.provider" :size="16" />
          </span>
          <h3 class="text-sm font-bold text-gray-900 dark:text-white">
            {{ providerLabel(group.provider) }}
          </h3>
          <span class="text-xs text-gray-400 dark:text-dark-500">{{ group.items.length }}</span>
        </div>
        <div class="grid gap-5 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          <MonitorCard
            v-for="item in group.items"
            :key="item.id"
            :item="item"
            :window="window"
            :availability-value="resolveAvailability(item)"
            :countdown-seconds="countdownSeconds"
            @click="emit('cardClick', item)"
          />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView, UserMonitorDetail } from '@/api/channelMonitor'
import EmptyState from '@/components/common/EmptyState.vue'
import MonitorCard from './MonitorCard.vue'
import ProviderIcon from './ProviderIcon.vue'
import { providerGradient, useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { PROVIDER_ORDER } from '@/constants/channelMonitor'

const props = defineProps<{
  items: UserMonitorView[]
  window: '7d' | '15d' | '30d'
  countdownSeconds: number
  loading: boolean
  detailCache: Record<number, UserMonitorDetail>
}>()

const emit = defineEmits<{
  (e: 'cardClick', item: UserMonitorView): void
}>()

const { t } = useI18n()
const { providerLabel } = useChannelMonitorFormat()

// 按提供商分组展示（顺序由 PROVIDER_ORDER 决定），组内保持原有卡片网格。
// 未知 provider 值兜底为末尾分组，避免卡片丢失。
const groupedItems = computed(() => {
  const byProvider = new Map<string, UserMonitorView[]>()
  for (const item of props.items) {
    const list = byProvider.get(item.provider)
    if (list) list.push(item)
    else byProvider.set(item.provider, [item])
  }
  const ordered: Array<{ provider: string; items: UserMonitorView[] }> = []
  for (const provider of PROVIDER_ORDER) {
    const group = byProvider.get(provider)
    if (group && group.length > 0) {
      ordered.push({ provider, items: group })
      byProvider.delete(provider)
    }
  }
  for (const [provider, group] of byProvider) {
    ordered.push({ provider, items: group })
  }
  return ordered
})

function resolveAvailability(item: UserMonitorView): number | null {
  if (props.window === '7d') {
    return item.availability_7d ?? null
  }
  const detail = props.detailCache[item.id]
  if (!detail) return null
  const primary = detail.models.find(m => m.model === item.primary_model)
  if (!primary) return null
  return props.window === '15d' ? primary.availability_15d ?? null : primary.availability_30d ?? null
}
</script>
