<template>
  <Transition name="unlock">
    <div
      v-if="achievement"
      class="fixed inset-0 z-[80] flex items-center justify-center overflow-hidden bg-slate-950/[0.74] px-4 backdrop-blur-md"
      :style="overlayStyle"
      role="dialog"
      aria-modal="true"
      @click.self="$emit('close')"
    >
      <div class="pointer-events-none absolute inset-0">
        <div class="unlock-rays"></div>
        <div class="unlock-flash"></div>
        <span
          v-for="piece in confettiPieces"
          :key="piece.id"
          class="unlock-confetti absolute"
          :style="piece.style"
        ></span>
        <span
          v-for="particle in particles"
          :key="particle.id"
          class="unlock-particle absolute rounded-full"
          :style="particle.style"
        ></span>
      </div>

      <div class="unlock-stage relative flex w-full max-w-md flex-col items-center text-center">
        <div class="unlock-ring unlock-ring-one"></div>
        <div class="unlock-ring unlock-ring-two"></div>
        <div class="unlock-burst"></div>

        <p class="relative z-10 text-xs font-semibold tracking-[0.28em] text-amber-100">
          成就已解锁
        </p>
        <div class="unlock-badge-wrap relative z-10 mt-5">
          <AchievementBadge
            :icon="achievement.icon"
            :motif="achievement.motif"
            :tone="achievement.tone"
            :tier="achievement.tier"
            :title="achievement.name"
            size="xl"
            unlocked
          />
        </div>
        <h2 class="relative z-10 mt-5 text-3xl font-bold text-white">
          {{ achievement.name }}
        </h2>
        <p class="relative z-10 mt-2 max-w-sm text-sm leading-6 text-slate-200">
          {{ achievement.title }}
        </p>
        <div class="relative z-10 mt-5 inline-flex items-center gap-2 rounded-full border border-amber-200/50 bg-amber-300/[0.16] px-4 py-2 text-sm font-semibold text-amber-50 shadow-[0_0_28px_rgba(245,158,11,0.32)]">
          <Icon name="gift" size="sm" />
          <span>{{ achievement.reward }}</span>
        </div>
        <button
          type="button"
          class="relative z-10 mt-7 inline-flex h-10 items-center justify-center rounded-lg bg-white px-4 text-sm font-semibold text-slate-950 transition hover:bg-amber-50 focus:outline-none focus:ring-2 focus:ring-amber-200"
          @click="$emit('close')"
        >
          收下徽章
        </button>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import AchievementBadge from './AchievementBadge.vue'
import type { ActivationAchievement, AchievementTone } from './activationAchievements'

defineEmits<{
  close: []
}>()

const props = defineProps<{
  achievement: ActivationAchievement | null
}>()

const toneOverlayMap: Record<AchievementTone, Record<string, string>> = {
  aqua: {
    '--unlock-primary': '#22d3ee',
    '--unlock-soft': 'rgba(34, 211, 238, 0.28)',
    '--unlock-hot': '#a7f3d0',
  },
  emerald: {
    '--unlock-primary': '#34d399',
    '--unlock-soft': 'rgba(52, 211, 153, 0.26)',
    '--unlock-hot': '#fef08a',
  },
  gold: {
    '--unlock-primary': '#f59e0b',
    '--unlock-soft': 'rgba(245, 158, 11, 0.3)',
    '--unlock-hot': '#67e8f9',
  },
  indigo: {
    '--unlock-primary': '#818cf8',
    '--unlock-soft': 'rgba(129, 140, 248, 0.28)',
    '--unlock-hot': '#fbbf24',
  },
  rose: {
    '--unlock-primary': '#fb7185',
    '--unlock-soft': 'rgba(251, 113, 133, 0.28)',
    '--unlock-hot': '#fde68a',
  },
  violet: {
    '--unlock-primary': '#a78bfa',
    '--unlock-soft': 'rgba(167, 139, 250, 0.3)',
    '--unlock-hot': '#22d3ee',
  },
}

