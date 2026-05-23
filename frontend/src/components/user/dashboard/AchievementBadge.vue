<template>
  <div
    class="achievement-badge relative inline-flex shrink-0 items-center justify-center"
    :class="[sizeClass, unlocked ? 'is-unlocked' : 'is-locked', `tier-${tier}`]"
    :style="toneStyle"
    :title="title"
  >
    <svg class="absolute inset-0 h-full w-full" viewBox="0 0 120 120" aria-hidden="true">
      <defs>
        <radialGradient :id="glowId" cx="50%" cy="34%" r="64%">
          <stop offset="0%" stop-color="var(--badge-glow)" stop-opacity="0.96" />
          <stop offset="54%" stop-color="var(--badge-base)" stop-opacity="0.36" />
          <stop offset="100%" stop-color="var(--badge-edge)" stop-opacity="0.08" />
        </radialGradient>
        <linearGradient :id="metalId" x1="16%" y1="10%" x2="84%" y2="92%">
          <stop offset="0%" stop-color="var(--badge-shine)" />
          <stop offset="46%" stop-color="var(--badge-base)" />
          <stop offset="100%" stop-color="var(--badge-edge)" />
        </linearGradient>
        <filter :id="shadowId" x="-30%" y="-30%" width="160%" height="160%">
          <feDropShadow dx="0" dy="10" stdDeviation="8" flood-color="var(--badge-shadow)" flood-opacity="0.28" />
        </filter>
      </defs>

      <circle
        cx="60"
        cy="60"
        r="48"
        :fill="`url(#${glowId})`"
        class="badge-aura"
      />
      <circle
        cx="60"
        cy="60"
        r="55"
        fill="none"
        class="badge-tier-ring"
      />
      <path
        d="M60 8 76.4 18.8 95.8 18.8 101.4 37.6 114 52.5 104.4 69.5 104.4 89.2 85.6 94.8 72.1 109.2 60 101.5 47.9 109.2 34.4 94.8 15.6 89.2 15.6 69.5 6 52.5 18.6 37.6 24.2 18.8 43.6 18.8Z"
        :fill="`url(#${metalId})`"
        :filter="`url(#${shadowId})`"
      />
      <path
        d="M60 11 68 27 86 24 81 42 98 49 81.5 58 91 74 72 72.5 66 91 60 75 54 91 48 72.5 29 74 38.5 58 22 49 39 42 34 24 52 27Z"
        fill="rgba(255,255,255,0.13)"
        class="badge-facet"
      />
      <path
        d="M60 17 73.5 26.1 89.7 25.9 94.1 41.5 104.4 54 96.2 68 96.5 84.3 80.9 88.8 69.9 100.8 60 94.1 50.1 100.8 39.1 88.8 23.5 84.3 23.8 68 15.6 54 25.9 41.5 30.3 25.9 46.5 26.1Z"
        fill="rgba(255,255,255,0.15)"
        stroke="rgba(255,255,255,0.54)"
        stroke-width="1.8"
      />
      <g
        class="badge-motif"
        fill="none"
        stroke="var(--badge-motif)"
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2.7"
      >
        <g v-if="motif === 'starburst'">
          <path d="M60 27v15M60 78v15M27 60h15M78 60h15" />
          <path d="M36.6 36.6 47 47M73 73l10.4 10.4M83.4 36.6 73 47M47 73 36.6 83.4" />
          <circle cx="60" cy="60" r="19" />
        </g>
        <g v-else-if="motif === 'keyhole'">
          <path d="M42 52h24a12 12 0 1 1 0 16H54l-6 6h-9l6-6h-7" />
          <circle cx="77" cy="60" r="6" />
        </g>
        <g v-else-if="motif === 'terminalGrid'">
          <path d="M35 43h50v34H35z" />
          <path d="m43 55 8 6-8 6M57 68h17" />
          <path d="M43 48h34M43 77h34" opacity="0.58" />
        </g>
        <g v-else-if="motif === 'compass'">
          <path d="M60 28 74 60 60 92 46 60Z" />
          <path d="M60 28 60 92M32 60h56" opacity="0.62" />
          <circle cx="60" cy="60" r="9" />
        </g>
        <g v-else-if="motif === 'pulse'">
          <path d="M29 64h14l8-18 11 32 8-20h21" />
          <path d="M33 48c10-11 26-15 41-7M45 82c11 7 24 8 37 0" opacity="0.54" />
        </g>
        <g v-else-if="motif === 'bars'">
          <path d="M38 79V62M53 79V49M68 79V39M83 79V29" />
          <path d="M35 82h53M38 56l15-11 15 5 15-18" opacity="0.68" />
        </g>
        <g v-else-if="motif === 'engine'">
          <circle cx="60" cy="60" r="23" />
          <path d="M60 29v11M60 80v11M29 60h11M80 60h11M38.1 38.1l7.8 7.8M74.1 74.1l7.8 7.8M81.9 38.1l-7.8 7.8M45.9 74.1l-7.8 7.8" />
          <path d="m60 47 5 9 10 2-7 7 2 10-10-5-10 5 2-10-7-7 10-2Z" />
        </g>
        <g v-else-if="motif === 'singleNode'">
          <circle cx="45" cy="63" r="9" />
          <circle cx="78" cy="48" r="8" />
          <path d="M53 59 70 52M42 73v10M81 58v11" />
        </g>
        <g v-else-if="motif === 'network'">
          <circle cx="38" cy="45" r="7" />
          <circle cx="76" cy="39" r="7" />
          <circle cx="52" cy="80" r="7" />
          <circle cx="87" cy="75" r="7" />
          <path d="M45 44 69 40M42 52l8 21M58 77l22-2M73 45 55 73M80 45l6 23" />
        </g>
        <g v-else-if="motif === 'rocket'">
          <path d="M42 78c16-31 34-45 55-52-7 21-21 39-52 55Z" />
          <path d="M63 57 79 73M42 78l-10 10 3-15M45 81l-15 3 10-10" />
          <circle cx="78" cy="45" r="5" />
        </g>
        <g v-else-if="motif === 'crown'">
          <path d="M34 78 40 42l16 19 12-28 13 28 16-19 6 36Z" />
          <path d="M38 84h44M47 72h26" opacity="0.62" />
        </g>
        <g v-else-if="motif === 'coin'">
          <circle cx="60" cy="60" r="25" />
          <path d="M60 43v34M49 53c4-5 16-5 20 0M51 67c4 5 14 5 18 0" />
          <path d="M34 51c4-12 15-21 28-22M86 69c-4 12-15 21-28 22" opacity="0.58" />
        </g>
        <g v-else-if="motif === 'shield'">
          <path d="M60 29c9 8 19 11 31 11v18c0 17-11 30-31 36-20-6-31-19-31-36V40c12 0 22-3 31-11Z" />
          <path d="m48 61 8 8 17-19" />
        </g>
      </g>
      <circle
        cx="60"
        cy="60"
        r="27"
        fill="rgba(255,255,255,0.18)"
        stroke="rgba(255,255,255,0.52)"
        stroke-width="1.5"
      />
      <path
        d="M36 38c9.5-9.2 25.6-14.7 41.6-6.2"
        fill="none"
        stroke="rgba(255,255,255,0.65)"
        stroke-linecap="round"
        stroke-width="3"
      />
      <g v-if="unlocked" class="badge-sparks">
        <circle cx="26" cy="52" r="2.4" fill="var(--badge-spark)" />
        <circle cx="91" cy="37" r="2.2" fill="var(--badge-spark)" />
        <circle cx="91" cy="78" r="1.9" fill="var(--badge-spark)" />
        <path d="M33 79l3-7 3 7 7 3-7 3-3 7-3-7-7-3Z" fill="var(--badge-spark)" opacity="0.9" />
      </g>
    </svg>

    <div class="relative z-10 flex items-center justify-center rounded-full bg-white/[0.18] text-white shadow-inner ring-1 ring-white/30 backdrop-blur-sm" :class="iconWrapClass">
      <Icon :name="icon" :size="iconSize" :stroke-width="2" />
    </div>

    <div
      v-if="!unlocked"
      class="absolute inset-0 z-20 flex items-center justify-center rounded-full bg-slate-950/[0.42] text-white backdrop-grayscale"
      aria-hidden="true"
    >
      <Icon name="lock" :size="lockSize" :stroke-width="2" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import type { AchievementIcon, AchievementMotif, AchievementTier, AchievementTone } from './activationAchievements'

