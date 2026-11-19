<template>
  <div>
    <div class="mb-2 flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
      <span>{{ label }}</span>
    </div>
    <div v-if="bars.length" class="flex h-5 w-full items-stretch gap-[3px]">
      <span
        v-for="(bar, index) in bars"
        :key="index"
        class="status-bar min-w-0 flex-1 rounded-[3px] outline-none"
        :class="[barClass(bar.status), activeIndex === index ? 'is-active' : '']"
        tabindex="0"
        role="img"
        :aria-label="bar.lines.join(' · ')"
        @mouseenter="show($event, index)"
        @mousemove="move($event)"
        @mouseleave="hide"
        @focus="show($event, index)"
        @blur="hide"
      ></span>
    </div>
    <div
      v-else
      class="flex h-5 w-full items-center justify-center rounded border border-dashed border-gray-200 text-[11px] text-gray-400 dark:border-dark-600"
    >
      {{ t('channelStatusBoard.noHistory') }}
    </div>
    <div class="mt-1.5 flex justify-between text-[11px] text-gray-400 dark:text-dark-400">
      <span>{{ t('channelStatusBoard.past') }}</span>
      <span>{{ t('channelStatusBoard.now') }}</span>
    </div>

    <Teleport to="body">
      <div
        v-if="tooltip.visible && activeBar"
        class="status-board-tooltip"
        :style="{ left: `${tooltip.x}px`, top: `${tooltip.y}px` }"
        role="tooltip"
      >
        <span
          v-for="(line, index) in activeBar.lines"
          :key="`${index}:${line}`"
          class="block whitespace-nowrap"
          :class="index === 0 ? 'mb-1 font-semibold text-white' : 'text-gray-200'"
        >
          {{ line }}
        </span>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { BoardBar, BoardStatus } from './types'

const props = defineProps<{
  bars: BoardBar[]
  label: string
}>()

const { t } = useI18n()

const activeIndex = ref<number | null>(null)
const activeBar = computed(() => (activeIndex.value == null ? null : props.bars[activeIndex.value] ?? null))
const tooltip = reactive({ visible: false, x: 0, y: 0 })

function barClass(status: BoardStatus): string {
  switch (status) {
    case 'operational':
      return 'bg-emerald-500/85 dark:bg-emerald-500/75'
    case 'degraded':
      return 'bg-amber-400 dark:bg-amber-400/80'
    case 'failed':
      return 'bg-red-400 dark:bg-red-500/80'
    default:
      return 'bg-gray-200 dark:bg-dark-600'
  }
}

function show(event: MouseEvent | FocusEvent, index: number) {
  activeIndex.value = index
  tooltip.visible = true
  position(event)
}

function move(event: MouseEvent) {
  if (tooltip.visible) position(event)
}

function hide() {
  tooltip.visible = false
  activeIndex.value = null
}

function position(event: MouseEvent | FocusEvent) {
  const target = event.target as HTMLElement | null
  const rect = target?.getBoundingClientRect()
  if (!rect) return
  // Anchor above the hovered bar, clamped to the viewport.
  tooltip.x = Math.min(window.innerWidth - 12, Math.max(12, rect.left + rect.width / 2))
  tooltip.y = Math.max(12, rect.top - 10)
}
</script>

<style scoped>
.status-bar {
  cursor: help;
  transition: transform 0.12s ease;
}
.status-bar.is-active,
.status-bar:focus-visible {
  box-shadow:
    0 0 0 2px rgb(255 255 255),
    0 0 0 4px rgb(var(--color-primary-500, 99 102 241) / 0.85);
  transform: scaleY(1.12);
  z-index: 1;
}
:global(.dark) .status-bar.is-active,
:global(.dark) .status-bar:focus-visible {
  box-shadow:
    0 0 0 2px rgb(31 41 55),
    0 0 0 4px rgb(var(--color-primary-400, 129 140 248) / 0.85);
}
.status-board-tooltip {
  pointer-events: none;
  position: fixed;
  z-index: 9999;
  transform: translate(-50%, -100%);
  max-width: min(20rem, calc(100vw - 1.5rem));
  border-radius: 0.75rem;
  background: rgb(31 33 45 / 0.96);
  padding: 0.625rem 0.75rem;
  font-size: 12px;
  line-height: 1.55;
  box-shadow: 0 18px 40px -12px rgb(0 0 0 / 0.45);
}
</style>
