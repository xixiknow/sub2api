<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>


  <!-- Default Home Page: 星空落地页 -->
  <div v-else class="star-home relative min-h-screen overflow-x-hidden" :class="{ dark: landingDark }">
    <!-- 背景：星云 + 星空 -->
    <div class="star-nebula pointer-events-none fixed inset-0" aria-hidden="true"></div>
    <StarfieldCanvas
      class="fixed inset-0 z-0"
      :warp="warpLevel"
      :zoom="heroZoom"
      :reduced-motion="reducedMotion"
      :links="false"
      :light="!landingDark"
    />
    <ParticleField :reduced-motion="reducedMotion" :burst="particleBurst" :light="!landingDark" />
    <div class="star-warp-flash pointer-events-none fixed inset-0 z-40" :class="{ 'is-on': warpLevel >= 1 }" aria-hidden="true"></div>

    <!-- Header -->
    <header
      class="fixed inset-x-0 top-0 z-30 transition-all duration-300"
      :class="headerSolid ? 'border-b border-slate-900/[0.06] bg-white/70 backdrop-blur-xl dark:border-white/[0.06] dark:bg-[#05060f]/70' : 'border-b border-transparent'"
    >
      <nav class="mx-auto flex h-16 max-w-6xl items-center justify-between px-5 sm:px-6">
        <router-link to="/home" class="flex min-w-0 items-center gap-2.5">
          <span class="h-9 w-9 shrink-0 overflow-hidden rounded-xl bg-slate-900/5 p-1 ring-1 ring-slate-900/10 dark:bg-white/10 dark:ring-white/15">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </span>
          <span class="truncate text-sm font-semibold tracking-wide text-slate-900 dark:text-white/90 sm:text-base">{{ siteName }}</span>
        </router-link>

        <div class="star-nav flex items-center gap-1 sm:gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="star-nav-btn"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="star-nav-btn gap-1.5" :title="t('nav.modelPlaza')">
            <Icon name="grid" size="md" />
            <span class="hidden text-sm sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="star-nav-btn"
            :title="landingDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleLandingTheme"
          >
            <Icon v-if="landingDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="star-login group ml-2 !py-1.5 !pl-1.5"
          >
            <span class="flex h-7 w-7 items-center justify-center rounded-full bg-gradient-to-br from-amber-400 to-fuchsia-500 text-xs font-semibold text-white">
              {{ userInitial }}
            </span>
            <span>{{ t('home.dashboard') }}</span>
            <Icon name="arrowRight" size="xs" :stroke-width="2.2" class="star-login__arrow" />
          </router-link>
          <router-link v-else to="/login" class="star-login group ml-2">
            <span>{{ t('home.login') }}</span>
            <Icon name="arrowRight" size="xs" :stroke-width="2.2" class="star-login__arrow" />
          </router-link>
        </div>
      </nav>
    </header>

    <main class="relative z-10">
      <!-- 1. Hero：左文右粒子 ST -->
      <section data-particle-section="st" class="relative flex min-h-[100svh] items-center px-5 pb-24 pt-24 sm:px-6">
        <div class="mx-auto grid w-full max-w-6xl items-center gap-8 lg:grid-cols-[1fr_1fr] lg:gap-6">
          <!-- 左：文案 -->
          <div class="order-2 text-left lg:order-1">
            <h1 class="star-hero-title star-hero-in" style="--d: 80ms">{{ t('home.star.heroTitle') }}</h1>

            <p class="star-hero-in mt-7 h-7 font-mono text-sm text-amber-700 dark:text-amber-200/90 md:text-base" style="--d: 240ms" aria-live="polite">
              <span class="text-fuchsia-600 dark:text-fuchsia-300">✦</span> {{ typedText }}<span class="star-caret"></span>
            </p>

            <p class="star-hero-in mt-3 max-w-xl text-sm leading-7 text-slate-600 dark:text-slate-400 md:text-base" style="--d: 420ms">
              {{ t('home.star.heroDesc') }}
            </p>

            <div class="star-hero-in mt-9 flex flex-wrap items-center gap-5" style="--d: 520ms">
              <MagneticButton :href="ctaHref" size="lg" @hover="onCtaHover" @launch="launch">
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="sm" :stroke-width="2.2" />
              </MagneticButton>
              <MagneticButton href="#connect" variant="ghost" size="lg" @launch="scrollToSection('connect')">
                <Icon name="terminal" size="sm" />
                {{ t('home.star.secondary') }}
              </MagneticButton>
            </div>
          </div>

          <!-- 右：粒子 ST（由 ParticleField 绘制） -->
          <div class="order-1 lg:order-2">
            <div data-particle-anchor class="mx-auto aspect-[6/5] w-full max-w-[340px] sm:max-w-[480px] lg:max-w-[640px]"></div>
          </div>
        </div>

        <button
          type="button"
          class="absolute bottom-8 left-1/2 hidden -translate-x-1/2 flex-col items-center gap-2 text-[11px] tracking-[0.3em] text-slate-500 transition-colors hover:text-slate-800 dark:hover:text-slate-300 sm:flex"
          @click="scrollToSection('orbit')"
        >
          <span class="star-scroll-mouse"><span></span></span>
          {{ t('home.star.scroll') }}
        </button>
      </section>

      <!-- 2. 章节 01：星图路由 -->
      <section id="orbit" data-particle-section="orbit" class="star-chapter-section">
        <div class="mx-auto grid w-full max-w-6xl items-center gap-10 lg:grid-cols-[1fr_1fr]">
          <div v-reveal class="order-2 min-w-0 lg:order-1">
            <div class="star-chapter"><span>01 / 03</span><i></i>{{ t('home.star.orbit.eyebrow') }}</div>
            <h2 class="star-h2">{{ t('home.star.orbit.title') }}</h2>
            <p class="mt-4 max-w-xl text-slate-600 dark:text-slate-400">{{ t('home.star.orbit.desc') }}</p>
            <p class="mt-3 hidden text-xs tracking-wide text-amber-700/80 dark:text-amber-200/70 lg:block">✦ {{ t('home.star.orbit.hint') }}</p>
            <ul class="mt-8 grid gap-3 sm:grid-cols-2">
              <li
                v-for="p in orbitProviders"
                :key="p.key"
                class="flex gap-3 rounded-2xl border border-slate-900/[0.08] bg-white/60 px-4 py-3 backdrop-blur-sm transition-colors hover:border-slate-900/20 dark:border-white/[0.07] dark:bg-white/[0.03] dark:hover:border-white/20"
              >
                <span class="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full" :style="{ background: p.color, boxShadow: `0 0 10px ${p.color}` }"></span>
                <div class="min-w-0">
                  <div class="text-sm font-semibold text-slate-900 dark:text-white">{{ p.name }}</div>
                  <div class="mt-0.5 text-xs leading-relaxed text-slate-600 dark:text-slate-400">{{ p.desc }}</div>
                </div>
              </li>
            </ul>
          </div>
          <div class="order-1 lg:order-2">
            <div data-particle-anchor class="star-anchor"></div>
          </div>
        </div>
      </section>

      <!-- 3. 能力卡片（光尘散成满屏星尘） -->
      <section data-particle-section="scatter" class="relative px-5 py-24">
        <div class="mx-auto max-w-6xl">
          <div v-reveal class="mx-auto max-w-3xl text-center">
            <span class="star-eyebrow">{{ t('home.star.features.eyebrow') }}</span>
            <h2 class="star-h2">{{ t('home.star.features.title') }}</h2>
          </div>
          <div class="mt-14 grid gap-5 md:grid-cols-2 lg:grid-cols-3">
            <div
              v-for="(f, i) in featureCards"
              :key="f.key"
              v-reveal
              :class="i === 0 ? 'md:col-span-2' : ''"
              :style="{ '--reveal-delay': `${i * 80}ms` }"
            >
              <SpotlightCard class="h-full">
                <div class="flex h-full flex-col">
                  <div
                    class="mb-5 flex h-12 w-12 items-center justify-center rounded-2xl transition-transform duration-300 group-hover:-translate-y-1 group-hover:scale-110"
                    :style="{ background: f.gradient, boxShadow: `0 10px 30px -8px ${f.glow}` }"
                  >
                    <Icon :name="f.icon" size="lg" class="text-white" />
                  </div>
                  <h3 class="mb-2 text-lg font-semibold text-slate-900 dark:text-white">{{ f.title }}</h3>
                  <p class="text-sm leading-relaxed text-slate-600 dark:text-slate-400">{{ f.desc }}</p>
                  <div v-if="i === 0" class="mt-6 flex flex-wrap gap-2">
                    <span v-for="p in orbitProviders" :key="p.key" class="star-chip !py-1 text-xs">
                      <span class="h-2 w-2 rounded-full" :style="{ background: p.color, boxShadow: `0 0 8px ${p.color}` }"></span>
                      {{ p.name }}
                    </span>
                  </div>
                </div>
              </SpotlightCard>
            </div>
          </div>
        </div>
      </section>

      <!-- 4. 章节 02：清晰计费 -->
      <section data-particle-section="bars" class="star-chapter-section">
        <div class="mx-auto grid w-full max-w-6xl items-center gap-10 lg:grid-cols-[1fr_1fr]">
          <div v-reveal class="order-2 min-w-0 lg:order-1">
            <div class="star-chapter"><span>02 / 03</span><i></i>{{ t('home.star.billing.eyebrow') }}</div>
            <h2 class="star-h2">{{ t('home.star.billing.title') }}</h2>
            <p class="mt-4 max-w-xl text-slate-600 dark:text-slate-400">{{ t('home.star.billing.desc') }}</p>
            <ul class="mt-8 space-y-3">
              <li v-for="pt in billingPoints" :key="pt" class="flex items-center gap-3 text-sm text-slate-700 dark:text-slate-200">
                <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-amber-500/10 text-amber-700 ring-1 ring-amber-600/30 dark:bg-amber-300/10 dark:text-amber-200 dark:ring-amber-200/30">
                  <Icon name="check" size="xs" :stroke-width="2.5" />
                </span>
                {{ pt }}
              </li>
            </ul>
          </div>
          <div class="order-1 lg:order-2">
            <div data-particle-anchor class="star-anchor"></div>
          </div>
        </div>
      </section>

      <!-- 5. 章节 03：极速接入 -->
      <section id="connect" data-particle-section="cursor" class="star-chapter-section">
        <div class="mx-auto grid w-full max-w-6xl items-center gap-10 lg:grid-cols-[1.15fr_1fr]">
          <div v-reveal class="order-2 min-w-0 lg:order-1">
            <div class="star-chapter"><span>03 / 03</span><i></i>{{ t('home.star.connect.eyebrow') }}</div>
            <h2 class="star-h2">{{ t('home.star.connect.title') }}</h2>
            <p class="mt-4 max-w-xl text-slate-600 dark:text-slate-400">{{ t('home.star.connect.desc') }}</p>
            <div class="mt-6 inline-flex max-w-full items-center gap-2 rounded-xl border border-slate-900/10 bg-white/70 px-4 py-2.5 font-mono text-sm dark:border-white/10 dark:bg-white/[0.04]">
              <span class="shrink-0 text-slate-500">Base URL</span>
              <span class="truncate text-amber-700 dark:text-amber-200">{{ baseUrl }}</span>
            </div>
            <div class="mt-6">
              <CodeTerminal
                :tabs="terminalTabs"
                :copy-label="t('home.star.connect.copy')"
                :copied-label="t('home.star.connect.copied')"
                :reduced-motion="reducedMotion"
              />
            </div>
          </div>
          <div class="order-1 lg:order-2">
            <div data-particle-anchor class="star-anchor"></div>
          </div>
        </div>
      </section>

      <!-- 6. 收尾 CTA：跃迁 -->
      <section data-particle-section="star" class="star-chapter-section pb-32">
        <div class="mx-auto grid w-full max-w-6xl items-center gap-10 lg:grid-cols-[1fr_1fr]">
          <div v-reveal class="order-2 min-w-0 lg:order-1">
            <h2 class="whitespace-pre-line text-4xl font-black leading-tight tracking-tight text-slate-900 dark:text-white md:text-6xl">{{ t('home.star.cta.title') }}</h2>
            <p class="mt-5 max-w-lg text-lg text-slate-600 dark:text-slate-300">{{ t('home.star.cta.desc') }}</p>
            <div class="mt-10">
              <MagneticButton :href="ctaHref" size="lg" @hover="onCtaHover" @launch="launch">
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="sm" :stroke-width="2.2" />
              </MagneticButton>
            </div>
          </div>
          <div class="order-1 lg:order-2">
            <div data-particle-anchor class="star-anchor"></div>
          </div>
        </div>
      </section>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-slate-900/[0.06] px-6 py-8 dark:border-white/[0.06]">
      <div class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 text-center sm:flex-row sm:text-left">
        <p class="text-sm text-slate-500">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-5">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-slate-500 transition-colors hover:text-slate-900 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="text-sm text-slate-500 transition-colors hover:text-slate-900 dark:hover:text-white">
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, type Directive } from 'vue'
import { useRouter } from 'vue-router'
import { usePreferredReducedMotion, useWindowScroll, useWindowSize } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import StarfieldCanvas from '@/components/home/StarfieldCanvas.vue'
import MagneticButton from '@/components/home/MagneticButton.vue'
import SpotlightCard from '@/components/home/SpotlightCard.vue'
import CodeTerminal, { type TerminalTab } from '@/components/home/CodeTerminal.vue'
import ParticleField from '@/components/home/ParticleField.vue'
import { useLandingTheme } from '@/composables/useLandingTheme'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})

