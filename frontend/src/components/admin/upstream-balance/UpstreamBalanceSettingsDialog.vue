<template>
  <BaseDialog :show="show" :title="t('admin.upstreamBalanceOps.config.title')" width="wide" @close="emit('close')">
    <div v-if="loading" class="flex h-40 items-center justify-center"><LoadingSpinner /></div>
    <div v-else-if="form" class="space-y-6">
      <label class="flex items-center justify-between">
        <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.upstreamBalanceOps.config.enabled') }}</span>
        <Toggle v-model="form.enabled" />
      </label>

      <!-- Thresholds -->
      <section class="space-y-3">
        <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.upstreamBalanceOps.config.thresholds') }}</h4>
        <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.warnDays') }}</label>
            <input v-model.number="form.warn_days" type="number" min="0" step="0.5" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.criticalDays') }}</label>
            <input v-model.number="form.critical_days" type="number" min="0" step="0.5" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.minBalance') }}</label>
            <input v-model.number="form.min_balance" type="number" min="0" step="1" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.targetDays') }}</label>
            <input v-model.number="form.target_days" type="number" min="1" step="1" class="input" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.refreshInterval') }}</label>
            <input v-model.number="form.refresh_interval_minutes" type="number" min="10" max="1440" step="5" class="input" />
          </div>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="form.auto_pause_exhausted" type="checkbox" class="rounded" />
          {{ t('admin.upstreamBalanceOps.config.autoPause') }}
        </label>
      </section>

      <!-- Delivery -->
      <section class="space-y-3">
        <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.upstreamBalanceOps.config.delivery') }}</h4>
        <div class="grid gap-3 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.phones') }}</label>
            <textarea v-model="phonesText" rows="3" class="input font-mono text-sm" placeholder="13800000000" />
          </div>
          <div class="space-y-3">
            <div>
              <label class="input-label">{{ t('admin.upstreamBalanceOps.config.dailyLimit') }}</label>
              <input v-model.number="form.daily_limit" type="number" min="0" max="1000" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.upstreamBalanceOps.config.quietHours') }}</label>
              <div class="flex items-center gap-2">
                <input v-model="form.quiet_start" type="time" class="input" />
                <span class="text-gray-400">-</span>
                <input v-model="form.quiet_end" type="time" class="input" />
              </div>
              <p class="mt-1 text-xs text-gray-400">{{ t('admin.upstreamBalanceOps.config.quietHint') }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- SMS provider -->
      <section class="space-y-3">
        <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.upstreamBalanceOps.config.provider') }}</h4>
        <div class="flex rounded-lg border border-gray-200 p-0.5 dark:border-dark-600" style="width: fit-content">
          <button
            v-for="p in providers"
            :key="p"
            type="button"
            class="rounded-md px-3 py-1 text-sm"
            :class="form.sms.provider === p ? 'bg-primary-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
            @click="form.sms.provider = p"
          >
            {{ t(`admin.upstreamBalanceOps.config.providers.${p}`) }}
          </button>
        </div>
        <div class="grid gap-3 md:grid-cols-2">
          <div>
            <label class="input-label">{{ isTencent ? t('admin.upstreamBalanceOps.config.accessKeyIdTencent') : t('admin.upstreamBalanceOps.config.accessKeyId') }}</label>
            <input v-model="form.sms.access_key_id" type="text" class="input" autocomplete="off" />
          </div>
          <div>
            <label class="input-label">{{ isTencent ? t('admin.upstreamBalanceOps.config.accessKeySecretTencent') : t('admin.upstreamBalanceOps.config.accessKeySecret') }}</label>
            <input
              v-model="secret"
              type="password"
              class="input"
              autocomplete="new-password"
              :placeholder="form.sms.access_key_secret_set ? t('admin.upstreamBalanceOps.config.secretSet') : ''"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.signName') }}</label>
            <input v-model="form.sms.sign_name" type="text" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.templateId') }}</label>
            <input v-model="form.sms.template_id" type="text" class="input" />
          </div>
          <template v-if="isTencent">
            <div>
              <label class="input-label">{{ t('admin.upstreamBalanceOps.config.sdkAppId') }}</label>
              <input v-model="form.sms.sdk_app_id" type="text" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.upstreamBalanceOps.config.region') }}</label>
              <input v-model="form.sms.region" type="text" class="input" placeholder="ap-guangzhou" />
            </div>
          </template>
          <div class="md:col-span-2">
            <label class="input-label">{{ t('admin.upstreamBalanceOps.config.endpoint') }}</label>
            <input v-model="form.sms.endpoint" type="text" class="input" placeholder="https://" />
          </div>
        </div>
        <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-200">
          <p>{{ t('admin.upstreamBalanceOps.config.templateHelp') }}</p>
          <p class="mt-1 font-mono">{{ isTencent ? TENCENT_TEMPLATE : ALIYUN_TEMPLATE }}</p>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex items-center justify-between gap-2">
        <button class="btn btn-secondary" :disabled="testing || saving || !form" :title="t('admin.upstreamBalanceOps.config.testHint')" @click="sendTest">
          {{ t('admin.upstreamBalanceOps.config.test') }}
        </button>
        <div class="flex gap-2">
          <button class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="saving || !form" @click="save">{{ t('admin.upstreamBalanceOps.config.save') }}</button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { SmsProvider, UpstreamBalanceAlertConfig } from '@/api/admin/upstreamBalance'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Toggle from '@/components/common/Toggle.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const { t } = useI18n()
const appStore = useAppStore()

// 模板变量名固定：阿里云按名字，腾讯云按顺序 count、name、balance、days。
const ALIYUN_TEMPLATE = '${count}个上游余额告急，最紧急：${name}，余额${balance}，约${days}天用完'
const TENCENT_TEMPLATE = '{1}个上游余额告急，最紧急：{2}，余额{3}，约{4}天用完'

const providers: SmsProvider[] = ['aliyun', 'tencent']
const form = ref<UpstreamBalanceAlertConfig | null>(null)
const secret = ref('')
const phonesText = ref('')
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)

const isTencent = computed(() => form.value?.sms.provider === 'tencent')

async function load() {
  loading.value = true
  try {
    const cfg = await adminAPI.upstreamBalance.getSettings()
    form.value = cfg
    phonesText.value = cfg.phones.join('\n')
    secret.value = ''
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalanceOps.loadFailed')))
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (show) => {
    if (show) void load()
  },
  { immediate: true }
)

async function save(): Promise<boolean> {
  if (!form.value) return false
  saving.value = true
  try {
    const payload: UpstreamBalanceAlertConfig = {
      ...form.value,
      phones: phonesText.value.split(/[\n,，\s]+/).map((p) => p.trim()).filter(Boolean),
      sms: { ...form.value.sms, access_key_secret: secret.value.trim() || undefined }
    }
    form.value = await adminAPI.upstreamBalance.updateSettings(payload)
    phonesText.value = form.value.phones.join('\n')
    secret.value = ''
    appStore.showSuccess(t('admin.upstreamBalanceOps.config.saved'))
    emit('saved')
    return true
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalanceOps.config.saveFailed')))
    return false
  } finally {
    saving.value = false
  }
}

async function sendTest() {
  testing.value = true
  try {
    if (!(await save())) return
    await adminAPI.upstreamBalance.sendTestSMS()
    appStore.showSuccess(t('admin.upstreamBalanceOps.config.testSent'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.upstreamBalanceOps.config.testFailed')))
  } finally {
    testing.value = false
  }
}
</script>
