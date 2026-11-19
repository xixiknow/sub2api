<template>
  <BaseDialog :show="show" :title="`${t('admin.iqTest.title')} · ${account?.name ?? ''}`" width="extra-wide" @close="emit('close')">
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.iqTest.description') }}</p>

      <div class="flex gap-1 border-b border-gray-200 dark:border-dark-600">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          :class="[
            '-mb-px border-b-2 px-3 py-2 text-sm',
            activeTab === tab.key ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500 hover:text-gray-700'
          ]"
          @click="switchTab(tab.key)"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- 手动测试 -->
      <div v-if="activeTab === 'run'" class="space-y-4">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.iqTest.question') }}</label>
            <Select v-model="runForm.question_slug" :options="questionOptions" />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.iqTest.model') }}</label>
            <Select v-model="runForm.model_id" :options="modelSelectOptions" :placeholder="t('admin.iqTest.modelPlaceholder')" creatable searchable />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.iqTest.parallel') }}</label>
            <Select v-model="runForm.parallel_count" :options="parallelOptions" />
          </div>
        </div>
        <div v-if="selectedQuestion?.prompt" class="rounded bg-gray-50 p-2 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
          {{ selectedQuestion.prompt }}
        </div>
        <div class="flex items-center gap-3">
          <button class="btn btn-primary flex items-center gap-1.5" :disabled="running || !runForm.question_slug" @click="runTest">
            <Icon name="brain" size="sm" />
            {{ running ? t('admin.iqTest.running') : t('admin.iqTest.run') }}
          </button>
          <span v-if="round" class="text-sm">
            {{ t('admin.iqTest.roundVerdict') }}:
            <b :class="verdictTextClass(round.verdict)">{{ t(`admin.iqTest.verdict.${round.verdict}`) }}</b>
          </span>
        </div>
        <div v-if="round" class="space-y-2">
          <IQResultCard
            v-for="(r, i) in round.results"
            :key="r.id || i"
            :result="r"
            :kind="questionKind(r.question_slug)"
            :title="`${t('admin.iqTest.sample')} #${i + 1}`"
            :default-open="round.results.length === 1"
            @labeled="(l) => (r.label = l)"
          />
        </div>
      </div>

      <!-- 定时计划 -->
      <div v-else-if="activeTab === 'plans'" class="space-y-4">
        <div class="rounded-xl border border-primary-200 bg-primary-50/50 p-4 dark:border-primary-800 dark:bg-primary-900/20">
          <div class="mb-3 text-sm font-medium">{{ editingPlanId ? `#${editingPlanId}` : t('admin.iqTest.plan.add') }}</div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div>
              <label class="mb-1 block text-xs text-gray-600 dark:text-gray-400">{{ t('admin.iqTest.question') }}</label>
              <Select v-model="planForm.question_slug" :options="questionOptions" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-gray-600 dark:text-gray-400">{{ t('admin.iqTest.model') }}</label>
              <Select v-model="planForm.model_id" :options="modelSelectOptions" :placeholder="t('admin.iqTest.modelPlaceholder')" creatable searchable />
            </div>
            <div>
              <label class="mb-1 block text-xs text-gray-600 dark:text-gray-400">{{ t('admin.iqTest.parallel') }}</label>
              <Select v-model="planForm.parallel_count" :options="parallelOptions" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-gray-600 dark:text-gray-400">{{ t('admin.iqTest.plan.cron') }}</label>
              <Input v-model="planForm.cron_expression" placeholder="0 */6 * * *" :hint="t('admin.iqTest.plan.cronHint')" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-gray-600 dark:text-gray-400">{{ t('admin.iqTest.plan.failAction') }}</label>
              <Select v-model="planForm.fail_action" :options="failActionOptions" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-gray-600 dark:text-gray-400">{{ t('admin.iqTest.plan.failThreshold') }}</label>
              <Select v-model="planForm.fail_threshold" :options="thresholdOptions" />
            </div>
          </div>
          <div class="mt-3 flex items-center justify-between">
            <label class="flex items-center gap-2 text-sm">
              <Toggle v-model="planForm.enabled" />
              {{ t('admin.iqTest.plan.enabled') }}
            </label>
            <p class="text-xs text-gray-500">{{ t('admin.iqTest.plan.actionHint') }}</p>
          </div>
          <div class="mt-3 flex justify-end gap-2">
            <button v-if="editingPlanId" class="btn btn-secondary" @click="resetPlanForm">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="savingPlan" @click="savePlan">{{ t('common.save') }}</button>
          </div>
        </div>

        <div v-if="plans.length === 0" class="py-6 text-center text-sm text-gray-500">{{ t('admin.iqTest.noResults') }}</div>
        <div v-for="p in plans" :key="p.id" class="flex flex-wrap items-center gap-3 rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-600">
          <span class="font-medium">{{ questionTitle(p.question_slug) }}</span>
          <span class="text-gray-500">{{ p.model_id || '-' }} · ×{{ p.parallel_count }} · {{ p.cron_expression }}</span>
          <span class="text-xs text-gray-500">{{ t(`admin.iqTest.plan.actions.${p.fail_action}`) }}</span>
          <span v-if="p.last_verdict" :class="['text-xs', verdictTextClass(p.last_verdict)]">
            {{ t('admin.iqTest.plan.lastVerdict') }}: {{ t(`admin.iqTest.verdict.${p.last_verdict}`) }}
          </span>
          <span v-if="p.consecutive_fails" class="text-xs text-red-600">{{ t('admin.iqTest.plan.consecutiveFails') }} {{ p.consecutive_fails }}</span>
          <span v-if="p.action_applied" class="rounded bg-red-100 px-1.5 text-xs text-red-700 dark:bg-red-900/40 dark:text-red-300">{{ t('admin.iqTest.plan.actionApplied') }}</span>
          <span v-if="!p.enabled" class="text-xs text-gray-400">{{ t('common.disabled') }}</span>
          <span class="text-xs text-gray-400">{{ t('admin.iqTest.plan.nextRun') }}: {{ p.next_run_at ? formatDateTime(p.next_run_at) : '-' }}</span>
          <div class="ml-auto flex gap-2">
            <button class="btn btn-secondary btn-sm" :disabled="runningPlanId === p.id" @click="runPlanNow(p.id)">{{ t('admin.iqTest.plan.runNow') }}</button>
            <button class="btn btn-secondary btn-sm" @click="editPlan(p)"><Icon name="edit" size="sm" /></button>
            <button class="btn btn-danger btn-sm" @click="removePlan(p.id)"><Icon name="trash" size="sm" /></button>
          </div>
        </div>
      </div>

      <!-- 历史 -->
      <div v-else class="space-y-2">
        <div v-if="history.length === 0" class="py-6 text-center text-sm text-gray-500">{{ t('admin.iqTest.noResults') }}</div>
        <IQResultCard
          v-for="r in history"
          :key="r.id"
          :result="r"
          :kind="questionKind(r.question_slug)"
          :title="`${questionTitle(r.question_slug)} · ${r.model_id || '-'} · ${formatDateTime(r.created_at)}`"
          @labeled="(l) => (r.label = l)"
        />
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import Toggle from '@/components/common/Toggle.vue'
import { Icon } from '@/components/icons'
import IQResultCard from './IQResultCard.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import type { Account } from '@/types'
import type { IQFailAction, IQQuestion, IQRoundResult, IQTestPlan, IQTestResult } from '@/api/admin/iq'