// ==================== 星空落地页 ====================
const router = useRouter()
const { landingDark, toggleLandingTheme } = useLandingTheme()
const reducedMotionPref = usePreferredReducedMotion()
const reducedMotion = computed(() => reducedMotionPref.value === 'reduce')

const { y: scrollY } = useWindowScroll()
const { height: winHeight } = useWindowSize()
const heroZoom = computed(() => Math.min(1, scrollY.value / Math.max(1, winHeight.value)))
const headerSolid = computed(() => scrollY.value > 24)

// 跃迁：悬停主按钮时星星缓缓向外漂移，点击后全速跃迁再跳转
const warpLevel = ref(0)
const particleBurst = ref(0)
const ctaTarget = computed(() => (isAuthenticated.value ? dashboardPath.value : '/login'))
const ctaHref = computed(() => router.resolve(ctaTarget.value).href)
let launchTimer: ReturnType<typeof setTimeout> | null = null

function onCtaHover(hovering: boolean) {
  if (warpLevel.value >= 1 || reducedMotion.value) return
  warpLevel.value = hovering ? 0.12 : 0
}

function launch() {
  if (reducedMotion.value) {
    router.push(ctaTarget.value)
    return
  }
  warpLevel.value = 1
  particleBurst.value++
  launchTimer = setTimeout(() => router.push(ctaTarget.value), 780)
}

