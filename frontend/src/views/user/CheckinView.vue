<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="page-title">{{ t('checkin.description') }}</h1>
          <p class="page-description">{{ t('checkin.subtitle') }}</p>
        </div>
        <span v-if="status" class="badge" :class="status.checked_in ? 'badge-primary' : 'badge-gray'">{{ t(status.checked_in ? 'checkin.done' : 'checkin.pending') }}</span>
      </div>
      <div v-if="loading && !status" class="flex justify-center py-16"><LoadingSpinner /></div>
      <div v-if="error" class="card flex flex-wrap items-center gap-3 border-red-200 p-4 text-sm text-red-600 dark:border-red-900/50 dark:text-red-400" role="alert">
        {{ error }} <button class="btn btn-secondary btn-sm" :disabled="busy || loading" @click="load">{{ t('checkin.retry') }}</button>
      </div>
      <template v-if="status">
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <section class="card overflow-hidden p-6 text-center">
            <div class="flex flex-wrap items-center justify-between gap-2 text-left">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('checkin.wheel') }}</h2>
              <span class="text-sm text-gray-500 dark:text-dark-400">{{ prizeRange }}</span>
            </div>
            <div class="wheel" role="img" :aria-label="t('checkin.wheel') + ': ' + status.snapshot.prizes.map(p => money(p.cents)).join(', ')">
              <div class="pointer" />
              <div class="disc" :style="{ transform: `rotate(${rotation}deg)` }">
                <span v-for="(prize, index) in status.snapshot.prizes" :key="prize.cents" class="prize" :class="{ jackpot: prize.cents === 100 }" :style="prizeStyle(index)">{{ money(prize.cents) }}</span>
              </div>
              <div class="hub">{{ t('checkin.title') }}</div>
            </div>
            <div v-if="!status.checked_in" class="mb-4">
              <p class="mb-3 text-xs text-gray-500 dark:text-dark-400">{{ t(captchaReady ? 'checkin.verifyHint' : 'checkin.verifyUnavailable') }}</p>
              <CaptchaChallenge v-if="captchaReady" ref="captchaRef"
                :turnstile-enabled="!!captchaSettings?.turnstile_enabled" :turnstile-site-key="captchaSettings?.turnstile_site_key || ''"
                :tencent-enabled="!!captchaSettings?.tencent_captcha_enabled" :tencent-app-id="captchaSettings?.tencent_captcha_app_id || ''" :tencent-region="captchaSettings?.tencent_captcha_region"
                :aliyun-enabled="!!captchaSettings?.aliyun_captcha_enabled" :aliyun-scene-id="captchaSettings?.aliyun_captcha_scene_id" :aliyun-prefix="captchaSettings?.aliyun_captcha_prefix" :aliyun-region="captchaSettings?.aliyun_captcha_region"
                @verify="onCaptchaVerify" @expire="clearCaptcha" @error="onCaptchaError" />
            </div>
            <button class="claim-button btn btn-primary btn-lg w-full" :disabled="busy || loading || status.checked_in || !captchaReady || (!!captchaSettings?.turnstile_enabled && !captchaToken)" @click="spin">{{ t(verifying ? 'checkin.verifying' : busy ? 'checkin.spinning' : status.checked_in ? 'checkin.tomorrow' : 'checkin.spin') }}</button>
            <div v-if="status.checked_in && !busy" class="reward-result mt-4 rounded-xl bg-primary-50 p-3 text-sm font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300" role="status">{{ t('checkin.won', { amount: money(status.reward_cents) }) }}</div>
            <p class="mt-3 text-xs text-gray-500 dark:text-dark-400">{{ t('checkin.autoCredit') }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('checkin.oddsNote') }}</p>
            <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">{{ t('checkin.balance') }} · ${{ (auth.user?.balance ?? 0).toFixed(2) }}</p>
          </section>
          <section class="card p-6">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('checkin.weekly') }}</h2>
              <span class="badge badge-primary">{{ t('checkin.recentWeek') }}</span>
            </div>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('checkin.factorsHint') }}</p>
            <div v-for="factor in factors" :key="factor.key" class="mt-5">
              <div class="mb-2 flex flex-wrap items-center justify-between gap-2 text-sm text-gray-700 dark:text-dark-200">
                <span>{{ t(`checkin.${factor.key}`) }} · {{ factor.max }}%</span>
                <strong class="font-medium text-primary-700 dark:text-primary-300">{{ factor.value }}</strong>
              </div>
              <div class="h-1.5 overflow-hidden rounded-full bg-primary-100 dark:bg-primary-900/30" role="progressbar" :aria-label="t(`checkin.${factor.key}`)" :aria-valuenow="factor.score" :aria-valuemin="0" :aria-valuemax="factor.max">
                <span class="block h-full rounded-full bg-primary-500" :style="{ width: `${factor.score / factor.max * 100}%` }" />
              </div>
              <p class="mt-1.5 text-xs text-gray-500 dark:text-dark-400">{{ t(`checkin.${factor.key}Hint`) }}</p>
            </div>
            <div class="mt-6 flex items-center justify-between gap-3 border-t border-terra-line/80 pt-4 dark:border-dark-700/60">
              <div>
                <span class="text-sm text-gray-700 dark:text-dark-200">{{ t('checkin.score') }}</span>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('checkin.noGuarantee') }}</p>
              </div>
              <div class="whitespace-nowrap">
                <strong class="font-display text-3xl font-bold text-primary-700 dark:text-primary-300">{{ status.snapshot.score.toFixed(1) }}</strong>
                <span class="text-sm text-gray-500 dark:text-dark-400"> / 100</span>
              </div>
            </div>
          </section>
        </div>
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <section class="card p-6">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('checkin.footprints') }}</h2>
              <span class="text-sm text-gray-500 dark:text-dark-400">{{ t('checkin.streakCount', { count: completedStreak }) }}</span>
            </div>
            <div class="my-5 grid grid-cols-7 gap-1.5">
              <div v-for="day in days" :key="day.date" class="text-center text-xs text-gray-500 dark:text-dark-400">
                <span>{{ day.date === status.date ? t('checkin.today') : day.date.slice(5).replace('-', '/') }}</span>
                <b class="mt-1.5 block rounded-xl py-2 text-base font-medium"
                  :class="[day.signed ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : 'bg-terra-sunken text-gray-400 dark:bg-dark-800 dark:text-dark-500', day.date === status.date ? 'ring-1 ring-primary-500' : '']"
                  :aria-label="t(day.signed ? 'checkin.signed' : day.date === status.date ? 'checkin.waiting' : 'checkin.missed')">{{ day.signed ? '✓' : day.date === status.date ? '○' : '—' }}</b>
              </div>
            </div>
            <p v-if="completedStreak >= 7" class="text-sm text-gray-500 dark:text-dark-400">{{ t('checkin.streakMax') }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('checkin.streakReset') }}</p>
          </section>
          <section class="card p-6">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('checkin.recentRewards') }}</h2>
              <span class="text-sm text-gray-500 dark:text-dark-400">{{ t('checkin.recentWeek') }} +{{ money(weeklyReward) }}</span>
            </div>
            <p v-if="!status.history.length" class="py-5 text-sm text-gray-500 dark:text-dark-400">{{ t('checkin.empty') }}</p>
            <ul v-else class="mt-3 divide-y divide-terra-line/80 dark:divide-dark-700/60">
              <li v-for="record in status.history.slice(0, 7)" :key="record.date" class="flex items-center justify-between gap-3 py-2.5 text-sm text-gray-700 dark:text-dark-200">
                <span>{{ record.date }} · {{ t('checkin.title') }}</span>
                <strong class="font-display text-lg text-primary-700 dark:text-primary-300">+{{ money(record.reward_cents) }}</strong>
              </li>
            </ul>
          </section>
        </div>
        <details class="card p-5 text-sm text-gray-500 dark:text-dark-400">
          <summary class="cursor-pointer font-medium text-primary-700 dark:text-primary-300">{{ t('checkin.rules') }}</summary>
          <p class="mt-3">{{ t('checkin.rulesBody') }}</p>
          <div class="my-3 flex flex-wrap gap-2"><span v-for="prize in status.snapshot.prizes" :key="prize.cents" class="badge badge-primary">{{ money(prize.cents) }} · {{ prize.probability.toFixed(2) }}%</span></div>
          <p>{{ t('checkin.thresholds') }}</p>
          <p class="mt-2">{{ t('checkin.frozen') }}</p>
        </details>
        <footer class="flex items-center justify-between gap-3 text-xs text-gray-500 dark:text-dark-400">
          <span>{{ t('checkin.reset') }}</span>
          <button class="btn btn-ghost btn-sm" :disabled="busy || loading" @click="load">{{ t('checkin.refresh') }}</button>
        </footer>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import CaptchaChallenge from '@/components/CaptchaChallenge.vue'
