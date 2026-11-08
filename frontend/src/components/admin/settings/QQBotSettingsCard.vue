<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('qqbot.admin.title') }}</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('qqbot.admin.description') }}</p>
    </div>
    <div class="space-y-5 p-6">
      <div class="flex items-center justify-between">
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('qqbot.admin.enabled') }}</label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('qqbot.admin.enabledHint') }}</p>
        </div>
        <Toggle v-model="form.enabled" />
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label for="qqbot-app-id" class="input-label">{{ t('qqbot.admin.appId') }}</label>
          <input id="qqbot-app-id" v-model.trim="form.app_id" class="input" maxlength="64" autocomplete="off" />
        </div>
        <div>
          <label for="qqbot-app-secret" class="input-label">{{ t('qqbot.admin.appSecret') }}</label>
          <input
            id="qqbot-app-secret"
            v-model.trim="form.app_secret"
            type="password"
            class="input"
            autocomplete="new-password"
            :placeholder="secretConfigured ? t('qqbot.admin.secretConfigured') : t('qqbot.admin.secretPlaceholder')"
          />
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('qqbot.admin.callbackUrl') }}</label>
        <div class="flex gap-2">
          <input class="input font-mono text-xs" :value="callbackUrl" readonly />
          <button type="button" class="btn btn-secondary shrink-0" @click="copyCallback">
            {{ copied ? t('qqbot.admin.copied') : t('qqbot.admin.copy') }}
          </button>
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('qqbot.admin.callbackHint') }}</p>
      </div>

      <p class="text-xs text-amber-600 dark:text-amber-400">{{ t('qqbot.admin.note') }}</p>

      <div class="space-y-4 border-t border-gray-100 pt-5 dark:border-dark-700">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('qqbot.admin.poster.title') }}</h3>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('qqbot.admin.poster.description') }}</p>
        </div>
        <div class="grid gap-4 sm:grid-cols-3">
          <div class="sm:col-span-3">
            <label for="qqbot-poster-key" class="input-label">{{ t('qqbot.admin.poster.apiKey') }}</label>
            <input
              id="qqbot-poster-key"
              v-model.trim="form.poster_api_key"
              type="password"
              class="input"
              autocomplete="new-password"
              :placeholder="posterKeyConfigured ? t('qqbot.admin.secretConfigured') : t('qqbot.admin.poster.apiKeyPlaceholder')"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('qqbot.admin.poster.apiKeyHint') }}</p>
          </div>
          <div class="sm:col-span-2">
            <label for="qqbot-poster-model" class="input-label">{{ t('qqbot.admin.poster.model') }}</label>
            <input id="qqbot-poster-model" v-model.trim="form.poster_model" class="input" maxlength="128" placeholder="gpt-image-2" />
          </div>
          <div>
            <label for="qqbot-poster-limit" class="input-label">{{ t('qqbot.admin.poster.dailyLimit') }}</label>
            <input id="qqbot-poster-limit" v-model.number="form.poster_daily_limit" type="number" min="0" max="50" class="input" />
          </div>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('qqbot.admin.poster.hint') }}</p>
      </div>

      <div class="flex justify-end">
        <button type="button" class="btn btn-primary" :disabled="saving || loading" @click="save">
          {{ saving ? t('common.processing') : t('qqbot.admin.save') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { getQQBotConfig, updateQQBotConfig, type QQBotConfig } from '@/api/qqbot'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const form = reactive({
  enabled: false,
  app_id: '',
  app_secret: '',
  poster_api_key: '',
  poster_model: 'gpt-image-2',
  poster_daily_limit: 3,
})
const secretConfigured = ref(false)
const posterKeyConfigured = ref(false)
const loading = ref(true)
const saving = ref(false)
const copied = ref(false)

const callbackUrl = computed(() => `${window.location.origin}/api/v1/qqbot/webhook`)

function errorMessage(error: unknown, fallback: string): string {
  const message = (error as { message?: string }).message
  return typeof message === 'string' && message.length > 0 ? message : fallback
}

function applyConfig(cfg: QQBotConfig): void {
  form.enabled = cfg.enabled
  form.app_id = cfg.app_id
  form.app_secret = ''
  form.poster_api_key = ''
  form.poster_model = cfg.poster_model || 'gpt-image-2'
  form.poster_daily_limit = cfg.poster_daily_limit
  secretConfigured.value = cfg.secret_configured
  posterKeyConfigured.value = cfg.poster_key_configured
}

async function load(): Promise<void> {
  loading.value = true
  try {
    applyConfig(await getQQBotConfig())
  } catch (error) {
    appStore.showError(errorMessage(error, t('qqbot.admin.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  saving.value = true
  try {
    const limit = Number.isFinite(form.poster_daily_limit) ? Math.min(50, Math.max(0, Math.trunc(form.poster_daily_limit))) : 3
    applyConfig(
      await updateQQBotConfig({
        enabled: form.enabled,
        app_id: form.app_id,
        app_secret: form.app_secret || undefined,
        poster_api_key: form.poster_api_key || undefined,
        poster_model: form.poster_model,
        poster_daily_limit: limit,
      }),
    )
    appStore.showSuccess(t('qqbot.admin.saved'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('qqbot.admin.saveFailed')))
    await load()
  } finally {
    saving.value = false
  }
}

async function copyCallback(): Promise<void> {
  try {
    await navigator.clipboard.writeText(callbackUrl.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    // clipboard may be unavailable over plain http; the field stays selectable
  }
}

onMounted(load)
</script>
