<template>
  <div>
    <div v-if="loading && cards.length === 0" class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      <div
        v-for="i in 8"
        :key="i"
        class="h-[262px] animate-pulse rounded-2xl border border-gray-200/70 bg-white p-5 dark:border-dark-700/70 dark:bg-dark-800"
      >
        <div class="flex items-start gap-3">
          <div class="h-11 w-11 rounded-xl bg-gray-100 dark:bg-dark-700"></div>
          <div class="flex-1 space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-100 dark:bg-dark-700"></div>
            <div class="h-3 w-1/2 rounded bg-gray-100 dark:bg-dark-700"></div>
          </div>
        </div>
        <div class="mt-6 grid grid-cols-3 gap-3">
          <div v-for="j in 3" :key="j" class="h-14 rounded-lg bg-gray-50 dark:bg-dark-900/40"></div>
        </div>
        <div class="mt-8 h-5 rounded bg-gray-50 dark:bg-dark-900/40"></div>
      </div>
    </div>

    <EmptyState
      v-else-if="cards.length === 0"
      :title="emptyTitle || t('channelStatusBoard.empty.title')"
      :description="emptyDescription || t('channelStatusBoard.empty.description')"
    />

    <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      <StatusCard
        v-for="card in cards"
        :key="card.key"
        :card="card"
        :clickable="clickable"
        @click="emit('cardClick', card)"
      >
        <slot name="extra" :card="card" />
      </StatusCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import EmptyState from '@/components/common/EmptyState.vue'
import StatusCard from './StatusCard.vue'
import type { StatusCardModel } from './types'

withDefaults(
  defineProps<{
    cards: StatusCardModel[]
    loading?: boolean
    clickable?: boolean
    emptyTitle?: string
    emptyDescription?: string
  }>(),
  { loading: false, clickable: false, emptyTitle: '', emptyDescription: '' },
)

const emit = defineEmits<{
  (e: 'cardClick', card: StatusCardModel): void
}>()

const { t } = useI18n()
</script>
