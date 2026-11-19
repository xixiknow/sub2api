<template>
  <span
    v-if="visible"
    :class="['inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium', toneClass]"
    :title="tooltip"
  >
    <span :class="['h-1.5 w-1.5 rounded-full', dotClass]" />
    {{ t('admin.accounts.health.title') }} {{ scorePercent }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountHealthSnapshot } from '@/api/admin/accounts'

const props = defineProps<{ health?: AccountHealthSnapshot | null }>()
const { t } = useI18n()

const visible = computed(() => {
  const h = props.health
  return !!h && (h.samples > 0 || h.penalty_factor < 1)
})

const scorePercent = computed(() => `${Math.round((props.health?.score ?? 1) * 100)}`)

const level = computed(() => {
  const s = props.health?.score ?? 1
  if (s >= 0.8) return 'good'
  if (s >= 0.5) return 'warn'
  return 'bad'
})

const toneClass = computed(() => ({
  good: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
  warn: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
  bad: 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300',
}[level.value]))

const dotClass = computed(() => ({
  good: 'bg-emerald-500',
  warn: 'bg-amber-500',
  bad: 'bg-red-500',
}[level.value]))

const tooltip = computed(() => {
  const h = props.health
  if (!h) return ''
  const lines = [
    t('admin.accounts.health.hint'),
    `${t('admin.accounts.health.errorRate')}: ${(h.error_rate * 100).toFixed(1)}%`,
    `${t('admin.accounts.health.ttft')}: ${h.ttft_ms != null ? `${Math.round(h.ttft_ms)} ms` : '-'}`,
    `${t('admin.accounts.health.samples')}: ${h.samples}`,
  ]
  if (h.penalty_factor < 1) {
    lines.push(`${t('admin.accounts.health.penalty')}: ×${h.penalty_factor} ${h.penalty_reason ?? ''}`)
  }
  return lines.join('\n')
})
</script>