const toneHues: Record<AchievementTone, number[]> = {
  aqua: [186, 168, 42, 212],
  emerald: [152, 118, 42, 186],
  gold: [42, 36, 186, 340],
  indigo: [236, 262, 42, 196],
  rose: [350, 330, 42, 186],
  violet: [262, 286, 196, 42],
}

const activeTone = computed<AchievementTone>(() => props.achievement?.tone ?? 'gold')
const overlayStyle = computed(() => toneOverlayMap[activeTone.value])

const particles = computed(() => Array.from({ length: 52 }, (_, index) => {
  const total = 52
  const angle = (index / total) * Math.PI * 2
  const distance = 108 + (index % 9) * 28
  const size = 4 + (index % 5)
  const hue = toneHues[activeTone.value][index % toneHues[activeTone.value].length]
  return {
    id: `particle-${index}`,
    style: {
      '--tx': `${Math.cos(angle) * distance}px`,
      '--ty': `${Math.sin(angle) * distance}px`,
      '--delay': `${(index % 9) * 0.045}s`,
      left: '50%',
      top: '50%',
      width: `${size}px`,
      height: `${size}px`,
      background: `hsl(${hue} 92% 72%)`,
      boxShadow: `0 0 ${size * 3}px hsl(${hue} 92% 66%)`,
      borderRadius: index % 4 === 0 ? '2px' : '9999px',
    },
  }
}))

const confettiPieces = computed(() => Array.from({ length: 24 }, (_, index) => {
  const hue = toneHues[activeTone.value][(index + 2) % toneHues[activeTone.value].length]
  return {
    id: `confetti-${index}`,
    style: {
      '--fall-x': `${((index % 7) - 3) * 24}px`,
      '--fall-y': `${120 + (index % 5) * 34}px`,
      '--delay': `${(index % 8) * 0.06}s`,
      '--spin': `${index % 2 === 0 ? 360 : -360}deg`,
      left: `${8 + ((index * 19) % 84)}%`,
      top: `${8 + ((index * 29) % 44)}%`,
      width: `${5 + (index % 4) * 2}px`,
      height: `${12 + (index % 3) * 4}px`,
      background: `linear-gradient(180deg, hsl(${hue} 96% 78%), hsl(${hue} 92% 58%))`,
    },
  }
}))
</script>

<style scoped>
.unlock-stage {
  min-height: 460px;
}

