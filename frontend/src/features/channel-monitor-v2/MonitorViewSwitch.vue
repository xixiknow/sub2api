<template>
  <div class="inline-flex rounded-xl bg-gray-100/80 p-0.5 dark:bg-dark-900/60" role="group" :aria-label="t('channelStatusBoard.view.aria')">
    <button
      v-for="option in options"
      :key="option"
      type="button"
      class="rounded-lg px-2.5 py-1 text-xs transition-colors"
      :class="modelValue === option
        ? 'bg-white font-semibold text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
        : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
      @click="emit('update:modelValue', option)"
    >
      {{ t(`channelStatusBoard.view.${option}`) }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { BoardViewMode as MonitorViewMode } from '@/components/user/monitor/status-board/types'

defineProps<{ modelValue: MonitorViewMode }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: MonitorViewMode): void }>()

const { t } = useI18n()
const options: MonitorViewMode[] = ['cards', 'detail']
</script>