const props = defineProps<{ show: boolean; account: Account | null; modelOptions: SelectOption[] }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()
const appStore = useAppStore()

type TabKey = 'run' | 'plans' | 'history'
const activeTab = ref<TabKey>('run')
const tabs = computed(() => [
  { key: 'run' as const, label: t('admin.iqTest.tabRun') },
  { key: 'plans' as const, label: t('admin.iqTest.tabPlans') },
  { key: 'history' as const, label: t('admin.iqTest.tabHistory') }
])

const questions = ref<IQQuestion[]>([])
const questionOptions = computed<SelectOption[]>(() =>
  questions.value
    .filter((q) => q.enabled)
    .map((q) => ({ value: q.slug, label: `${q.title || q.slug}（${t(`admin.iqTest.kind.${q.kind}`)}）` }))
)
const modelSelectOptions = computed<SelectOption[]>(() => [{ value: '', label: t('admin.iqTest.modelPlaceholder') }, ...props.modelOptions])
const parallelOptions: SelectOption[] = [1, 2, 3, 4, 6, 8].map((n) => ({ value: n, label: String(n) }))
const thresholdOptions: SelectOption[] = [1, 2, 3, 4, 5].map((n) => ({ value: n, label: String(n) }))
const failActionOptions = computed<SelectOption[]>(() =>
  (['observe_only', 'health_penalty', 'disable_scheduling'] as IQFailAction[]).map((a) => ({ value: a, label: t(`admin.iqTest.plan.actions.${a}`) }))
)

