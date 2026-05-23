<template>
  <div v-if="user" ref="rootRef" class="relative">
    <button
      type="button"
      class="inline-flex h-9 items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-2.5 text-sm font-semibold text-amber-800 transition hover:border-amber-300 hover:bg-amber-100 focus:outline-none focus:ring-2 focus:ring-amber-300/70 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200 dark:hover:bg-amber-900/30"
      :aria-expanded="panelOpen"
      aria-haspopup="dialog"
      @click="togglePanel"
    >
      <Icon name="sparkles" size="sm" :stroke-width="2" />
      <span class="hidden sm:inline">成长</span>
      <span
        v-if="stats && !starterTasksCompleted"
        class="rounded-full bg-white px-1.5 py-0.5 text-[11px] leading-none text-amber-700 shadow-sm dark:bg-dark-900 dark:text-amber-200"
      >
        {{ completedTaskCount }}/{{ starterTasks.length }}
      </span>
    </button>

    <transition name="dropdown">
      <div
        v-if="panelOpen"
        class="absolute right-0 z-50 mt-3 w-[min(92vw,420px)] overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-900"
        role="dialog"
        aria-label="成长中心"
      >
        <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="flex items-center gap-2">
                <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
                  <Icon name="sparkles" size="sm" :stroke-width="2" />
                </span>
                <div>
                  <h2 class="text-sm font-semibold text-gray-900 dark:text-white">成长中心</h2>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                    {{ growthSummaryText }}
                  </p>
                </div>
              </div>
            </div>
            <button
              type="button"
              class="rounded-lg p-1.5 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-800 dark:hover:text-dark-200"
              @click="panelOpen = false"
            >
              <Icon name="x" size="sm" :stroke-width="2" />
            </button>
          </div>
        </div>

        <div class="max-h-[calc(100vh-5rem)] space-y-4 overflow-y-auto p-4">
          <div v-if="loadError" class="rounded-xl border border-red-100 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
            {{ loadError }}
          </div>

          <div class="rounded-xl border border-emerald-100 bg-emerald-50/70 p-3 dark:border-emerald-900/50 dark:bg-emerald-900/[0.14]">
            <div class="flex items-center justify-between gap-3">
              <div class="min-w-0">
                <p class="text-xs font-medium text-emerald-700 dark:text-emerald-300">初始额度已到账</p>
                <p class="mt-1 truncate text-lg font-semibold text-emerald-900 dark:text-emerald-100">
                  ${{ formatCurrency(user.balance || 0) }}
                </p>
              </div>
              <button
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-lg bg-white px-2.5 text-xs font-semibold text-emerald-700 shadow-sm ring-1 ring-emerald-200 transition hover:bg-emerald-50 dark:bg-dark-900 dark:text-emerald-300 dark:ring-emerald-900/60"
                @click="goToBalance"
              >
                <Icon name="eye" size="xs" :stroke-width="2" />
                <span>查看余额</span>
              </button>
            </div>
          </div>

          <template v-if="!starterTasksCompleted">
            <div>
              <div class="mb-2 flex items-center justify-between text-xs">
                <span class="font-medium text-gray-600 dark:text-dark-300">新人任务</span>
                <span class="text-gray-500 dark:text-dark-400">{{ Math.round(taskProgress * 100) }}%</span>
              </div>
              <div class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                <div
                  class="h-full rounded-full bg-amber-500 transition-all duration-500"
                  :style="{ width: `${Math.round(taskProgress * 100)}%` }"
                ></div>
              </div>
            </div>

            <div class="space-y-2">
              <button
                v-for="task in starterTasks"
                :key="task.id"
                type="button"
                class="group flex w-full items-center gap-3 rounded-xl border px-3 py-2.5 text-left transition disabled:cursor-not-allowed"
                :disabled="task.disabled"
                :class="task.done
                  ? 'border-emerald-100 bg-emerald-50/60 dark:border-emerald-900/40 dark:bg-emerald-900/[0.12]'
                  : task.disabled
                    ? 'border-gray-100 bg-gray-50 opacity-75 dark:border-dark-700 dark:bg-dark-800/40'
                    : 'border-gray-100 bg-gray-50 hover:border-amber-200 hover:bg-white dark:border-dark-700 dark:bg-dark-800/60 dark:hover:border-amber-900/70 dark:hover:bg-dark-800'"
                @click="handleTaskAction(task)"
              >
                <span
                  class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
                  :class="task.done
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/50 dark:text-emerald-300'
                    : task.disabled
                      ? 'bg-white text-gray-400 ring-1 ring-gray-200 dark:bg-dark-900 dark:text-dark-500 dark:ring-dark-700'
                      : 'bg-white text-gray-500 ring-1 ring-gray-200 group-hover:text-amber-700 dark:bg-dark-900 dark:text-dark-300 dark:ring-dark-700 dark:group-hover:text-amber-200'"
                >
                  <Icon :name="taskIconName(task)" size="sm" :stroke-width="2" />
                </span>
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ task.title }}</span>
                  <span class="mt-0.5 block truncate text-xs text-gray-500 dark:text-dark-400">
                    {{ task.done ? '已完成' : task.progress }}
                  </span>
                </span>
                <span
                  class="inline-flex shrink-0 items-center gap-1 text-xs font-semibold"
                  :class="task.disabled ? 'text-gray-400 dark:text-dark-500' : 'text-amber-700 dark:text-amber-200'"
                >
                  {{ task.action }}
                  <Icon name="chevronRight" size="xs" :stroke-width="2" />
                </span>
              </button>
            </div>

            <div
              v-if="communityGuideVisible"
              class="overflow-hidden rounded-xl border border-amber-100 bg-amber-50/70 dark:border-amber-900/50 dark:bg-amber-900/[0.14]"
            >
              <div class="flex items-center justify-between gap-3 px-3 pt-3">
                <div class="flex min-w-0 items-center gap-2">
                  <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-white text-amber-700 ring-1 ring-amber-200 dark:bg-dark-900 dark:text-amber-200 dark:ring-amber-900/60">
                    <Icon name="users" size="sm" :stroke-width="2" />
                  </span>
                  <div class="min-w-0">
                    <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">社群福利</p>
                    <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400">
                      {{ communityWelfareUnlocked ? '加入社群后兑换福利码' : '完成前三步后兑换福利码' }}
                    </p>
                  </div>
                </div>
                <a
                  v-if="communityLinkURL"
                  :href="communityLinkURL"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg bg-white px-2.5 text-xs font-semibold text-amber-700 shadow-sm ring-1 ring-amber-200 transition hover:bg-amber-50 dark:bg-dark-900 dark:text-amber-200 dark:ring-amber-900/60"
                  @click.stop
                >
                  <span>打开社群</span>
                  <Icon name="externalLink" size="xs" :stroke-width="2" />
                </a>
              </div>
              <div v-if="communityImageURL" class="px-3 pb-3 pt-2">
                <img
                  :src="communityImageURL"
                  alt="社群入口"
                  class="h-40 w-full rounded-lg border border-amber-100 bg-white object-contain p-2 dark:border-amber-900/40 dark:bg-dark-950"
                />
              </div>
            </div>
          </template>

          <div class="rounded-xl border border-gray-100 p-3 dark:border-dark-700">
            <div class="mb-3 flex items-center justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">成就徽章</h3>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">点击徽章查看说明</p>
              </div>
              <button
                type="button"
                class="inline-flex h-7 items-center gap-1 rounded-lg px-2 text-xs font-semibold text-gray-600 transition hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
                @click="galleryOpen = true"
              >
                <Icon name="grid" size="xs" :stroke-width="2" />
                全部
              </button>
            </div>

            <div class="grid grid-cols-4 gap-2">
              <button
                v-for="achievement in previewAchievements"
                :key="achievement.id"
                type="button"
                class="group flex min-w-0 flex-col items-center rounded-lg p-1.5 transition hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-amber-300 dark:hover:bg-dark-800"
                @click="openAchievement(achievement)"
              >
                <AchievementBadge
                  :icon="achievement.icon"
                  :motif="achievement.motif"
                  :tone="achievement.tone"
                  :tier="achievement.tier"
                  :title="achievement.name"
                  :unlocked="achievement.unlocked"
                  size="sm"
                />
                <span class="mt-1 w-full truncate text-center text-[11px] font-medium text-gray-700 dark:text-dark-200">
                  {{ achievement.name }}
                </span>
              </button>
            </div>

            <div v-if="nextAchievement" class="mt-4 rounded-lg bg-gray-50 p-3 dark:bg-dark-800/60">
              <div class="flex items-center justify-between gap-3">
                <div class="min-w-0">
                  <p class="text-xs font-medium text-gray-500 dark:text-dark-400">下一枚徽章</p>
                  <button
                    type="button"
                    class="mt-0.5 truncate text-left text-sm font-semibold text-gray-900 transition hover:text-amber-700 dark:text-white dark:hover:text-amber-200"
                    @click="openAchievement(nextAchievement)"
                  >
                    {{ nextAchievement.name }}
                  </button>
                </div>
                <span class="shrink-0 text-xs font-medium text-gray-500 dark:text-dark-400">
                  {{ nextAchievement.progress(ctx).label }}
                </span>
              </div>
              <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
                <div
                  class="h-full rounded-full bg-amber-500 transition-all duration-500"
                  :style="{ width: `${Math.round(nextAchievement.progressValue * 100)}%` }"
                ></div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </transition>

    <Teleport to="body">
      <div
        v-if="selectedAchievement"
        class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/55 p-4 backdrop-blur-sm"
        @click.self="selectedAchievement = null"
      >
        <div class="w-full max-w-md rounded-2xl bg-white shadow-2xl dark:bg-dark-900">
          <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">徽章说明</h3>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                {{ selectedAchievement.unlocked ? '已解锁' : '未解锁' }}
              </p>
            </div>
            <button
              type="button"
              class="rounded-lg p-2 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-800 dark:hover:text-dark-200"
              @click="selectedAchievement = null"
            >
              <Icon name="x" size="sm" :stroke-width="2" />
            </button>
          </div>
          <div class="p-5">
            <div class="flex items-start gap-4">
              <AchievementBadge
                :icon="selectedAchievement.icon"
                :motif="selectedAchievement.motif"
                :tone="selectedAchievement.tone"
                :tier="selectedAchievement.tier"
                :title="selectedAchievement.name"
                :unlocked="selectedAchievement.unlocked"
                size="md"
              />
              <div class="min-w-0 flex-1">
                <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ selectedAchievement.name }}</p>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">{{ selectedAchievement.title }}</p>
                <p class="mt-3 text-sm leading-6 text-gray-500 dark:text-dark-400">{{ selectedAchievement.description }}</p>
              </div>
            </div>

            <div class="mt-5 grid gap-3 text-sm">
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800/70">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">解锁条件</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ selectedAchievement.condition }}</p>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800/70">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">奖励/权益</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ selectedAchievement.reward }}</p>
              </div>
            </div>

            <div class="mt-5">
              <div class="mb-2 flex items-center justify-between text-xs">
                <span class="font-medium text-gray-500 dark:text-dark-400">当前进度</span>
                <span class="font-semibold text-gray-700 dark:text-dark-200">{{ selectedAchievement.progress(ctx).label }}</span>
              </div>
              <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                <div
                  class="h-full rounded-full bg-amber-500"
                  :style="{ width: `${Math.round(selectedAchievement.progressValue * 100)}%` }"
                ></div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="galleryOpen"
        class="fixed inset-0 z-40 flex items-center justify-center bg-slate-950/55 p-4 backdrop-blur-sm"
        @click.self="galleryOpen = false"
      >
        <div class="w-full max-w-3xl rounded-2xl bg-white shadow-2xl dark:bg-dark-900">
          <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">全部成就徽章</h3>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                已解锁 {{ unlockedCount }}/{{ achievements.length }}，点击任一徽章查看详情
              </p>
            </div>
            <button
              type="button"
              class="rounded-lg p-2 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-800 dark:hover:text-dark-200"
              @click="galleryOpen = false"
            >
              <Icon name="x" size="sm" :stroke-width="2" />
            </button>
          </div>
          <div class="max-h-[70vh] overflow-y-auto p-5">
            <div class="grid gap-3 sm:grid-cols-2">
              <button
                v-for="achievement in achievements"
                :key="achievement.id"
                type="button"
                class="flex items-center gap-3 rounded-xl border border-gray-100 p-3 text-left transition hover:border-amber-200 hover:bg-gray-50 dark:border-dark-700 dark:hover:border-amber-900/60 dark:hover:bg-dark-800/70"
                @click="openAchievementFromGallery(achievement)"
              >
                <AchievementBadge
                  :icon="achievement.icon"
                  :motif="achievement.motif"
                  :tone="achievement.tone"
                  :tier="achievement.tier"
                  :title="achievement.name"
                  :unlocked="achievement.unlocked"
                  size="sm"
                />
                <span class="min-w-0 flex-1">
                  <span class="flex items-center gap-2">
                    <span class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ achievement.name }}</span>
                    <span
                      class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium"
                      :class="achievement.unlocked ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-400'"
                    >
                      {{ achievement.unlocked ? '已解锁' : achievement.progress(ctx).label }}
                    </span>
                  </span>
                  <span class="mt-1 block truncate text-xs text-gray-500 dark:text-dark-400">{{ achievement.condition }}</span>
                </span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <BadgeUnlockOverlay :achievement="activeUnlock" @close="closeUnlockOverlay" />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter, type LocationQueryRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { authAPI } from '@/api/auth'