const props = withDefaults(defineProps<{
  icon: AchievementIcon
  motif?: AchievementMotif
  tone: AchievementTone
  tier?: AchievementTier
  unlocked?: boolean
  title?: string
  size?: 'sm' | 'md' | 'lg' | 'xl'
}>(), {
  motif: 'starburst',
  tier: 'bronze',
  unlocked: true,
  title: '',
  size: 'md',
})

const uniqueId = `achievement-${Math.random().toString(36).slice(2)}`
const glowId = `${uniqueId}-glow`
const metalId = `${uniqueId}-metal`
const shadowId = `${uniqueId}-shadow`

const toneMap: Record<AchievementTone, Record<string, string>> = {
  aqua: {
    '--badge-base': '#0891b2',
    '--badge-edge': '#0f766e',
    '--badge-glow': '#67e8f9',
    '--badge-shine': '#ecfeff',
    '--badge-spark': '#cffafe',
    '--badge-shadow': '#155e75',
  },
  emerald: {
    '--badge-base': '#059669',
    '--badge-edge': '#166534',
    '--badge-glow': '#86efac',
    '--badge-shine': '#ecfdf5',
    '--badge-spark': '#bbf7d0',
    '--badge-shadow': '#14532d',
  },
  gold: {
    '--badge-base': '#d97706',
    '--badge-edge': '#92400e',
    '--badge-glow': '#fde68a',
    '--badge-shine': '#fffbeb',
    '--badge-spark': '#fef3c7',
    '--badge-shadow': '#78350f',
  },
  indigo: {
    '--badge-base': '#4f46e5',
    '--badge-edge': '#1d4ed8',
    '--badge-glow': '#a5b4fc',
    '--badge-shine': '#eef2ff',
    '--badge-spark': '#c7d2fe',
    '--badge-shadow': '#312e81',
  },
  rose: {
    '--badge-base': '#e11d48',
    '--badge-edge': '#be123c',
    '--badge-glow': '#fda4af',
    '--badge-shine': '#fff1f2',
    '--badge-spark': '#ffe4e6',
    '--badge-shadow': '#881337',
  },
  violet: {
    '--badge-base': '#7c3aed',
    '--badge-edge': '#5b21b6',
    '--badge-glow': '#c4b5fd',
    '--badge-shine': '#f5f3ff',
    '--badge-spark': '#ddd6fe',
    '--badge-shadow': '#4c1d95',
  },
}

