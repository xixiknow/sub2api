<template>
  <div
    ref="zoneRef"
    class="magnetic-zone"
    @pointerenter="emit('hover', true)"
    @pointermove="onMove"
    @pointerleave="onLeave"
  >
    <a
      :href="href"
      class="magnetic-btn"
      :class="[`magnetic-btn--${variant}`, size === 'lg' ? 'magnetic-btn--lg' : '']"
      :style="btnStyle"
      @click="onClick"
    >
      <span v-if="variant === 'primary'" class="magnetic-btn__ring" aria-hidden="true"></span>
      <span class="magnetic-btn__label" :style="labelStyle"><slot /></span>
    </a>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

/**
 * 磁吸按钮：鼠标靠近时按钮被「吸」向指针，主按钮带旋转流光边框。
 * 普通点击触发 launch（由父组件决定跳转方式，例如先播放跃迁动画）；
 * 带修饰键（新标签页打开等）时保留浏览器默认行为。
 */
withDefaults(
  defineProps<{
    href: string
    variant?: 'primary' | 'ghost'
    size?: 'md' | 'lg'
  }>(),
  { variant: 'primary', size: 'md' }
)

const emit = defineEmits<{
  (e: 'hover', value: boolean): void
  (e: 'launch'): void
}>()

const zoneRef = ref<HTMLElement | null>(null)
const offset = ref({ x: 0, y: 0 })

const btnStyle = computed(() => ({
  transform: `translate3d(${offset.value.x * 0.35}px, ${offset.value.y * 0.35}px, 0)`
}))
const labelStyle = computed(() => ({
  transform: `translate3d(${offset.value.x * 0.15}px, ${offset.value.y * 0.15}px, 0)`
}))

function onMove(e: PointerEvent) {
  if (e.pointerType !== 'mouse' || !zoneRef.value) return
  const rect = zoneRef.value.getBoundingClientRect()
  offset.value = {
    x: e.clientX - (rect.left + rect.width / 2),
    y: e.clientY - (rect.top + rect.height / 2)
  }
}

function onLeave() {
  offset.value = { x: 0, y: 0 }
  emit('hover', false)
}

function onClick(e: MouseEvent) {
  if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return
  e.preventDefault()
  emit('launch')
}

</script>

<style scoped>
.magnetic-zone {
  display: inline-flex;
  padding: 18px;
  margin: -18px;
}

.magnetic-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  padding: 0.85rem 1.9rem;
  font-weight: 600;
  font-size: 0.95rem;
  isolation: isolate;
  transition:
    transform 0.25s cubic-bezier(0.2, 0.8, 0.2, 1),
    box-shadow 0.3s ease,
    background-color 0.3s ease;
  will-change: transform;
}

.magnetic-btn--lg {
  padding: 1.05rem 2.4rem;
  font-size: 1.05rem;
}

.magnetic-btn__label {
  position: relative;
  z-index: 2;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  transition: transform 0.25s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.magnetic-btn--primary {
  color: #fff;
  background: linear-gradient(120deg, #f59e0b 0%, #d946ef 50%, #6366f1 100%);
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.12) inset,
    0 10px 40px -10px rgba(217, 70, 239, 0.7);
}

.magnetic-btn--primary:hover {
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.25) inset,
    0 0 60px -6px rgba(217, 70, 239, 0.9),
    0 0 120px -20px rgba(245, 158, 11, 0.8);
}

/* 旋转流光边框 */
.magnetic-btn__ring {
  position: absolute;
  inset: -2px;
  z-index: -1;
  border-radius: inherit;
  padding: 2px;
  background: conic-gradient(from var(--angle, 0deg), transparent 0 65%, #fde68a 80%, #fff 85%, transparent 92%);
  -webkit-mask:
    linear-gradient(#000 0 0) content-box,
    linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  animation: ring-spin 3s linear infinite;
}

/* 次按钮颜色取自页面主题变量（--sh-*），未设置时使用深色值 */
.magnetic-btn--ghost {
  color: var(--sh-ghost-fg, rgba(226, 232, 240, 0.95));
  background: var(--sh-ghost-bg, rgba(255, 255, 255, 0.04));
  border: 1px solid var(--sh-ghost-border, rgba(255, 255, 255, 0.14));
  backdrop-filter: blur(10px);
}

.magnetic-btn--ghost:hover {
  background: var(--sh-ghost-bg-hover, rgba(255, 255, 255, 0.09));
  border-color: var(--sh-ghost-border-hover, rgba(253, 230, 138, 0.45));
}

@property --angle {
  syntax: '<angle>';
  initial-value: 0deg;
  inherits: false;
}

@keyframes ring-spin {
  to {
    --angle: 360deg;
  }
}

@media (prefers-reduced-motion: reduce) {
  .magnetic-btn__ring {
    animation: none;
  }
}
</style>