import userAPI from '@/api/user'
import { usageAPI, type UserDashboardStats } from '@/api/usage'
import type { PublicSettings, UserAffiliateDetail, UserGrowthStatus } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import AchievementBadge from '@/components/user/dashboard/AchievementBadge.vue'
import BadgeUnlockOverlay from '@/components/user/dashboard/BadgeUnlockOverlay.vue'
import {
  formatPercent,
  resolveActivationAchievements,
  type ActivationAchievement,
  type ActivationContext,
  type AchievementIcon,
} from '@/components/user/dashboard/activationAchievements'

interface StarterTask {
  id: 'key' | 'request' | 'community' | 'affiliate_tutorial'
  title: string
  icon: AchievementIcon
  done: boolean
  progress: string
  action: string
  to?: string
  query?: LocationQueryRaw
  disabled?: boolean
}

type StarterTaskIcon = AchievementIcon | 'check' | 'lock'

const router = useRouter()
const authStore = useAuthStore()
const user = computed(() => authStore.user)

const rootRef = ref<HTMLElement | null>(null)
const panelOpen = ref(false)
const loading = ref(false)
const loadError = ref('')
const stats = ref<UserDashboardStats | null>(null)
const affiliate = ref<UserAffiliateDetail | null>(null)
const growth = ref<UserGrowthStatus | null>(null)
const publicSettings = ref<PublicSettings | null>(null)
const selectedAchievement = ref<ActivationAchievement | null>(null)
const galleryOpen = ref(false)
const activeUnlock = ref<ActivationAchievement | null>(null)
let unlockTimer: ReturnType<typeof setTimeout> | null = null

