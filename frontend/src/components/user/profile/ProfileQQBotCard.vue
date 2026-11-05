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
      <div v-else class="space-y-2 text-sm text-gray-600 dark:text-gray-300">
        <div class="text-gray-500 dark:text-gray-400">{{ t('qqbot.profile.notBound') }}</div>
        <div>{{ t('qqbot.profile.howTo') }}</div>
        <code class="inline-block rounded bg-gray-100 px-2 py-1 font-mono text-xs dark:bg-dark-700">
          {{ t('qqbot.profile.command', { email: email || 'name@example.com' }) }}
        </code>
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
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { getMyQQBinding, unbindMyQQ, type QQBotBinding } from '@/api/qqbot'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

defineProps<{ email?: string }>()

const { t } = useI18n()
const appStore = useAppStore()
const binding = ref<QQBotBinding | null>(null)
const busy = ref(false)
const showConfirm = ref(false)

async function load(): Promise<void> {
  try {
    binding.value = await getMyQQBinding()
  } catch {
    // Card stays hidden: the bot is an optional feature.
    binding.value = null
  }
}

async function unbind(): Promise<void> {
  showConfirm.value = false
  busy.value = true
  try {
    await unbindMyQQ()
    appStore.showSuccess(t('qqbot.profile.unbound'))
    await load()
  } catch {
    appStore.showError(t('qqbot.profile.unbindFailed'))
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>