import { useAppStore } from '@/stores/app'
import type { ActionCaptchaRequestProof, PublicSettings } from '@/types'
import { useAuthStore } from '@/stores/auth'
import { claimCheckin, getCheckin, type CheckinStatus } from '@/api/checkin'

const { t } = useI18n()
const auth = useAuthStore()
const app = useAppStore()
const captchaSettings = ref<PublicSettings | null>(null)
const captchaRef = ref<InstanceType<typeof CaptchaChallenge> | null>(null)
const captchaToken = ref('')
const verifying = ref(false)
const captchaReady = computed(() => {
  const c = captchaSettings.value
  if (!c || [c.turnstile_enabled, c.tencent_captcha_enabled, c.aliyun_captcha_enabled].filter(Boolean).length !== 1) return false
  return Boolean(c.turnstile_enabled && c.turnstile_site_key || c.tencent_captcha_enabled && c.tencent_captcha_app_id || c.aliyun_captcha_enabled && c.aliyun_captcha_scene_id && c.aliyun_captcha_prefix)
})
function onCaptchaVerify(token: string) { captchaToken.value = token }
function clearCaptcha() { captchaToken.value = '' }
function onCaptchaError() { clearCaptcha(); error.value = t('checkin.verifyFailed') }
const status = ref<CheckinStatus | null>(null)
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const rotation = ref(0)
let disposed = false
let animationTimer: ReturnType<typeof setTimeout> | undefined
let resetTimer: ReturnType<typeof setInterval> | undefined
let finishAnimation: (() => void) | undefined
const money = (cents: number) => `$${(cents / 100).toFixed(2)}`
const prizeStyle = (index: number) => ({ left: `${50 + 34 * Math.sin(index * 2 * Math.PI / 5)}%`, top: `${50 - 34 * Math.cos(index * 2 * Math.PI / 5)}%`, transform: `translate(-50%, -50%) rotate(${-rotation.value}deg)` })
const prizeRange = computed(() => {
  const cents = status.value?.snapshot.prizes.map(p => p.cents) ?? []
  return cents.length ? `${money(Math.min(...cents))} — ${money(Math.max(...cents))}` : ''
})
const factors = computed(() => {
  if (!status.value) return []
  const s = status.value.snapshot
  return [
    { key: 'activity', max: 40, score: s.activity_score, value: t('checkin.days', { count: s.active_days }) },
    { key: 'streak', max: 40, score: s.streak_score, value: t('checkin.days', { count: s.streak_days }) },
    { key: 'spending', max: 15, score: s.spend_score, value: `$${s.spend.toFixed(2)}` },
    { key: 'recharge', max: 5, score: s.recharge_score, value: t(s.has_recharge ? 'checkin.earned' : 'checkin.notEarned') }
  ]
})
const completedStreak = computed(() => {
  if (!status.value) return 0
  const signed = new Set(status.value.history.map(r => r.date))
  const today = Date.parse(`${status.value.date}T00:00:00Z`)
  let count = 0
  for (let offset = status.value.checked_in ? 0 : 1; count < 7; offset++) {
    if (!signed.has(new Date(today - offset * 86400000).toISOString().slice(0, 10))) break
    count++
  }
  return count
})
const days = computed(() => {
  if (!status.value) return []
  const date = new Date(`${status.value.date}T00:00:00Z`)
  const signed = new Set(status.value.history.map(r => r.date))
  return Array.from({ length: 7 }, (_, i) => {
    const day = new Date(date.getTime() - (6 - i) * 86400000).toISOString().slice(0, 10)
    return { date: day, signed: signed.has(day) }
  })
})
const weeklyReward = computed(() => status.value?.history.filter(r => r.date >= (days.value[0]?.date ?? '') && r.date <= status.value!.date).reduce((sum, r) => sum + r.reward_cents, 0) ?? 0)

