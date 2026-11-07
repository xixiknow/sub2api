<template>
  <!-- 认证页跟随落地页主题（默认深色）：根节点按需带 dark，内部表单控件随之切换 -->
  <div class="auth-star relative min-h-screen overflow-hidden" :class="{ dark: landingDark }">
    <div class="auth-nebula pointer-events-none fixed inset-0" aria-hidden="true"></div>
    <StarfieldCanvas class="fixed inset-0 z-0" :links="false" :light="!landingDark" :reduced-motion="reducedMotion" />
    <ParticleField :light="!landingDark" :reduced-motion="reducedMotion" />

    <!-- 顶栏 -->
    <header class="relative z-20">
      <nav class="mx-auto flex h-16 max-w-7xl items-center justify-between px-5 sm:px-8">
        <router-link to="/home" class="flex min-w-0 items-center gap-2.5">
          <span class="h-9 w-9 shrink-0 overflow-hidden rounded-xl bg-slate-900/5 p-1 ring-1 ring-slate-900/10 dark:bg-white/10 dark:ring-white/15">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </span>
          <span class="truncate text-sm font-semibold tracking-wide text-slate-900 dark:text-white/90 sm:text-base">{{ siteName }}</span>
        </router-link>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="auth-icon-btn"
            :title="landingDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleLandingTheme"
          >
            <Icon :name="landingDark ? 'sun' : 'moon'" size="md" />
          </button>
          <router-link to="/home" class="auth-back group">
            <Icon name="arrowLeft" size="xs" :stroke-width="2.2" class="transition-transform group-hover:-translate-x-0.5" />
            {{ t('home.star.auth.backHome') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="relative z-10 mx-auto grid min-h-[calc(100vh-4rem)] max-w-7xl items-center gap-10 px-5 pb-10 sm:px-8 lg:grid-cols-[1.1fr_1fr]">
      <!-- 左：品牌区（仅桌面），光尘 ST 由 ParticleField 绘制在锚点处 -->
      <section data-particle-section="st" class="hidden flex-col justify-center lg:flex">
        <h2 class="auth-title">{{ t('home.star.heroTitle') }}</h2>
        <p class="mt-5 max-w-md text-sm leading-7 text-slate-600 dark:text-slate-400">{{ t('home.star.heroDesc') }}</p>
        <div data-particle-anchor class="mt-6 aspect-[6/5] w-full max-w-[520px]"></div>
      </section>

      <!-- 右：表单卡片 -->
      <section class="mx-auto w-full max-w-md">
        <div class="auth-card">
          <slot />
        </div>

        <div class="mt-6 text-center text-sm">
          <slot name="footer" />
        </div>

        <div class="mt-8 text-center text-xs text-slate-500">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePreferredReducedMotion } from '@vueuse/core'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import Icon from '@/components/icons/Icon.vue'
import StarfieldCanvas from '@/components/home/StarfieldCanvas.vue'
import ParticleField from '@/components/home/ParticleField.vue'
import { useLandingTheme } from '@/composables/useLandingTheme'

const { t } = useI18n()
const appStore = useAppStore()
const { landingDark, toggleLandingTheme } = useLandingTheme()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const currentYear = computed(() => new Date().getFullYear())

const reducedMotionPref = usePreferredReducedMotion()
const reducedMotion = computed(() => reducedMotionPref.value === 'reduce')

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
/* 主题变量：默认亮色，.dark 覆盖为深色 */
.auth-star {
  --au-bg: #f8f6f1;
  --au-fg: #0f172a;
  --au-soft: #334155;
  --au-line: rgba(15, 23, 42, 0.14);
  --au-surface: rgba(255, 255, 255, 0.65);
  --au-surface-hover: #ffffff;
  --au-hover-border: rgba(180, 83, 9, 0.45);
  --au-title-grad: linear-gradient(180deg, #0f172a 45%, #6d28d9 100%);
  --au-card-bg: linear-gradient(160deg, rgba(255, 255, 255, 0.92), rgba(255, 255, 255, 0.78));
  --au-card-border: rgba(15, 23, 42, 0.08);
  --au-card-shadow: 0 30px 70px -36px rgba(30, 27, 75, 0.35);
  --au-card-line: linear-gradient(90deg, transparent, rgba(217, 119, 6, 0.7), rgba(124, 58, 237, 0.5), transparent);
  --au-input-bg: rgba(255, 255, 255, 0.85);
  --au-input-bg-focus: #ffffff;
  --au-input-border: rgba(15, 23, 42, 0.14);
  --au-input-fg: #0f172a;
  --au-placeholder: rgba(100, 116, 139, 0.75);
  --au-focus-border: rgba(217, 119, 6, 0.6);
  --au-focus-ring: rgba(217, 119, 6, 0.14);
  --au-label: #334155;
  --au-link: #b45309;
  --au-link-hover: #92400e;
  background: var(--au-bg);
  color: var(--au-fg);
  color-scheme: light;
}

.auth-star.dark {
  --au-bg: #04050d;
  --au-fg: #ffffff;
  --au-soft: rgba(226, 232, 240, 0.85);
  --au-line: rgba(255, 255, 255, 0.14);
  --au-surface: rgba(255, 255, 255, 0.03);
  --au-surface-hover: rgba(255, 255, 255, 0.08);
  --au-hover-border: rgba(253, 230, 138, 0.5);
  --au-title-grad: linear-gradient(180deg, #ffffff 45%, #c4b5fd 100%);
  --au-card-bg: linear-gradient(160deg, rgba(23, 23, 42, 0.78), rgba(10, 10, 24, 0.82));
  --au-card-border: rgba(255, 255, 255, 0.1);
  --au-card-shadow: 0 30px 80px -30px rgba(99, 102, 241, 0.45);
  --au-card-line: linear-gradient(90deg, transparent, rgba(253, 230, 138, 0.8), rgba(240, 171, 252, 0.6), transparent);
  --au-input-bg: rgba(255, 255, 255, 0.04);
  --au-input-bg-focus: rgba(255, 255, 255, 0.06);
  --au-input-border: rgba(255, 255, 255, 0.12);
  --au-input-fg: #f1f5f9;
  --au-placeholder: rgba(148, 163, 184, 0.6);
  --au-focus-border: rgba(253, 230, 138, 0.6);
  --au-focus-ring: rgba(253, 230, 138, 0.12);
  --au-label: rgba(203, 213, 225, 0.9);
  --au-link: #fcd34d;
  --au-link-hover: #fde68a;
  color-scheme: dark;
}

.auth-nebula {
  background:
    radial-gradient(ellipse 55% 45% at 12% 15%, rgba(251, 191, 36, 0.2), transparent 70%),
    radial-gradient(ellipse 50% 50% at 88% 20%, rgba(167, 139, 250, 0.22), transparent 70%),
    radial-gradient(ellipse 70% 50% at 50% 105%, rgba(244, 114, 182, 0.14), transparent 70%),
    linear-gradient(180deg, #fbf8f2 0%, #f6f3ff 55%, #f8fafc 100%);
}

.auth-star.dark .auth-nebula {
  background:
    radial-gradient(ellipse 55% 45% at 12% 15%, rgba(245, 158, 11, 0.12), transparent 70%),
    radial-gradient(ellipse 50% 50% at 88% 20%, rgba(99, 102, 241, 0.2), transparent 70%),
    radial-gradient(ellipse 70% 50% at 50% 105%, rgba(217, 70, 239, 0.15), transparent 70%),
    linear-gradient(180deg, #04050d 0%, #070818 55%, #05060f 100%);
}

.auth-back,
.auth-icon-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  border-radius: 9999px;
  border: 1px solid var(--au-line);
  background: var(--au-surface);
  color: var(--au-soft);
  backdrop-filter: blur(10px);
  transition:
    border-color 0.25s ease,
    background-color 0.25s ease,
    color 0.25s ease;
}

.auth-back {
  padding: 0.45rem 0.95rem;
  font-size: 0.8rem;
}

.auth-icon-btn {
  justify-content: center;
  height: 2.15rem;
  width: 2.15rem;
}

.auth-back:hover,
.auth-icon-btn:hover {
  border-color: var(--au-hover-border);
  background: var(--au-surface-hover);
  color: var(--au-fg);
}

.auth-title {
  white-space: pre-line;
  font-size: clamp(2.2rem, 3.4vw, 3.25rem);
  font-weight: 800;
  line-height: 1.15;
  letter-spacing: -0.02em;
  background: var(--au-title-grad);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

/* 玻璃卡片，顶部一道高光 */
.auth-card {
  position: relative;
  border-radius: 1.75rem;
  border: 1px solid var(--au-card-border);
  background: var(--au-card-bg);
  padding: 2.25rem 2rem;
  box-shadow: var(--au-card-shadow);
  backdrop-filter: blur(18px);
}

.auth-card::before {
  content: '';
  position: absolute;
  left: 12%;
  right: 12%;
  top: -1px;
  height: 1px;
  background: var(--au-card-line);
}

@media (min-width: 640px) {
  .auth-card {
    padding: 2.5rem 2.5rem;
  }
}

/* 表单控件统一到星空配色 */
.auth-star :deep(.input) {
  background: var(--au-input-bg);
  border-color: var(--au-input-border);
  color: var(--au-input-fg);
  border-radius: 0.9rem;
}

.auth-star :deep(.input::placeholder) {
  color: var(--au-placeholder);
}

.auth-star :deep(.input:focus) {
  border-color: var(--au-focus-border);
  box-shadow: 0 0 0 3px var(--au-focus-ring);
  background: var(--au-input-bg-focus);
}

.auth-star :deep(.input-label) {
  color: var(--au-label);
}

.auth-star :deep(.btn-primary) {
  border: 0;
  border-radius: 9999px;
  background: linear-gradient(120deg, #d97706 0%, #c026d3 50%, #4f46e5 100%);
  box-shadow: 0 10px 34px -12px rgba(217, 70, 239, 0.75);
  transition:
    box-shadow 0.3s ease,
    transform 0.2s ease,
    opacity 0.2s ease;
}

.auth-star :deep(.btn-primary:hover:not(:disabled)) {
  transform: translateY(-1px);
  box-shadow:
    0 0 40px -6px rgba(217, 70, 239, 0.8),
    0 0 80px -24px rgba(245, 158, 11, 0.7);
}

.auth-star :deep(.btn-secondary) {
  border-radius: 9999px;
  border: 1px solid var(--au-line);
  background: var(--au-surface);
  color: var(--au-soft);
}

.auth-star :deep(.btn-secondary:hover:not(:disabled)) {
  border-color: var(--au-hover-border);
  background: var(--au-surface-hover);
}

/* 主题色链接（忘记密码、注册等）改为琥珀 / 淡金色 */
.auth-star :deep(a.text-primary-600) {
  color: var(--au-link);
}

.auth-star :deep(a.text-primary-600:hover) {
  color: var(--au-link-hover);
}
</style>
