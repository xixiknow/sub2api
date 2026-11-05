<template>
  <BaseDialog
    :show="show"
    :title="editingId ? t('admin.marketingEmail.editCampaign') : t('admin.marketingEmail.createCampaign')"
    width="full"
    :close-on-click-outside="false"
    @close="emit('close')"
  >
    <div class="grid gap-6 xl:grid-cols-2">
      <!-- Left: audience + content -->
      <div class="space-y-6">
        <section class="space-y-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.marketingEmail.audience.title') }}</h3>

          <div>
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="audience.inactiveEnabled" type="checkbox" class="rounded" />
              {{ t('admin.marketingEmail.audience.inactive') }}
            </label>
            <div v-if="audience.inactiveEnabled" class="mt-2 flex flex-wrap items-center gap-2 pl-6">
              <input v-model.number="audience.inactiveDays" type="number" min="1" class="input w-24" />
              <span class="text-sm text-gray-500">{{ t('admin.marketingEmail.audience.days') }}</span>
              <button
                v-for="d in [7, 14, 30, 60]"
                :key="d"
                type="button"
                class="btn btn-secondary btn-sm"
                :class="audience.inactiveDays === d ? 'ring-2 ring-primary-500' : ''"
                @click="audience.inactiveDays = d"
              >
                {{ d }}
              </button>
            </div>
            <p class="input-hint pl-6">{{ t('admin.marketingEmail.audience.inactiveHint') }}</p>
          </div>

          <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="audience.neverUsedApi" type="checkbox" class="rounded" />
            {{ t('admin.marketingEmail.audience.neverUsedApi') }}
          </label>

          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.marketingEmail.audience.balanceRange') }}</label>
              <div class="flex items-center gap-2">
                <input v-model="audience.balanceMin" type="number" step="0.01" class="input" :placeholder="t('admin.marketingEmail.audience.min')" />
                <span class="text-gray-400">~</span>
                <input v-model="audience.balanceMax" type="number" step="0.01" class="input" :placeholder="t('admin.marketingEmail.audience.max')" />
              </div>
            </div>
            <div>
              <label class="input-label">{{ t('admin.marketingEmail.audience.registeredRange') }}</label>
              <div class="flex items-center gap-2">
                <input v-model="audience.registeredFrom" type="date" class="input" />
                <span class="text-gray-400">~</span>
                <input v-model="audience.registeredTo" type="date" class="input" />
              </div>
            </div>
          </div>

          <div>
            <label class="input-label">{{ t('admin.marketingEmail.audience.explicit') }}</label>
            <textarea
              v-model="audience.explicit"
              rows="2"
              class="input font-mono text-sm"
              :placeholder="t('admin.marketingEmail.audience.explicitPlaceholder')"
            ></textarea>
            <p class="input-hint">{{ t('admin.marketingEmail.audience.explicitHint') }}</p>
          </div>

          <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="audience.includeAdmins" type="checkbox" class="rounded" />
            {{ t('admin.marketingEmail.audience.includeAdmins') }}
          </label>

          <div class="flex flex-wrap items-center gap-3">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="audienceLoading" @click="loadAudience">
              <Icon name="users" size="sm" class="mr-1" />
              {{ t('admin.marketingEmail.audience.calculate') }}
            </button>
            <span v-if="audiencePreview" class="text-sm text-gray-700 dark:text-gray-300">
              {{ t('admin.marketingEmail.audience.matched', { count: audiencePreview.count }) }}
            </span>
          </div>
          <div v-if="audiencePreview && audiencePreview.sample.length" class="rounded-md bg-gray-50 p-3 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-400">
            <div class="mb-1 font-medium">{{ t('admin.marketingEmail.audience.sample') }}</div>
            <div v-for="u in audiencePreview.sample" :key="u.user_id" class="flex justify-between gap-2">
              <span class="truncate">#{{ u.user_id }} {{ u.email }}</span>
              <span class="shrink-0">{{ u.last_activity_at ? formatDateTime(u.last_activity_at) : '-' }}</span>
            </div>
          </div>
        </section>

        <section class="space-y-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.marketingEmail.content.title') }}</h3>
            <div class="flex items-center gap-2">
              <span class="text-xs text-gray-500">{{ t('admin.marketingEmail.content.starter') }}</span>
              <button type="button" class="btn btn-secondary btn-sm" @click="applyStarter('retention')">
                {{ t('admin.marketingEmail.content.starterRetention') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="applyStarter('feature')">
                {{ t('admin.marketingEmail.content.starterFeature') }}
              </button>
            </div>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.marketingEmail.content.name') }}</label>
              <input v-model="form.name" type="text" class="input" maxlength="200" :placeholder="t('admin.marketingEmail.content.namePlaceholder')" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.marketingEmail.content.locale') }}</label>
              <select v-model="form.locale" class="input">
                <option value="zh">简体中文</option>
                <option value="en">English</option>
              </select>
            </div>
          </div>

          <div>
            <label class="input-label">{{ t('admin.marketingEmail.content.subject') }}</label>
            <input v-model="form.subject" type="text" class="input" maxlength="200" />
            <p class="input-hint" :class="subjectLength > 40 ? 'text-amber-600 dark:text-amber-400' : ''">
              {{ t('admin.marketingEmail.content.subjectHint', { count: subjectLength }) }}
            </p>
          </div>

          <div class="grid gap-3 sm:grid-cols-3">
            <div>
              <label class="input-label">{{ t('admin.marketingEmail.content.layout') }}</label>
              <select v-model="form.layout_mode" class="input">
                <option value="card">{{ t('admin.marketingEmail.content.layoutCard') }}</option>
                <option value="raw">{{ t('admin.marketingEmail.content.layoutRaw') }}</option>
              </select>
            </div>
            <template v-if="form.layout_mode === 'card'">
              <div>
                <label class="input-label">{{ t('admin.marketingEmail.content.headerTitle') }}</label>
                <input v-model="form.title" type="text" class="input" maxlength="200" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.marketingEmail.content.accent') }}</label>
                <div class="flex items-center gap-2">
                  <input v-model="form.accent_color" type="color" class="h-9 w-12 cursor-pointer rounded border border-gray-300 dark:border-dark-600" />
                  <input v-model="form.accent_color" type="text" class="input font-mono" maxlength="7" />
                </div>
              </div>
            </template>
          </div>

          <div>
            <label class="input-label">{{ t('admin.marketingEmail.content.body') }}</label>
            <div class="mb-2 flex flex-wrap gap-1.5">
              <button
                v-for="p in CAMPAIGN_PLACEHOLDERS"
                :key="p"
                type="button"
                class="rounded bg-gray-100 px-2 py-0.5 font-mono text-xs text-gray-700 hover:bg-primary-50 hover:text-primary-700 dark:bg-dark-700 dark:text-gray-300"
                :title="t(`admin.marketingEmail.placeholders.${p}`)"
                @click="insertPlaceholder(p)"
              >
                <span v-text="placeholderToken(p)" />
              </button>
            </div>
            <textarea ref="bodyRef" v-model="form.body_html" class="input min-h-[22rem] resize-y font-mono text-sm"></textarea>
            <p class="input-hint">
              {{ form.layout_mode === 'card' ? t('admin.marketingEmail.content.bodyHintCard') : t('admin.marketingEmail.content.bodyHintRaw') }}
            </p>
          </div>

          <div>
            <label class="input-label">{{ t('admin.marketingEmail.content.throttle') }}</label>
            <input v-model.number="form.throttle_per_minute" type="number" min="1" max="600" class="input w-32" />
            <p class="input-hint">{{ t('admin.marketingEmail.content.throttleHint') }}</p>
          </div>
        </section>
      </div>

      <!-- Right: preview + tips -->
      <div class="space-y-4">
        <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700 xl:sticky xl:top-0">
          <div class="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-dark-700">
            <div class="min-w-0">
              <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.marketingEmail.preview.title') }}</div>
              <div class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">{{ previewSubject || '-' }}</div>
            </div>
            <Icon v-if="previewLoading" name="refresh" size="sm" class="animate-spin text-gray-400" />
          </div>
          <div class="bg-gray-100 p-3 dark:bg-dark-900">
            <div v-if="previewError" class="mb-2 rounded bg-red-50 px-3 py-2 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-400">
              {{ previewError }}
            </div>
            <iframe
              class="h-[40rem] w-full rounded-md border border-gray-200 bg-white dark:border-dark-700"
              sandbox=""
              :srcdoc="previewHtml"
              :title="t('admin.marketingEmail.preview.title')"
            ></iframe>
          </div>
        </div>
        <div class="rounded-lg bg-amber-50 p-4 text-xs leading-relaxed text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
          <div class="mb-1 font-semibold">{{ t('admin.marketingEmail.tips.title') }}</div>
          <ul class="list-disc space-y-0.5 pl-4">
            <li>{{ t('admin.marketingEmail.tips.subject') }}</li>
            <li>{{ t('admin.marketingEmail.tips.spam') }}</li>
            <li>{{ t('admin.marketingEmail.tips.cta') }}</li>
            <li>{{ t('admin.marketingEmail.tips.footer') }}</li>
            <li>{{ t('admin.marketingEmail.tips.test') }}</li>
          </ul>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="handleTest">
          <Icon name="mail" size="sm" class="mr-1" />
          {{ t('admin.marketingEmail.actions.sendTest') }}
        </button>
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="handleSave(false)">
          {{ t('admin.marketingEmail.actions.saveDraft') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="busy" @click="handleSave(true)">
          {{ t('admin.marketingEmail.actions.saveAndSend') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type {
  EmailCampaign,
  EmailCampaignAudience,
  EmailCampaignAudiencePreview,
  EmailCampaignInput
} from '@/api/admin/emailCampaigns'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { CAMPAIGN_PLACEHOLDERS, getStarterTemplate, type StarterTemplateKey } from './starterTemplates'

const props = defineProps<{
  show: boolean
  /** Campaign to edit (when editingId is set) or to copy from. */
  source: EmailCampaign | null
  editingId: number | null
}>()

const emit = defineEmits<{
  close: []
  saved: [campaign: EmailCampaign, send: boolean]
}>()

const { t, locale } = useI18n()
const appStore = useAppStore()

const form = reactive<Omit<EmailCampaignInput, 'audience'>>({
  name: '',
  subject: '',
  body_html: '',
  layout_mode: 'card',
  title: '',
  accent_color: '#4f46e5',
  locale: 'zh',
  throttle_per_minute: 30
})

const audience = reactive({
  inactiveEnabled: true,
  inactiveDays: 14,
  neverUsedApi: false,
  balanceMin: '' as string | number,
  balanceMax: '' as string | number,
  registeredFrom: '',
  registeredTo: '',
  explicit: '',
  includeAdmins: false
})

const audiencePreview = ref<EmailCampaignAudiencePreview | null>(null)
const audienceLoading = ref(false)
const busy = ref(false)
const bodyRef = ref<HTMLTextAreaElement | null>(null)

const previewHtml = ref('')
const previewSubject = ref('')
const previewError = ref('')
const previewLoading = ref(false)

const subjectLength = computed(() => [...form.subject].length)

function toDateInput(value?: string): string {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function fromDateInput(value: string, endOfDay = false): string | undefined {
  if (!value) return undefined
  const d = new Date(`${value}T00:00:00`)
  if (Number.isNaN(d.getTime())) return undefined
  if (endOfDay) d.setDate(d.getDate() + 1)
  return d.toISOString()
}

function numberOrUndefined(value: string | number): number | undefined {
  if (value === '' || value === null || value === undefined) return undefined
  const n = Number(value)
  return Number.isFinite(n) ? n : undefined
}

function buildAudience(): EmailCampaignAudience {
  const ids: number[] = []
  const emails: string[] = []
  for (const token of audience.explicit.split(/[\s,;，；]+/)) {
    const v = token.trim()
    if (!v) continue
    if (v.includes('@')) emails.push(v)
    else if (/^\d+$/.test(v)) ids.push(Number(v))
  }
  return {
    inactive_days: audience.inactiveEnabled && audience.inactiveDays > 0 ? audience.inactiveDays : undefined,
    never_used_api: audience.neverUsedApi || undefined,
    balance_min: numberOrUndefined(audience.balanceMin),
    balance_max: numberOrUndefined(audience.balanceMax),
    registered_from: fromDateInput(audience.registeredFrom),
    registered_to: fromDateInput(audience.registeredTo, true),
    user_ids: ids.length ? ids : undefined,
    emails: emails.length ? emails : undefined,
    include_admins: audience.includeAdmins || undefined
  }
}

function buildInput(): EmailCampaignInput {
  return { ...form, audience: buildAudience() }
}

function resetForm() {
  const src = props.source
  const defaultLocale: 'zh' | 'en' = String(locale.value).startsWith('zh') ? 'zh' : 'en'
  audiencePreview.value = null
  previewError.value = ''
  if (src) {
    Object.assign(form, {
      name: props.editingId ? src.name : `${src.name} (copy)`,
      subject: src.subject,
      body_html: src.body_html,
      layout_mode: src.layout_mode,
      title: src.title,
      accent_color: src.accent_color,
      locale: src.locale,
      throttle_per_minute: src.throttle_per_minute
    })
    const a = src.audience || {}
    Object.assign(audience, {
      inactiveEnabled: !!a.inactive_days,
      inactiveDays: a.inactive_days || 14,
      neverUsedApi: !!a.never_used_api,
      balanceMin: a.balance_min ?? '',
      balanceMax: a.balance_max ?? '',
      registeredFrom: toDateInput(a.registered_from),
      registeredTo: a.registered_to ? toDateInput(new Date(new Date(a.registered_to).getTime() - 1).toISOString()) : '',
      explicit: [...(a.user_ids || []).map(String), ...(a.emails || [])].join('\n'),
      includeAdmins: !!a.include_admins
    })
    return
  }
  const starter = getStarterTemplate('retention', defaultLocale)
  Object.assign(form, {
    name: '',
    subject: starter.subject,
    body_html: starter.body,
    layout_mode: 'card',
    title: starter.title,
    accent_color: starter.accent,
    locale: defaultLocale,
    throttle_per_minute: 30
  })
  Object.assign(audience, {
    inactiveEnabled: true,
    inactiveDays: 14,
    neverUsedApi: false,
    balanceMin: '',
    balanceMax: '',
    registeredFrom: '',
    registeredTo: '',
    explicit: '',
    includeAdmins: false
  })
}

function applyStarter(key: StarterTemplateKey) {
  const starter = getStarterTemplate(key, form.locale)
  form.subject = starter.subject
  form.title = starter.title
  form.accent_color = starter.accent
  form.body_html = starter.body
  form.layout_mode = 'card'
}

// 模板插值里不能直接写 }}，否则会提前结束插值表达式。
function placeholderToken(name: string): string {
  return '{' + '{' + name + '}' + '}'
}

async function insertPlaceholder(name: string) {
  const token = `{{${name}}}`
  const el = bodyRef.value
  if (!el) {
    form.body_html += token
    return
  }
  const start = el.selectionStart ?? form.body_html.length
  const end = el.selectionEnd ?? start
  form.body_html = form.body_html.slice(0, start) + token + form.body_html.slice(end)
  await nextTick()
  el.focus()
  el.setSelectionRange(start + token.length, start + token.length)
}

async function loadAudience() {
  audienceLoading.value = true
  try {
    audiencePreview.value = await adminAPI.emailCampaigns.previewAudience(buildAudience())
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.marketingEmail.failed')))
  } finally {
    audienceLoading.value = false
  }
}

let previewTimer: number | null = null
let previewSeq = 0
function schedulePreview() {
  if (previewTimer) window.clearTimeout(previewTimer)
  previewTimer = window.setTimeout(refreshPreview, 500)
}

async function refreshPreview() {
  if (!form.subject.trim() || !form.body_html.trim()) {
    previewError.value = ''
    return
  }
  const seq = ++previewSeq
  previewLoading.value = true
  try {
    const res = await adminAPI.emailCampaigns.preview(buildInput())
    if (seq !== previewSeq) return
    previewHtml.value = res.html
    previewSubject.value = res.subject
    previewError.value = ''
  } catch (error) {
    if (seq !== previewSeq) return
    previewError.value = extractApiErrorMessage(error, t('admin.marketingEmail.failed'))
  } finally {
    if (seq === previewSeq) previewLoading.value = false
  }
}

watch(form, schedulePreview, { deep: true })

watch(
  () => props.show,
  (visible) => {
    if (!visible) return
    resetForm()
    previewHtml.value = ''
    previewSubject.value = ''
    void refreshPreview()
  },
  { immediate: true }
)

async function handleTest() {
  busy.value = true
  try {
    const res = await adminAPI.emailCampaigns.sendTest(buildInput())
    appStore.showSuccess(t('admin.marketingEmail.testSent', { email: res.email }))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.marketingEmail.failed')))
  } finally {
    busy.value = false
  }
}

async function handleSave(send: boolean) {
  if (!form.subject.trim() || !form.body_html.trim()) {
    appStore.showError(t('admin.marketingEmail.contentRequired'))
    return
  }
  busy.value = true
  try {
    const input = buildInput()
    const saved = props.editingId
      ? await adminAPI.emailCampaigns.update(props.editingId, input)
      : await adminAPI.emailCampaigns.create(input)
    emit('saved', saved, send)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.marketingEmail.failed')))
  } finally {
    busy.value = false
  }
}
</script>