function scrollToSection(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: reducedMotion.value ? 'auto' : 'smooth' })
}

// Hero 打字机副标题
const typedPhrases = computed(() => [t('home.star.typed1'), t('home.star.typed2'), t('home.star.typed3')])
const typedText = ref('')
let typeTimer: ReturnType<typeof setTimeout> | null = null
let phraseIndex = 0
let charIndex = 0
let deleting = false

function typeTick() {
  const phrase = typedPhrases.value[phraseIndex % typedPhrases.value.length]
  if (reducedMotion.value) {
    typedText.value = phrase
    return
  }
  if (!deleting) {
    charIndex++
    typedText.value = phrase.slice(0, charIndex)
    if (charIndex >= phrase.length) {
      deleting = true
      typeTimer = setTimeout(typeTick, 2200)
      return
    }
    typeTimer = setTimeout(typeTick, 70)
  } else {
    charIndex--
    typedText.value = phrase.slice(0, charIndex)
    if (charIndex <= 0) {
      deleting = false
      phraseIndex++
      typeTimer = setTimeout(typeTick, 400)
      return
    }
    typeTimer = setTimeout(typeTick, 28)
  }
}

const orbitProviders = computed(() => [
  { key: 'claude', name: t('home.providers.claude'), letter: 'C', color: '#f97316', desc: t('home.star.orbit.claudeDesc') },
  { key: 'gpt', name: 'GPT', letter: 'G', color: '#10b981', desc: t('home.star.orbit.gptDesc') },
  { key: 'gemini', name: t('home.providers.gemini'), letter: 'G', color: '#3b82f6', desc: t('home.star.orbit.geminiDesc') },
  { key: 'antigravity', name: t('home.providers.antigravity'), letter: 'A', color: '#ec4899', desc: t('home.star.orbit.antigravityDesc') }
])