async function load() {
  if (loading.value || busy.value) return
  loading.value = true
  error.value = ''
  try {
    const [result, settings] = await Promise.all([getCheckin(), app.fetchPublicSettings()])
    if (!disposed) {
      captchaSettings.value = settings
      status.value = result
      if (result.checked_in) rotation.value = (360 - result.snapshot.prizes.findIndex(p => p.cents === result.reward_cents) * 72) % 360
    }
  } catch { if (!disposed) error.value = t('checkin.loadError') }
  finally { loading.value = false }
}

async function spin() {
  if (busy.value || loading.value || !status.value || status.value.checked_in || !captchaReady.value) return
  busy.value = true
  error.value = ''
  try {
    let proof: ActionCaptchaRequestProof
    if (captchaSettings.value?.turnstile_enabled) {
      if (!captchaToken.value) { error.value = t('checkin.verifyHint'); return }
      proof = { turnstile_token: captchaToken.value }
    } else {
      verifying.value = true
      const challenge = await captchaRef.value?.verifyAction()
      if (!challenge || disposed) return
      proof = captchaSettings.value?.tencent_captcha_enabled
        ? { tencent_captcha_ticket: challenge.token, tencent_captcha_randstr: challenge.randstr }
        : { turnstile_token: challenge.token }
    }
    verifying.value = false
    // Server commits the award before the visual animation starts.
    const result = await claimCheckin(proof)
    if (disposed) return
    const index = result.snapshot.prizes.findIndex(p => p.cents === result.reward_cents)
    rotation.value = Math.ceil(rotation.value / 360) * 360 + 1440 + (360 - index * 72) % 360
    await new Promise<void>(resolve => {
      finishAnimation = resolve
      animationTimer = setTimeout(resolve, window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 3300)
    })
    if (disposed) return
    status.value = result
    // A profile refresh failure must not turn an already committed reward into an error.
    await auth.refreshUser().catch(() => {})
  } catch { if (!disposed) error.value = t('checkin.claimError') }
  finally {
    busy.value = false; verifying.value = false; finishAnimation = undefined
    clearCaptcha()
    if (!disposed) captchaRef.value?.reset()
  }
}