const emptyStats: UserDashboardStats = {
  total_api_keys: 0,
  active_api_keys: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  average_duration_ms: 0,
  rpm: 0,
  tpm: 0,
}

const currentStats = computed(() => stats.value ?? emptyStats)

const ctx = computed<ActivationContext>(() => ({
  stats: currentStats.value,
  user: user.value,
  affiliate: affiliate.value,
  growth: growth.value,
}))

const achievements = computed(() => stats.value ? resolveActivationAchievements(ctx.value) : [])
const unlockedAchievements = computed(() => achievements.value.filter((item) => item.unlocked))
const unlockedCount = computed(() => unlockedAchievements.value.length)
const previewAchievements = computed(() => [
  ...unlockedAchievements.value,
  ...achievements.value.filter((item) => !item.unlocked),
].slice(0, 4))
const nextAchievement = computed(() => achievements.value.find((item) => !item.unlocked) ?? null)

function rateForLevel(level: number): number {
  const rows = affiliate.value?.effective_level_rates ?? []
  const match = rows.find((item) => item.level === level)
  if (match) return match.rate_percent
  if (level === 1) return affiliate.value?.effective_rebate_rate_percent ?? 5
  return 0
}

const secondLevelRate = computed(() => formatPercent(rateForLevel(2)))
const thirdLevelRate = computed(() => formatPercent(rateForLevel(3)))
const hasCreatedAPIKey = computed(() => currentStats.value.total_api_keys > 0)
const hasFirstRequest = computed(() => currentStats.value.total_requests > 0)
const hasAffiliateTutorial = computed(() => (
  growth.value?.affiliate_tutorial_done === true || (affiliate.value?.aff_count ?? 0) > 0
))
const communityWelfareUnlocked = computed(() => (
  hasCreatedAPIKey.value && hasFirstRequest.value && hasAffiliateTutorial.value
))