const featureCards = computed(() => [
  {
    key: 'gateway',
    icon: 'key' as const,
    title: t('home.star.features.gateway'),
    desc: t('home.star.features.gatewayDesc'),
    gradient: 'linear-gradient(135deg,#f59e0b,#d946ef)',
    glow: 'rgba(217,70,239,0.6)'
  },
  {
    key: 'pool',
    icon: 'server' as const,
    title: t('home.star.features.pool'),
    desc: t('home.star.features.poolDesc'),
    gradient: 'linear-gradient(135deg,#6366f1,#22d3ee)',
    glow: 'rgba(99,102,241,0.6)'
  },
  {
    key: 'billing',
    icon: 'chart' as const,
    title: t('home.star.features.billing'),
    desc: t('home.star.features.billingDesc'),
    gradient: 'linear-gradient(135deg,#10b981,#84cc16)',
    glow: 'rgba(16,185,129,0.55)'
  },
  {
    key: 'sticky',
    icon: 'shield' as const,
    title: t('home.star.features.sticky'),
    desc: t('home.star.features.stickyDesc'),
    gradient: 'linear-gradient(135deg,#ec4899,#f97316)',
    glow: 'rgba(236,72,153,0.55)'
  },
  {
    key: 'compat',
    icon: 'globe' as const,
    title: t('home.star.features.compat'),
    desc: t('home.star.features.compatDesc'),
    gradient: 'linear-gradient(135deg,#3b82f6,#a855f7)',
    glow: 'rgba(59,130,246,0.55)'
  }
])

