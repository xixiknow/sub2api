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
import { getQQBotConfig, updateQQBotConfig } from '@/api/qqbot'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const form = reactive({ enabled: false, app_id: '', app_secret: '' })
const secretConfigured = ref(false)
const loading = ref(true)
const saving = ref(false)
const copied = ref(false)

const callbackUrl = computed(() => `${window.location.origin}/api/v1/qqbot/webhook`)

function errorMessage(error: unknown, fallback: string): string {
  const message = (error as { message?: string }).message
  return typeof message === 'string' && message.length > 0 ? message : fallback
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const cfg = await getQQBotConfig()
    form.enabled = cfg.enabled
    form.app_id = cfg.app_id
    secretConfigured.value = cfg.secret_configured
  } catch (error) {
    appStore.showError(errorMessage(error, t('qqbot.admin.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  saving.value = true
  try {
    const cfg = await updateQQBotConfig({ enabled: form.enabled, app_id: form.app_id, app_secret: form.app_secret || undefined })
    form.enabled = cfg.enabled
    form.app_id = cfg.app_id
    form.app_secret = ''
    secretConfigured.value = cfg.secret_configured
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
