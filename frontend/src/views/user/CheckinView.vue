<template>
  <AppLayout>
    <div class="checkin-page">
      <div class="page-heading">
        <div><p class="eyebrow">A LITTLE LUCK, EVERY DAY</p><h1>{{ t('checkin.description') }}</h1><p class="secondary">{{ t('checkin.subtitle') }}</p></div>
        <span v-if="status" class="status-pill">{{ t(status.checked_in ? 'checkin.done' : 'checkin.pending') }}</span>
      </div>
      <div v-if="loading && !status" class="py-16 flex justify-center"><LoadingSpinner /></div>
      <div v-if="error" class="error-box" role="alert">{{ error }} <button class="btn btn-secondary btn-sm" :disabled="busy || loading" @click="load">{{ t('checkin.retry') }}</button></div>
      <template v-if="status">
        <div class="main-grid">
          <section class="checkin-panel wheel-panel">
            <div class="panel-heading"><h2>{{ t('checkin.wheel') }}</h2><span class="secondary">$0.01 — $1.00</span></div>
            <div class="wheel" role="img" :aria-label="t('checkin.wheel') + ': ' + status.snapshot.prizes.map(p => money(p.cents)).join(', ')">
              <div class="pointer" />
              <div class="disc" :style="{ transform: `rotate(${rotation}deg)` }">
                <span v-for="(prize, index) in status.snapshot.prizes" :key="prize.cents" class="prize" :class="{ jackpot: prize.cents === 100 }" :style="prizeStyle(index)">{{ money(prize.cents) }}</span>
              </div>
              <div class="hub">DAILY<span>{{ t('checkin.title') }}</span></div>
            </div>
            <div v-if="!status.checked_in" class="mb-4">
              <p class="small-note mb-3">{{ t(captchaReady ? 'checkin.verifyHint' : 'checkin.verifyUnavailable') }}</p>
              <CaptchaChallenge v-if="captchaReady" ref="captchaRef"
                :turnstile-enabled="!!captchaSettings?.turnstile_enabled" :turnstile-site-key="captchaSettings?.turnstile_site_key || ''"
                :tencent-enabled="!!captchaSettings?.tencent_captcha_enabled" :tencent-app-id="captchaSettings?.tencent_captcha_app_id || ''" :tencent-region="captchaSettings?.tencent_captcha_region"
                :aliyun-enabled="!!captchaSettings?.aliyun_captcha_enabled" :aliyun-scene-id="captchaSettings?.aliyun_captcha_scene_id" :aliyun-prefix="captchaSettings?.aliyun_captcha_prefix" :aliyun-region="captchaSettings?.aliyun_captcha_region"
                @verify="onCaptchaVerify" @expire="clearCaptcha" @error="onCaptchaError" />
            </div>
            <button class="btn btn-primary claim-button" :disabled="busy || loading || status.checked_in || !captchaReady || (!!captchaSettings?.turnstile_enabled && !captchaToken)" @click="spin">{{ t(verifying ? 'checkin.verifying' : busy ? 'checkin.spinning' : status.checked_in ? 'checkin.tomorrow' : 'checkin.spin') }}</button>
            <div v-if="status.checked_in && !busy" class="reward-result" role="status">✦ {{ t('checkin.won', { amount: money(status.reward_cents) }) }}</div>
            <p class="small-note">{{ t('checkin.autoCredit') }}</p><p class="small-note">{{ t('checkin.oddsNote') }}</p>
            <p class="secondary mt-3">{{ t('checkin.balance') }} · ${{ (auth.user?.balance ?? 0).toFixed(2) }}</p>
          </section>
          <section class="checkin-panel">
            <div class="panel-heading"><h2>{{ t('checkin.weekly') }}</h2><span class="status-pill">{{ t('checkin.recentWeek') }}</span></div>
            <p class="secondary">{{ t('checkin.factorsHint') }}</p>
            <div v-for="factor in factors" :key="factor.key" class="factor">
              <div class="factor-heading"><span>{{ t(`checkin.${factor.key}`) }} · {{ factor.max }}%</span><strong>{{ factor.value }}</strong></div>
              <div class="factor-track" role="progressbar" :aria-label="t(`checkin.${factor.key}`)" :aria-valuenow="factor.score" :aria-valuemin="0" :aria-valuemax="factor.max"><span :style="{ width: `${factor.score / factor.max * 100}%` }" /></div>
              <p class="small-note">{{ t(`checkin.${factor.key}Hint`) }}</p>
            </div>
            <div class="score-row"><div><span class="secondary">{{ t('checkin.score') }}</span><p class="small-note">{{ t('checkin.noGuarantee') }}</p></div><div><strong>{{ status.snapshot.score.toFixed(1) }}</strong><span class="secondary"> / 100</span></div></div>
          </section>
        </div>
        <div class="main-grid mt-5">
          <section class="checkin-panel">
            <div class="panel-heading"><h2>{{ t('checkin.footprints') }}</h2><span class="secondary">{{ t('checkin.streakCount', { count: completedStreak }) }}</span></div>
            <div class="days">
              <div v-for="day in days" :key="day.date" class="day" :class="{ signed: day.signed, today: day.date === status.date }"><span>{{ day.date === status.date ? t('checkin.today') : day.date.slice(5).replace('-', '/') }}</span><b :aria-label="t(day.signed ? 'checkin.signed' : day.date === status.date ? 'checkin.waiting' : 'checkin.missed')">{{ day.signed ? '✓' : day.date === status.date ? '○' : '—' }}</b></div>
            </div>
            <p v-if="completedStreak >= 7" class="secondary">{{ t('checkin.streakMax') }}</p><p class="small-note">{{ t('checkin.streakReset') }}</p>
          </section>
          <section class="checkin-panel">
            <div class="panel-heading"><h2>{{ t('checkin.recentRewards') }}</h2><span class="secondary">{{ t('checkin.recentWeek') }} +{{ money(weeklyReward) }}</span></div>
            <p v-if="!status.history.length" class="secondary py-5">{{ t('checkin.empty') }}</p>
            <ul v-else class="rewards"><li v-for="record in status.history.slice(0, 7)" :key="record.date"><span>{{ record.date }} · {{ t('checkin.title') }}</span><strong>+{{ money(record.reward_cents) }}</strong></li></ul>
          </section>
        </div>
        <details class="rules"><summary>{{ t('checkin.rules') }}</summary><p>{{ t('checkin.rulesBody') }}</p><div class="odds"><span v-for="prize in status.snapshot.prizes" :key="prize.cents">{{ money(prize.cents) }} · {{ prize.probability.toFixed(2) }}%</span></div><p>{{ t('checkin.thresholds') }}</p><p>{{ t('checkin.frozen') }}</p></details>
        <footer><span>{{ t('checkin.reset') }}</span><button class="btn btn-ghost btn-sm" :disabled="busy || loading" @click="load">{{ t('checkin.refresh') }}</button></footer>
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
.wheel-panel { overflow: hidden; }
.prize { transition: transform 3.2s cubic-bezier(.14,.65,.12,1); }
.score-row > div:last-child { white-space: nowrap; }
.checkin-page { --paper: #faf6f0; --panel: #fffaf4; --ink: #2e3230; --muted: #6f746a; --line: #ded5c6; --soft: #e4eddd; --green: #345740; color: var(--ink); max-width: 1120px; margin: 0 auto; }
.dark .checkin-page { --paper: #101511; --panel: #1a211b; --ink: #ece6da; --muted: #b6aa98; --line: #3d463c; --soft: #273b2d; --green: #abc595; }
.page-heading,.panel-heading,.factor-heading,.score-row,footer { display:flex; justify-content:space-between; align-items:center; gap:12px; }
.page-heading { margin-bottom:24px; }.eyebrow { color:var(--muted); font-size:10px; letter-spacing:2px; margin-bottom:8px; }h1 { font-size:28px; font-weight:500; margin-bottom:8px; }h2 { font-size:16px; font-weight:600; }.secondary { color:var(--muted); font-size:12px; }.status-pill { color:var(--green); background:var(--soft); padding:5px 10px; border-radius:20px; font-size:12px; white-space:nowrap; }
.main-grid { display:grid; grid-template-columns:minmax(0,1.1fr) minmax(0,1fr); gap:20px; }.checkin-panel { padding:24px; background:var(--panel); border:1px solid var(--line); border-radius:16px; }.wheel-panel { text-align:center; }.wheel { position:relative; width:min(100%,310px); aspect-ratio:1; margin:24px auto; }.disc { position:absolute; inset:6px; border:8px solid var(--panel); outline:2px solid var(--line); border-radius:50%; background:conic-gradient(from -36deg,#e4eddd 0deg 72deg,#f4ead6 72deg 144deg,#ccdcbc 144deg 216deg,#e7d2a9 216deg 288deg,#345740 288deg 360deg); transition:transform 3.2s cubic-bezier(.14,.65,.12,1); box-shadow:0 8px 22px #23362918; }.prize { position:absolute; transform:translate(-50%,-50%); font:500 23px Georgia,serif; color:#2e3230; }.prize.jackpot { color:#fffaf4; }.pointer { position:absolute; top:-5px; left:calc(50% - 11px); border-left:11px solid transparent; border-right:11px solid transparent; border-top:27px solid #705c30; z-index:2; }.hub { position:absolute; inset:36%; border-radius:50%; background:var(--panel); border:1px solid var(--line); display:flex; flex-direction:column; align-items:center; justify-content:center; color:var(--green); font:500 21px Georgia,serif; }.hub span { font:11px 'Microsoft YaHei',sans-serif; margin-top:4px; }.claim-button { width:100%; }.small-note { font-size:11px; color:var(--muted); margin-top:8px; }.reward-result { margin-top:14px; padding:12px; border-radius:10px; background:var(--soft); color:var(--green); }.factor { margin-top:24px; }.factor-heading { font-size:13px; margin-bottom:8px; flex-wrap:wrap; }.factor-heading strong { color:var(--green); font-weight:500; }.factor-track { height:5px; background:var(--soft); border-radius:8px; overflow:hidden; }.factor-track span { display:block; height:100%; background:var(--green); }.score-row { border-top:1px solid var(--line); margin-top:24px; padding-top:18px; }.score-row strong { font:32px Georgia,serif; color:var(--green); }.days { display:grid; grid-template-columns:repeat(7,minmax(0,1fr)); gap:5px; margin:20px 0; }.day { text-align:center; font-size:11px; color:var(--muted); }.day b { display:block; padding:9px 0; margin-top:6px; border-radius:9px; background:var(--paper); font-size:16px; font-weight:500; }.day.signed b { background:var(--soft); color:var(--green); }.day.today b { outline:1px dashed var(--green); }.rewards { margin-top:12px; }.rewards li { display:flex; align-items:center; justify-content:space-between; gap:10px; border-bottom:1px solid var(--line); padding:9px 0; font-size:12px; }.rewards strong { font:18px Georgia,serif; color:var(--green); }.rules { border-top:1px solid var(--line); margin-top:22px; padding-top:18px; font-size:12px; color:var(--muted); }.rules summary { cursor:pointer; color:var(--green); min-height:35px; }.rules p { margin:10px 0; }.odds { display:flex; flex-wrap:wrap; gap:8px; margin:14px 0; }.odds span { background:var(--soft); color:var(--green); padding:6px 10px; border-radius:6px; }footer { margin-top:16px; color:var(--muted); font-size:11px; }.error-box { padding:16px; margin-bottom:20px; border:1px solid var(--line); border-radius:12px; }.panel-heading { flex-wrap:wrap; }
@media(max-width:700px) { .main-grid { grid-template-columns:1fr; }.checkin-panel { padding:18px; }h1 { font-size:23px; }.page-heading { align-items:flex-start; flex-wrap:wrap; }.prize { font-size:20px; } }
@media(prefers-reduced-motion:reduce) { .disc,.prize { transition:none; } }
</style>