const starterTasks = computed<StarterTask[]>(() => {
  const current = currentStats.value
  const communityJoined = growth.value?.community_joined === true
  const communityLocked = !communityJoined && !communityWelfareUnlocked.value
  return [
    {
      id: 'key',
      title: '创建 API Key',
      icon: 'key',
      done: hasCreatedAPIKey.value,
      progress: `${Math.min(current.total_api_keys, 1)}/1`,
      action: '去创建',
      to: '/keys',
    },
    {
      id: 'request',
      title: '完成首次调用',
      icon: 'terminal',
      done: hasFirstRequest.value,
      progress: `${Math.min(current.total_requests, 1)}/1`,
      action: '看日志',
      to: '/usage',
    },
    {
      id: 'affiliate_tutorial',
      title: '了解邀请返利',
      icon: 'users',
      done: hasAffiliateTutorial.value,
      progress: `二级 ${secondLevelRate.value}% / 三级 ${thirdLevelRate.value}%`,
      action: '去了解',
      to: '/affiliate',
      query: { growth_tutorial: '1' },
    },
    {
      id: 'community',
      title: '兑换群福利',
      icon: 'gift',
      done: communityJoined,
      progress: communityLocked ? '完成前三步后可兑换 QQ 福利码' : '加入 QQ 群后兑换福利码',
      action: communityLocked ? '待解锁' : '去兑换',
      to: communityLocked ? undefined : '/redeem',
      query: { growth_task: 'community' },
      disabled: communityLocked,
    },
  ]
})