const questionKind = (slug: string) => questions.value.find((q) => q.slug === slug)?.kind
const questionTitle = (slug: string) => questions.value.find((q) => q.slug === slug)?.title || slug
const selectedQuestion = computed(() => questions.value.find((q) => q.slug === runForm.question_slug))

function verdictTextClass(v: string) {
  return (
    {
      passed: 'text-emerald-600',
      failed: 'text-red-600',
      inconclusive: 'text-amber-600',
      error: 'text-gray-500'
    } as Record<string, string>
  )[v] ?? ''
}

// ---------- run ----------
const runForm = reactive({ question_slug: 'candy', model_id: '', parallel_count: 1 })
const running = ref(false)
const round = ref<IQRoundResult | null>(null)

async function runTest() {
  if (!props.account) return
  running.value = true
  round.value = null
  try {
    round.value = await adminAPI.iq.runAccountTest(props.account.id, {
      model_id: String(runForm.model_id || ''),
      question_slug: runForm.question_slug,
      parallel_count: Number(runForm.parallel_count) || 1
    })
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  } finally {
    running.value = false
  }
}

// ---------- plans ----------
const plans = ref<IQTestPlan[]>([])
const editingPlanId = ref<number | null>(null)
const savingPlan = ref(false)
const runningPlanId = ref<number | null>(null)
const defaultPlan = () => ({
  question_slug: 'random',
  model_id: '',
  parallel_count: 2,
  cron_expression: '0 */6 * * *',
  fail_action: 'health_penalty' as IQFailAction,
  fail_threshold: 2,
  enabled: true
})
const planForm = reactive(defaultPlan())

function resetPlanForm() {
  Object.assign(planForm, defaultPlan())
  editingPlanId.value = null
}

function editPlan(p: IQTestPlan) {
  editingPlanId.value = p.id
  Object.assign(planForm, {
    question_slug: p.question_slug,
    model_id: p.model_id,
    parallel_count: p.parallel_count,
    cron_expression: p.cron_expression,
    fail_action: p.fail_action,
    fail_threshold: p.fail_threshold,
    enabled: p.enabled
  })
}

async function loadPlans() {
  if (!props.account) return
  try {
    plans.value = await adminAPI.iq.listPlans(props.account.id)
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  }
}

async function savePlan() {
  if (!props.account) return
  savingPlan.value = true
  const payload = {
    ...planForm,
    model_id: String(planForm.model_id || ''),
    parallel_count: Number(planForm.parallel_count),
    fail_threshold: Number(planForm.fail_threshold)
  }
  try {
    if (editingPlanId.value) {
      await adminAPI.iq.updatePlan(editingPlanId.value, payload)
    } else {
      await adminAPI.iq.createPlan(props.account.id, payload)
    }
    appStore.showSuccess(t('admin.iqTest.plan.saveSuccess'))
    resetPlanForm()
    await loadPlans()
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  } finally {
    savingPlan.value = false
  }
}

async function removePlan(id: number) {
  if (!window.confirm(t('admin.iqTest.plan.deleteConfirm'))) return
  try {
    await adminAPI.iq.deletePlan(id)
    await loadPlans()
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  }
}

async function runPlanNow(id: number) {
  runningPlanId.value = id
  try {
    const r = await adminAPI.iq.runPlan(id)
    appStore.showSuccess(`${t('admin.iqTest.roundVerdict')}: ${t(`admin.iqTest.verdict.${r.verdict}`)}`)
    await loadPlans()
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  } finally {
    runningPlanId.value = null
  }
}

// ---------- history ----------
const history = ref<IQTestResult[]>([])
async function loadHistory() {
  if (!props.account) return
  try {
    history.value = await adminAPI.iq.listResults({ account_id: props.account.id, limit: 50 })
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  }
}

function switchTab(key: TabKey) {
  activeTab.value = key
  if (key === 'plans') void loadPlans()
  if (key === 'history') void loadHistory()
}

watch(
  () => props.show,
  async (v) => {
    if (!v) return
    activeTab.value = 'run'
    round.value = null
    resetPlanForm()
    try {
      questions.value = (await adminAPI.iq.listQuestions()).questions ?? []
    } catch (e: any) {
      appStore.showError(e?.message || 'Failed')
    }
  },
  { immediate: true }
)
</script>