function refreshOnFocus() { if (!document.hidden) void load() }
onMounted(() => {
  void load()
  window.addEventListener('focus', refreshOnFocus)
  document.addEventListener('visibilitychange', refreshOnFocus)
  resetTimer = setInterval(() => {
    if (!document.hidden && status.value && Date.now() >= Date.parse(status.value.next_reset_at)) void load()
  }, 30000)
})
onUnmounted(() => {
  disposed = true
  clearInterval(resetTimer)
  clearTimeout(animationTimer)
  finishAnimation?.()
  window.removeEventListener('focus', refreshOnFocus)
  document.removeEventListener('visibilitychange', refreshOnFocus)
})
</script>

<style scoped>
.wheel { --wheel-panel: #fffaf4; --wheel-line: #ded5c6; --wheel-accent: #345740; position: relative; width: min(100%, 310px); aspect-ratio: 1; margin: 24px auto; }
:global(.dark) .wheel { --wheel-panel: #1a211b; --wheel-line: #3d463c; --wheel-accent: #abc595; }
.disc { position: absolute; inset: 6px; border: 8px solid var(--wheel-panel); outline: 2px solid var(--wheel-line); border-radius: 50%; background: conic-gradient(from -36deg, #e4eddd 0deg 72deg, #f4ead6 72deg 144deg, #ccdcbc 144deg 216deg, #e7d2a9 216deg 288deg, #345740 288deg 360deg); transition: transform 3.2s cubic-bezier(.14, .65, .12, 1); box-shadow: 0 8px 22px #23362918; }
.prize { position: absolute; transform: translate(-50%, -50%); font: 600 22px Literata, Georgia, 'Songti SC', serif; color: #2e3230; transition: transform 3.2s cubic-bezier(.14, .65, .12, 1); }
.prize.jackpot { color: #fffaf4; }
.pointer { position: absolute; top: -5px; left: calc(50% - 11px); border-left: 11px solid transparent; border-right: 11px solid transparent; border-top: 27px solid #705c30; z-index: 2; }
.hub { position: absolute; inset: 36%; border-radius: 50%; background: var(--wheel-panel); border: 1px solid var(--wheel-line); display: flex; align-items: center; justify-content: center; color: var(--wheel-accent); font: 600 15px Literata, Georgia, 'Songti SC', serif; }
@media (max-width: 700px) { .prize { font-size: 19px; } }
@media (prefers-reduced-motion: reduce) { .disc, .prize { transition: none; } }
</style>