.unlock-rays {
  position: absolute;
  left: 50%;
  top: 44%;
  width: 740px;
  height: 740px;
  transform: translate(-50%, -50%);
  background:
    repeating-conic-gradient(from 8deg, transparent 0deg 8deg, var(--unlock-soft) 8deg 10deg),
    radial-gradient(circle at 50% 50%, var(--unlock-soft), transparent 48%);
  filter: blur(0.4px);
  opacity: 0;
  animation: unlockRays 1.5s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.unlock-flash {
  position: absolute;
  left: 50%;
  top: 44%;
  width: 280px;
  height: 280px;
  transform: translate(-50%, -50%);
  border-radius: 9999px;
  background:
    radial-gradient(circle, rgba(255, 255, 255, 0.82), var(--unlock-primary) 24%, transparent 66%);
  mix-blend-mode: screen;
  opacity: 0;
  animation: unlockFlash 0.62s ease-out both;
}

.unlock-ring {
  position: absolute;
  left: 50%;
  top: 44%;
  border-radius: 9999px;
  transform: translate(-50%, -50%);
  border: 1px solid rgba(255, 255, 255, 0.34);
  box-shadow:
    0 0 38px var(--unlock-soft),
    inset 0 0 34px rgba(255, 255, 255, 0.13);
}

.unlock-ring-one {
  width: 284px;
  height: 284px;
  animation: unlockSpin 5s linear infinite;
  border-top-color: var(--unlock-primary);
  border-right-color: var(--unlock-hot);
}

.unlock-ring-two {
  width: 220px;
  height: 220px;
  animation: unlockSpin 3.6s linear infinite reverse;
  border-bottom-color: rgba(244, 114, 182, 0.72);
  border-left-color: var(--unlock-primary);
}

.unlock-burst {
  position: absolute;
  left: 50%;
  top: 44%;
  width: 360px;
  height: 360px;
  transform: translate(-50%, -50%);
  border-radius: 9999px;
  background:
    radial-gradient(circle at 50% 50%, var(--unlock-soft), transparent 27%),
    conic-gradient(from 20deg, transparent, var(--unlock-soft), transparent, rgba(34, 211, 238, 0.18), transparent);
  filter: blur(2px);
  opacity: 0.9;
  animation: unlockBurst 1.4s ease-out both;
}

.unlock-badge-wrap {
  filter: drop-shadow(0 0 32px var(--unlock-primary));
  animation: unlockBadgeFloat 1.6s ease-out both;
}

.unlock-particle {
  animation: particleFly 1.35s cubic-bezier(0.19, 1, 0.22, 1) both;
  animation-delay: var(--delay);
}

.unlock-confetti {
  border-radius: 2px;
  box-shadow: 0 0 14px var(--unlock-soft);
  opacity: 0;
  transform-origin: center;
  animation: confettiFall 1.8s cubic-bezier(0.16, 1, 0.3, 1) both;
  animation-delay: var(--delay);
}

.unlock-enter-active,
.unlock-leave-active {
  transition: opacity 0.28s ease;
}

.unlock-enter-from,
.unlock-leave-to {
  opacity: 0;
}

.unlock-enter-active .unlock-stage {
  animation: stagePop 0.68s cubic-bezier(0.16, 1, 0.3, 1) both;
}

@keyframes stagePop {
  0% {
    opacity: 0;
    transform: translateY(18px) scale(0.86) rotate(-2deg);
  }
  68% {
    opacity: 1;
    transform: translateY(-4px) scale(1.035) rotate(1deg);
  }
  100% {
    opacity: 1;
    transform: translateY(0) scale(1) rotate(0deg);
  }
}

@keyframes unlockRays {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.3) rotate(-18deg);
  }
  42% {
    opacity: 0.72;
  }
  100% {
    opacity: 0.24;
    transform: translate(-50%, -50%) scale(1) rotate(24deg);
  }
}

@keyframes unlockFlash {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.12);
  }
  34% {
    opacity: 0.88;
  }
  100% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(2.2);
  }
}

@keyframes unlockSpin {
  to {
    transform: translate(-50%, -50%) rotate(360deg);
  }
}

@keyframes unlockBurst {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.42) rotate(0deg);
  }
  55% {
    opacity: 1;
  }
  100% {
    opacity: 0.62;
    transform: translate(-50%, -50%) scale(1) rotate(28deg);
  }
}

@keyframes unlockBadgeFloat {
  0% {
    opacity: 0;
    transform: translateY(20px) scale(0.42) rotate(-10deg);
  }
  52% {
    opacity: 1;
    transform: translateY(-10px) scale(1.1) rotate(4deg);
  }
  100% {
    opacity: 1;
    transform: translateY(0) scale(1) rotate(0deg);
  }
}

@keyframes particleFly {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.2);
  }
  18% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translate(calc(-50% + var(--tx)), calc(-50% + var(--ty))) scale(1);
  }
}

@keyframes confettiFall {
  0% {
    opacity: 0;
    transform: translateY(-18px) rotate(0deg) scale(0.7);
  }
  18% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translate(var(--fall-x), var(--fall-y)) rotate(var(--spin)) scale(1);
  }
}

@media (prefers-reduced-motion: reduce) {
  .unlock-rays,
  .unlock-flash,
  .unlock-ring,
  .unlock-burst,
  .unlock-badge-wrap,
  .unlock-particle,
  .unlock-confetti,
  .unlock-enter-active .unlock-stage {
    animation: none;
  }
}
</style>