const completedTaskCount = computed(() => starterTasks.value.filter((task) => task.done).length)
const taskProgress = computed(() => (
  starterTasks.value.length > 0 ? completedTaskCount.value / starterTasks.value.length : 0
))
const starterTasksCompleted = computed(() => (
  starterTasks.value.length > 0 && completedTaskCount.value >= starterTasks.value.length
))
const communityImageURL = computed(() => publicSettings.value?.community_image_url?.trim() ?? '')
const communityLinkURL = computed(() => normalizeHttpURL(publicSettings.value?.community_link_url))
const communityGuideVisible = computed(() => (
  !starterTasksCompleted.value
  && growth.value?.community_joined !== true
  && (communityImageURL.value !== '' || communityLinkURL.value !== '')
))
const growthSummaryText = computed(() => {
  if (starterTasksCompleted.value) {
    return `${unlockedCount.value} 枚徽章`
  }
  return `新手进度 ${completedTaskCount.value}/${starterTasks.value.length} · ${unlockedCount.value} 枚徽章`
})

function taskIconName(task: StarterTask): StarterTaskIcon {
  if (task.done) return 'check'
  if (task.disabled) return 'lock'
  return task.icon
}

function normalizeHttpURL(value?: string | null): string {
  const raw = value?.trim()
  if (!raw) return ''
  try {
    const url = new URL(raw)
    return url.protocol === 'http:' || url.protocol === 'https:' ? raw : ''
  } catch {
    return ''
  }
}

async function loadGrowthData(silent = false): Promise<void> {
  if (!user.value) return
  if (!silent) loading.value = true
  loadError.value = ''
  const [statsResult, affiliateResult, growthResult, settingsResult] = await Promise.allSettled([
    usageAPI.getDashboardStats(),
    userAPI.getAffiliateDetail(),
    userAPI.getGrowthStatus(),
    authAPI.getPublicSettings(),
  ])

  if (statsResult.status === 'fulfilled') {
    stats.value = statsResult.value
  }
  if (affiliateResult.status === 'fulfilled') {
    affiliate.value = affiliateResult.value
  }
  if (growthResult.status === 'fulfilled') {
    growth.value = growthResult.value
  }
  if (settingsResult.status === 'fulfilled') {
    publicSettings.value = settingsResult.value
  }

  if (statsResult.status === 'rejected' || growthResult.status === 'rejected') {
    loadError.value = '成长状态加载失败，请稍后重试'
  }
  loading.value = false
}