const billingPoints = computed(() => [t('home.star.billing.point1'), t('home.star.billing.point2'), t('home.star.billing.point3')])

const baseUrl = window.location.origin
const terminalTabs = computed<TerminalTab[]>(() => [
  {
    key: 'curl',
    label: 'cURL',
    lines: [
      { text: `$ curl ${baseUrl}/v1/messages \\` },
      { text: '    -H "x-api-key: $API_KEY" \\' },
      { text: '    -H "content-type: application/json" \\' },
      { text: `    -d '{"model":"claude-sonnet-4-5","max_tokens":256,` },
      { text: `         "messages":[{"role":"user","content":"Hello, stars"}]}'` },
      { text: '' },
      { text: '200 OK  ✦ routed to the healthiest upstream', kind: 'out' }
    ]
  },
  {
    key: 'python',
    label: 'Python',
    lines: [
      { text: 'from openai import OpenAI' },
      { text: '' },
      { text: `client = OpenAI(base_url="${baseUrl}/v1", api_key="sk-...")` },
      { text: 'resp = client.chat.completions.create(' },
      { text: '    model="gpt-5",' },
      { text: '    messages=[{"role": "user", "content": "Hello, stars"}],' },
      { text: ')' },
      { text: 'print(resp.choices[0].message.content)' }
    ]
  },
  {
    key: 'claude-code',
    label: 'Claude Code',
    lines: [
      { text: t('home.star.connect.ccComment') },
      { text: `$ export ANTHROPIC_BASE_URL=${baseUrl}` },
      { text: '$ export ANTHROPIC_AUTH_TOKEN=sk-...' },
      { text: '$ claude' },
      { text: '' },
      { text: `✦ Welcome to Claude Code — powered by ${siteName.value}`, kind: 'out' }
    ]
  }
])