const tierMap: Record<AchievementTier, string> = {
  bronze: 'rgba(251, 191, 36, 0.42)',
  silver: 'rgba(226, 232, 240, 0.58)',
  gold: 'rgba(250, 204, 21, 0.68)',
  platinum: 'rgba(186, 230, 253, 0.78)',
}

const toneStyle = computed(() => ({
  ...toneMap[props.tone],
  '--badge-tier': tierMap[props.tier],
  '--badge-motif': toneMap[props.tone]['--badge-spark'],
}))

const sizeClass = computed(() => ({
  sm: 'h-14 w-14',
  md: 'h-20 w-20',
  lg: 'h-28 w-28',
  xl: 'h-40 w-40',
}[props.size]))

const iconWrapClass = computed(() => ({
  sm: 'h-7 w-7',
  md: 'h-10 w-10',
  lg: 'h-14 w-14',
  xl: 'h-20 w-20',
}[props.size]))

const iconSize = computed(() => ({
  sm: 'sm',
  md: 'md',
  lg: 'xl',
  xl: 'xl',
}[props.size] as 'sm' | 'md' | 'xl'))

const lockSize = computed(() => (props.size === 'sm' ? 'sm' : 'md'))
</script>

<style scoped>
.achievement-badge {
  isolation: isolate;
}

.achievement-badge.is-locked {
  filter: saturate(0.55);
  opacity: 0.72;
}

.badge-tier-ring {
  stroke: var(--badge-tier);
  stroke-dasharray: 5 8;
  stroke-linecap: round;
  stroke-width: 2;
  transform-origin: 60px 60px;
}

.tier-silver .badge-tier-ring {
  stroke-dasharray: 9 6;
}

.tier-gold .badge-tier-ring {
  stroke-dasharray: 14 5;
}

.tier-platinum .badge-tier-ring {
  stroke-dasharray: 2 5 16 5;
  stroke-width: 2.4;
}

.badge-facet {
  mix-blend-mode: screen;
  transform-origin: 60px 60px;
}

.badge-motif {
  opacity: 0.72;
  filter: drop-shadow(0 0 7px var(--badge-glow));
}

.achievement-badge.is-unlocked .badge-aura {
  animation: badgePulse 2.6s ease-in-out infinite;
}

.achievement-badge.is-unlocked .badge-tier-ring {
  animation: badgeRing 6s linear infinite;
}

.achievement-badge.is-unlocked .badge-facet {
  animation: badgeFacet 4.2s ease-in-out infinite;
}

.achievement-badge.is-unlocked .badge-motif {
  animation: badgeMotif 3.2s ease-in-out infinite;
}

.achievement-badge.is-unlocked .badge-sparks {
  animation: badgeTwinkle 2.2s ease-in-out infinite;
  transform-origin: 60px 60px;
}

@keyframes badgePulse {
  0%,
  100% {
    opacity: 0.72;
    transform: scale(0.98);
    transform-origin: 60px 60px;
  }
  50% {
    opacity: 1;
    transform: scale(1.04);
    transform-origin: 60px 60px;
  }
}

@keyframes badgeRing {
  to {
    transform: rotate(360deg);
  }
}

@keyframes badgeFacet {
  0%,
  100% {
    opacity: 0.62;
    transform: scale(0.985);
  }
  50% {
    opacity: 1;
    transform: scale(1.025);
  }
}

@keyframes badgeMotif {
  0%,
  100% {
    opacity: 0.58;
    transform: scale(0.98);
    transform-origin: 60px 60px;
  }
  50% {
    opacity: 0.94;
    transform: scale(1.025);
    transform-origin: 60px 60px;
  }
}

@keyframes badgeTwinkle {
  0%,
  100% {
    opacity: 0.62;
    transform: rotate(0deg) scale(0.96);
  }
  50% {
    opacity: 1;
    transform: rotate(8deg) scale(1.04);
  }
}
</style>