function togglePanel(): void {
  panelOpen.value = !panelOpen.value
  if (panelOpen.value) {
    loadGrowthData(true)
  }
}

function handleTaskAction(task: StarterTask): void {
  if (task.disabled) return
  if (!task.to) return
  panelOpen.value = false
  router.push({ path: task.to, query: task.query })
}

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value)
}

function focusBalanceCard(): void {
  const target = document.getElementById('dashboard-balance-card')
  if (!target) return
  target.scrollIntoView({ behavior: 'smooth', block: 'center' })
  target.focus({ preventScroll: true })
}

async function goToBalance(): Promise<void> {
  panelOpen.value = false
  await router.push('/dashboard')
  window.setTimeout(focusBalanceCard, 220)
}

function openAchievement(achievement: ActivationAchievement): void {
  selectedAchievement.value = achievement
}

function openAchievementFromGallery(achievement: ActivationAchievement): void {
  galleryOpen.value = false
  selectedAchievement.value = achievement
}

function closeUnlockOverlay(): void {
  activeUnlock.value = null
  if (unlockTimer) {
    clearTimeout(unlockTimer)
    unlockTimer = null
  }
}

function showUnlockOverlay(achievement: ActivationAchievement): void {
  activeUnlock.value = achievement
  if (unlockTimer) clearTimeout(unlockTimer)
  unlockTimer = setTimeout(() => {
    activeUnlock.value = null
    unlockTimer = null
  }, 5200)
}

function seenStorageKey(): string | null {
  const userID = user.value?.id
  return userID ? `activation_achievements_seen:${userID}` : null
}

function readSeenAchievements(key: string): Set<string> {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return new Set()
    const parsed = JSON.parse(raw)
    return new Set(Array.isArray(parsed) ? parsed.filter((item) => typeof item === 'string') : [])
  } catch {
    return new Set()
  }
}

function writeSeenAchievements(key: string, ids: string[]): void {
  localStorage.setItem(key, JSON.stringify(Array.from(new Set(ids))))
}

function isRecentlyCreatedUser(): boolean {
  const createdAt = user.value?.created_at
  if (!createdAt) return false
  const createdTime = new Date(createdAt).getTime()
  if (!Number.isFinite(createdTime)) return false
  return Date.now() - createdTime < 30 * 60 * 1000
}

function handleDocumentMouseDown(event: MouseEvent): void {
  if (!panelOpen.value) return
  const root = rootRef.value
  if (root && event.target instanceof Node && root.contains(event.target)) return
  panelOpen.value = false
}

function handleKeyDown(event: KeyboardEvent): void {
  if (event.key === 'Escape') {
    panelOpen.value = false
    selectedAchievement.value = null
    galleryOpen.value = false
  }
}

function handleGrowthRefresh(): void {
  loadGrowthData(true)
}

watch(achievements, (items) => {
  const key = seenStorageKey()
  if (!key || items.length === 0) return
  const unlocked = items.filter((item) => item.unlocked)
  if (unlocked.length === 0) return

  const hasStored = localStorage.getItem(key) !== null
  const seen = readSeenAchievements(key)
  if (!hasStored && !isRecentlyCreatedUser()) {
    writeSeenAchievements(key, unlocked.map((item) => item.id))
    return
  }

  const newlyUnlocked = unlocked.find((item) => !seen.has(item.id))
  writeSeenAchievements(key, unlocked.map((item) => item.id))
  if (newlyUnlocked) {
    showUnlockOverlay(newlyUnlocked)
  }
}, { deep: true })

watch(user, (next) => {
  if (next) {
    loadGrowthData(true)
  } else {
    stats.value = null
    affiliate.value = null
    growth.value = null
    publicSettings.value = null
  }
}, { immediate: true })

onMounted(() => {
  document.addEventListener('mousedown', handleDocumentMouseDown)
  document.addEventListener('keydown', handleKeyDown)
  window.addEventListener('growth:center-refresh', handleGrowthRefresh)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', handleDocumentMouseDown)
  document.removeEventListener('keydown', handleKeyDown)
  window.removeEventListener('growth:center-refresh', handleGrowthRefresh)
  if (unlockTimer) clearTimeout(unlockTimer)
})
</script>
