<template>
  <div v-if="binding && (binding.enabled || binding.bound)" class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">{{ t('qqbot.profile.title') }}</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('qqbot.profile.description') }}</p>
    </div>
    <div class="px-6 py-5">
      <div v-if="binding.bound" class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('qqbot.profile.bound') }}
            <span class="ml-1 font-mono text-xs text-gray-500 dark:text-gray-400">{{ binding.openid }}</span>
          </div>
          <div v-if="binding.created_at" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('qqbot.profile.boundAt', { time: formatDateTime(binding.created_at) }) }}
          </div>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="showConfirm = true">
          {{ t('qqbot.profile.unbind') }}
        </button>
      </div>
      <div v-else class="space-y-3 text-sm text-gray-600 dark:text-gray-300">
        <div class="text-gray-500 dark:text-gray-400">{{ t('qqbot.profile.notBound') }}</div>
        <template v-if="activeCode">
          <div>{{ t('qqbot.profile.howTo') }}</div>
          <div class="flex flex-wrap items-center gap-2">
            <code class="inline-block rounded bg-gray-100 px-2 py-1 font-mono text-sm tracking-wider dark:bg-dark-700">
              {{ command }}
            </code>
            <button type="button" class="btn btn-secondary btn-sm" @click="copyToClipboard(command)">
              {{ copied ? t('qqbot.profile.copied') : t('qqbot.profile.copy') }}
            </button>
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('qqbot.profile.codeExpires', { seconds: remaining }) }}
          </div>
        </template>
        <div v-else class="flex flex-wrap items-center gap-3">
          <button type="button" class="btn btn-primary" :disabled="busy" @click="generate">
            {{ codeInfo ? t('qqbot.profile.regenerate') : t('qqbot.profile.generate') }}
          </button>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('qqbot.profile.generateHint') }}</span>
        </div>
      </div>
    </div>
    <ConfirmDialog
      :show="showConfirm"
      :title="t('qqbot.profile.unbind')"
      :message="t('qqbot.profile.unbindConfirm')"
      danger
      @confirm="unbind"
      @cancel="showConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { createQQBindCode, getMyQQBinding, unbindMyQQ, type QQBotBinding, type QQBotBindCode } from '@/api/qqbot'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const { copied, copyToClipboard } = useClipboard()
const binding = ref<QQBotBinding | null>(null)
const busy = ref(false)
const showConfirm = ref(false)
const codeInfo = ref<QQBotBindCode | null>(null)
const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | undefined
let polls = 0

const remaining = computed(() => {
  if (!codeInfo.value) return 0
  return Math.max(0, Math.ceil((new Date(codeInfo.value.expires_at).getTime() - now.value) / 1000))
})
const activeCode = computed(() => remaining.value > 0)
const command = computed(() => `/绑定 ${codeInfo.value?.code ?? ''}`)

async function load(): Promise<void> {
  try {
    binding.value = await getMyQQBinding()
  } catch {
    // Card stays hidden: the bot is an optional feature.
    binding.value = null
  }
}

function stopTimer(): void {
  if (timer) clearInterval(timer)
  timer = undefined
}

function startTimer(): void {
  stopTimer()
  polls = 0
  timer = setInterval(() => {
    now.value = Date.now()
    if (!activeCode.value) {
      stopTimer()
      return
    }
    // Refresh the binding every 5s so the card flips to "bound" once the bot redeems the code.
    if (++polls % 5 === 0) {
      void load().then(() => {
        if (binding.value?.bound) {
          codeInfo.value = null
          stopTimer()
        }
      })
    }
  }, 1000)
}

async function generate(): Promise<void> {
  busy.value = true
  try {
    codeInfo.value = await createQQBindCode()
    now.value = Date.now()
    startTimer()
  } catch (err: unknown) {
    const msg = (err as { message?: string })?.message
    appStore.showError(msg || t('qqbot.profile.generateFailed'))
    await load()
  } finally {
    busy.value = false
  }
}

async function unbind(): Promise<void> {
  showConfirm.value = false
  busy.value = true
  try {
    await unbindMyQQ()
    appStore.showSuccess(t('qqbot.profile.unbound'))
    codeInfo.value = null
    await load()
  } catch {
    appStore.showError(t('qqbot.profile.unbindFailed'))
  } finally {
    busy.value = false
  }
}

onMounted(load)
onBeforeUnmount(stopTimer)
</script>