// 滚动进入视口时淡入上浮
const revealObservers = new WeakMap<HTMLElement, IntersectionObserver>()
const vReveal: Directive<HTMLElement> = {
  mounted(el) {
    el.classList.add('star-reveal')
    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          el.classList.add('is-visible')
          io.disconnect()
        }
      },
      { threshold: 0.12 }
    )
    io.observe(el)
    revealObservers.set(el, io)
  },
  unmounted(el) {
    revealObservers.get(el)?.disconnect()
  }
}

onMounted(() => {
  typeTimer = setTimeout(typeTick, 900)
})

onBeforeUnmount(() => {
  if (typeTimer) clearTimeout(typeTimer)
  if (launchTimer) clearTimeout(launchTimer)
})
</script>

<style scoped>
/* ==================== 星空落地页 ==================== */
/* 主题变量：默认亮色，.dark 覆盖为深色。SpotlightCard / MagneticButton 也读取这些变量 */
.star-home {
  --sh-bg: #f8f6f1;
  --sh-fg: #0f172a;
  --sh-nav: rgba(51, 65, 85, 0.85);
  --sh-nav-hover-bg: rgba(15, 23, 42, 0.06);
  --sh-line: rgba(15, 23, 42, 0.16);
  --sh-line-soft: rgba(15, 23, 42, 0.1);
  --sh-surface: rgba(255, 255, 255, 0.65);
  --sh-surface-hover: rgba(255, 255, 255, 0.95);
  --sh-accent: #b45309;
  --sh-caret: #d97706;
  --sh-chip-fg: #334155;
  --sh-chapter: rgba(71, 85, 105, 0.9);
  --sh-title-grad: linear-gradient(180deg, #0f172a 45%, #6d28d9 100%);
  --sh-h2-grad: linear-gradient(180deg, #0f172a 40%, #7c3aed);
  --sh-login-hover-border: rgba(180, 83, 9, 0.45);
  --sh-login-glow: rgba(217, 119, 6, 0.22);
  --sh-card-bg: linear-gradient(160deg, rgba(255, 255, 255, 0.88), rgba(255, 255, 255, 0.62));
  --sh-card-border: rgba(15, 23, 42, 0.08);
  --sh-card-shadow: 0 18px 40px -26px rgba(30, 27, 75, 0.3);
  --sh-spot: rgba(124, 58, 237, 0.1);
  --sh-ghost-fg: #1e293b;
  --sh-ghost-bg: rgba(255, 255, 255, 0.6);
  --sh-ghost-border: rgba(15, 23, 42, 0.15);
  --sh-ghost-bg-hover: #ffffff;
  --sh-ghost-border-hover: rgba(180, 83, 9, 0.45);
  background: var(--sh-bg);
  color: var(--sh-fg);
  color-scheme: light;
}

.star-home.dark {
  --sh-bg: #04050d;
  --sh-fg: #ffffff;
  --sh-nav: rgba(203, 213, 225, 0.8);
  --sh-nav-hover-bg: rgba(255, 255, 255, 0.08);
  --sh-line: rgba(255, 255, 255, 0.18);
  --sh-line-soft: rgba(255, 255, 255, 0.1);
  --sh-surface: rgba(255, 255, 255, 0.04);
  --sh-surface-hover: rgba(255, 255, 255, 0.08);
  --sh-accent: #fcd34d;
  --sh-caret: #fde68a;
  --sh-chip-fg: rgba(226, 232, 240, 0.9);
  --sh-chapter: rgba(203, 213, 225, 0.75);
  --sh-title-grad: linear-gradient(180deg, #ffffff 45%, #c4b5fd 100%);
  --sh-h2-grad: linear-gradient(180deg, #ffffff 40%, rgba(196, 181, 253, 0.85));
  --sh-login-hover-border: rgba(253, 230, 138, 0.55);
  --sh-login-glow: rgba(253, 230, 138, 0.45);
  --sh-card-bg: linear-gradient(160deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0.015));
  --sh-card-border: rgba(255, 255, 255, 0.08);
  --sh-card-shadow: none;
  --sh-spot: rgba(167, 139, 250, 0.16);
  --sh-ghost-fg: rgba(226, 232, 240, 0.95);
  --sh-ghost-bg: rgba(255, 255, 255, 0.04);
  --sh-ghost-border: rgba(255, 255, 255, 0.14);
  --sh-ghost-bg-hover: rgba(255, 255, 255, 0.09);
  --sh-ghost-border-hover: rgba(253, 230, 138, 0.45);
  color-scheme: dark;
}

.star-nebula {
  /* 亮色：晨光般的暖白底与淡彩晕染 */
  background:
    radial-gradient(ellipse 60% 45% at 15% 10%, rgba(251, 191, 36, 0.2), transparent 70%),
    radial-gradient(ellipse 55% 50% at 85% 15%, rgba(167, 139, 250, 0.22), transparent 70%),
    radial-gradient(ellipse 70% 50% at 50% 100%, rgba(244, 114, 182, 0.14), transparent 70%),
    radial-gradient(ellipse 40% 35% at 70% 55%, rgba(56, 189, 248, 0.1), transparent 70%),
    linear-gradient(180deg, #fbf8f2 0%, #f6f3ff 50%, #f8fafc 100%);
  animation: nebula-drift 26s ease-in-out infinite alternate;
}

.star-home.dark .star-nebula {
  background:
    radial-gradient(ellipse 60% 45% at 15% 10%, rgba(245, 158, 11, 0.13), transparent 70%),
    radial-gradient(ellipse 55% 50% at 85% 15%, rgba(99, 102, 241, 0.22), transparent 70%),
    radial-gradient(ellipse 70% 50% at 50% 100%, rgba(217, 70, 239, 0.16), transparent 70%),
    radial-gradient(ellipse 40% 35% at 70% 55%, rgba(34, 211, 238, 0.07), transparent 70%),
    linear-gradient(180deg, #04050d 0%, #070818 50%, #05060f 100%);
}

@keyframes nebula-drift {
  0% {
    transform: scale(1) translate3d(0, 0, 0);
    filter: hue-rotate(0deg);
  }
  100% {
    transform: scale(1.08) translate3d(-1.5%, 1%, 0);
    filter: hue-rotate(-18deg);
  }
}

/* 跃迁结束的白光闪 */
.star-warp-flash {
  background: radial-gradient(circle at 50% 50%, rgba(255, 255, 255, 0.95), rgba(253, 230, 138, 0.4) 30%, transparent 70%);
  opacity: 0;
  transition: opacity 0.3s ease;
}

.star-warp-flash.is-on {
  animation: warp-flash 0.8s ease-in forwards;
}

@keyframes warp-flash {
  0%,
  55% {
    opacity: 0;
  }
  100% {
    opacity: 1;
  }
}

/* Header 按钮 */
.star-nav-btn {
  display: inline-flex;
  align-items: center;
  border-radius: 0.6rem;
  padding: 0.5rem;
  color: var(--sh-nav);
  transition:
    color 0.2s ease,
    background-color 0.2s ease;
}

.star-nav-btn:hover {
  color: var(--sh-fg);
  background: var(--sh-nav-hover-bg);
}

.star-nav :deep(button) {
  color: var(--sh-nav);
}

/* 顶栏登录 / 控制台按钮 */
.star-login {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  border-radius: 9999px;
  border: 1px solid var(--sh-line);
  background: var(--sh-surface);
  padding: 0.5rem 0.95rem 0.5rem 1.15rem;
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--sh-fg);
  backdrop-filter: blur(10px);
  transition:
    border-color 0.25s ease,
    background-color 0.25s ease,
    box-shadow 0.25s ease;
}

.star-login:hover {
  border-color: var(--sh-login-hover-border);
  background: var(--sh-surface-hover);
  box-shadow: 0 0 24px -6px var(--sh-login-glow);
}

.star-login__arrow {
  transform: rotate(-45deg);
  opacity: 0.75;
  transition:
    transform 0.25s ease,
    opacity 0.25s ease;
}

.star-login:hover .star-login__arrow {
  transform: rotate(-45deg) translateX(2px);
  opacity: 1;
}

/* Hero */
.star-hero-title {
  white-space: pre-line;
  font-size: clamp(2.3rem, 4vw, 3.6rem);
  font-weight: 800;
  line-height: 1.12;
  letter-spacing: -0.025em;
  background: var(--sh-title-grad);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.star-hero-in {
  animation: hero-in 1s cubic-bezier(0.2, 0.8, 0.2, 1) both;
  animation-delay: var(--d, 0ms);
}

@keyframes hero-in {
  from {
    opacity: 0;
    transform: translateY(24px);
    filter: blur(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
    filter: blur(0);
  }
}

.star-caret {
  display: inline-block;
  width: 2px;
  height: 1.1em;
  margin-left: 3px;
  vertical-align: -0.18em;
  background: var(--sh-caret);
  animation: caret-blink 1s step-end infinite;
}

@keyframes caret-blink {
  50% {
    opacity: 0;
  }
}

.star-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  border-radius: 9999px;
  border: 1px solid var(--sh-line-soft);
  background: var(--sh-surface);
  padding: 0.5rem 1rem;
  font-size: 0.85rem;
  color: var(--sh-chip-fg);
  backdrop-filter: blur(8px);
}

.star-scroll-mouse {
  position: relative;
  display: block;
  width: 22px;
  height: 34px;
  border-radius: 9999px;
  border: 1.5px solid var(--sh-line);
}

.star-scroll-mouse span {
  position: absolute;
  left: 50%;
  top: 6px;
  width: 3px;
  height: 7px;
  margin-left: -1.5px;
  border-radius: 9999px;
  background: var(--sh-caret);
  animation: mouse-wheel 1.8s ease-in-out infinite;
}

@keyframes mouse-wheel {
  0% {
    opacity: 0;
    transform: translateY(-2px);
  }
  30% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translateY(12px);
  }
}

/* 区块标题 */
.star-eyebrow {
  display: inline-block;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.3em;
  text-transform: uppercase;
  color: var(--sh-accent);
}

.star-h2 {
  white-space: pre-line;
  margin-top: 0.9rem;
  font-size: clamp(1.9rem, 4.2vw, 3.2rem);
  font-weight: 900;
  line-height: 1.15;
  letter-spacing: -0.02em;
  background: var(--sh-h2-grad);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

/* 滚动淡入 */
.star-reveal {
  opacity: 0;
  transform: translateY(36px);
  transition:
    opacity 0.9s cubic-bezier(0.2, 0.8, 0.2, 1),
    transform 0.9s cubic-bezier(0.2, 0.8, 0.2, 1);
  transition-delay: var(--reveal-delay, 0ms);
}

.star-reveal.is-visible {
  opacity: 1;
  transform: none;
}

.star-chapter-section {
  position: relative;
  display: flex;
  min-height: 92svh;
  align-items: center;
  scroll-margin-top: 4rem;
  padding: 6rem 1.25rem;
}

.star-chapter {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.22em;
  color: var(--sh-chapter);
}

.star-chapter span {
  color: var(--sh-accent);
  font-variant-numeric: tabular-nums;
}

.star-chapter i {
  width: 1px;
  height: 0.9rem;
  background: var(--sh-line);
}

.star-anchor {
  margin: 0 auto;
  aspect-ratio: 6 / 5;
  width: 100%;
  max-width: 320px;
}

@media (min-width: 640px) {
  .star-anchor {
    max-width: 440px;
  }
}

@media (min-width: 1024px) {
  .star-anchor {
    max-width: 560px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .star-nebula,
  .star-hero-in,
  .star-scroll-mouse span {
    animation: none !important;
  }

  .star-reveal {
    opacity: 1;
    transform: none;
    transition: none;
  }
}
</style>
