<template>
  <div
    ref="cardRef"
    class="spotlight-card group"
    :style="style"
    @pointermove="onMove"
    @pointerleave="onLeave"
  >
    <div class="relative z-[1] h-full">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

/** 聚光灯卡片：光斑与边框流光跟随鼠标，卡片随指针 3D 微倾。 */
const cardRef = ref<HTMLElement | null>(null)
const pos = ref({ x: 50, y: 50, rx: 0, ry: 0 })

const style = computed(() => ({
  '--mx': `${pos.value.x}%`,
  '--my': `${pos.value.y}%`,
  transform: `perspective(900px) rotateX(${pos.value.rx}deg) rotateY(${pos.value.ry}deg)`
}))

function onMove(e: PointerEvent) {
  if (e.pointerType !== 'mouse' || !cardRef.value) return
  const rect = cardRef.value.getBoundingClientRect()
  const x = (e.clientX - rect.left) / rect.width
  const y = (e.clientY - rect.top) / rect.height
  pos.value = { x: x * 100, y: y * 100, rx: (0.5 - y) * 8, ry: (x - 0.5) * 8 }
}

function onLeave() {
  pos.value = { ...pos.value, rx: 0, ry: 0 }
}
</script>

<style scoped>
.spotlight-card {
  position: relative;
  border-radius: 1.5rem;
  padding: 1.75rem;
  /* 颜色取自页面主题变量（--sh-*），未设置时使用深色值 */
  background: var(--sh-card-bg, linear-gradient(160deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0.015)));
  border: 1px solid var(--sh-card-border, rgba(255, 255, 255, 0.08));
  box-shadow: var(--sh-card-shadow, none);
  backdrop-filter: blur(14px);
  transition:
    transform 0.25s cubic-bezier(0.2, 0.8, 0.2, 1),
    border-color 0.3s ease;
  transform-style: preserve-3d;
  will-change: transform;
}

.spotlight-card::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: radial-gradient(420px circle at var(--mx) var(--my), var(--sh-spot, rgba(167, 139, 250, 0.16)), transparent 45%);
  opacity: 0;
  transition: opacity 0.3s ease;
  pointer-events: none;
}

.spotlight-card::after {
  content: '';
  position: absolute;
  inset: -1px;
  border-radius: inherit;
  padding: 1px;
  background: radial-gradient(260px circle at var(--mx) var(--my), rgba(253, 230, 138, 0.85), rgba(217, 70, 239, 0.4) 30%, transparent 55%);
  -webkit-mask:
    linear-gradient(#000 0 0) content-box,
    linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  opacity: 0;
  transition: opacity 0.3s ease;
  pointer-events: none;
}

.spotlight-card:hover::before,
.spotlight-card:hover::after {
  opacity: 1;
}
</style>
