<template>
  <AppLayout>
    <div class="space-y-4 p-4 sm:p-6">
      <div>
        <h1 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.iqTest.bank.title') }}</h1>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.iqTest.bank.description') }}</p>
      </div>

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <!-- 题目列表 -->
        <div class="card space-y-2 p-3 lg:col-span-1">
          <button
            v-for="q in questions"
            :key="q.slug"
            :class="[
              'w-full rounded-lg border px-3 py-2 text-left text-sm',
              selected?.slug === q.slug ? 'border-primary-400 bg-primary-50 dark:bg-primary-900/20' : 'border-gray-200 hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700'
            ]"
            @click="selectQuestion(q)"
          >
            <div class="flex items-center gap-2">
              <span class="font-medium">{{ q.title || q.slug }}</span>
              <span class="rounded bg-gray-100 px-1.5 text-xs text-gray-500 dark:bg-dark-600">{{ t(`admin.iqTest.kind.${q.kind}`) }}</span>
              <span v-if="q.builtin" class="text-xs text-gray-400">{{ t('admin.iqTest.bank.builtin') }}</span>
            </div>
            <div v-if="q.kind === 'svg_draw'" class="mt-1 text-xs" :class="goodCount(q.slug) >= minRefs ? 'text-emerald-600' : 'text-amber-600'">
              {{ t('admin.iqTest.bank.good') }} {{ goodCount(q.slug) }} · {{ t('admin.iqTest.bank.bad') }} {{ badCount(q.slug) }}
            </div>
          </button>

          <details class="rounded-lg border border-dashed border-gray-300 p-3 text-sm dark:border-dark-500">
            <summary class="cursor-pointer text-primary-600">{{ t('admin.iqTest.bank.addQuestion') }}</summary>
            <div class="mt-3 space-y-2">
              <Input v-model="newQuestion.slug" :placeholder="t('admin.iqTest.bank.slug')" />
              <Input v-model="newQuestion.title" :placeholder="t('admin.iqTest.bank.questionTitle')" />
              <textarea v-model="newQuestion.prompt" rows="3" class="input w-full" :placeholder="t('admin.iqTest.bank.questionPrompt')" />
              <Input v-model="newQuestion.keywords" :placeholder="t('admin.iqTest.bank.keywords')" />
              <button class="btn btn-primary btn-sm" :disabled="!newQuestion.slug || !newQuestion.prompt" @click="createQuestion">{{ t('common.save') }}</button>
            </div>
          </details>
        </div>

        <!-- 题目详情 -->
        <div v-if="selected" class="card space-y-4 p-4 lg:col-span-2">
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="font-medium">{{ selected.title || selected.slug }}</div>
              <p class="mt-1 whitespace-pre-wrap text-xs text-gray-500">{{ selected.prompt || t('admin.iqTest.kind.random_reasoning') }}</p>
              <p v-if="selected.keywords?.length" class="mt-1 text-xs text-gray-400">{{ selected.keywords.join(', ') }}</p>
            </div>
            <button v-if="!selected.builtin" class="btn btn-danger btn-sm" @click="removeQuestion(selected.slug)"><Icon name="trash" size="sm" /></button>
          </div>

          <template v-if="selected.kind === 'svg_draw'">
            <div :class="['rounded p-2 text-sm', goodCount(selected.slug) >= recommendedRefs ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30' : 'bg-amber-50 text-amber-700 dark:bg-amber-900/30']">
              {{ goodCount(selected.slug) >= recommendedRefs ? t('admin.iqTest.bank.enough') : t('admin.iqTest.bank.needMore', { min: minRefs, rec: recommendedRefs }) }}
            </div>

            <!-- 上传 -->
            <div class="space-y-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
              <div class="text-sm font-medium">{{ t('admin.iqTest.bank.upload') }}</div>
              <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
                <Input v-model="uploadModel" :placeholder="t('admin.iqTest.bank.sampleModel')" />
                <Select v-model="uploadLabel" :options="labelOptions" />
              </div>
              <input type="file" multiple accept=".html,.htm,.svg,.txt" class="block text-sm" @change="onFiles" />
              <p class="text-xs text-gray-500">{{ t('admin.iqTest.bank.uploadHint') }}</p>
              <textarea v-model="pasteContent" rows="4" class="input w-full font-mono text-xs" :placeholder="t('admin.iqTest.bank.pasteHint')" />
              <button class="btn btn-primary btn-sm" :disabled="uploading || !pasteContent.trim()" @click="submitPaste">{{ t('admin.iqTest.bank.pasteSubmit') }}</button>
              <ul v-if="uploadErrors.length" class="text-xs text-red-600">
                <li v-for="(e, i) in uploadErrors" :key="i">{{ e }}</li>
              </ul>
            </div>

            <!-- 分类器 -->
            <div class="flex items-center justify-between rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-600">
              <div>
                <div class="font-medium">{{ t('admin.iqTest.bank.classifier') }}</div>
                <div class="text-xs text-gray-500">
                  <template v-if="classifier">
                    {{ t('admin.iqTest.bank.classifierInfo', { good: classifier.n_good, bad: classifier.n_bad, acc: `${(classifier.accuracy * 100).toFixed(0)}%` }) }}
                    · {{ formatDateTime(classifier.trained_at) }}
                  </template>
                  <template v-else>{{ t('admin.iqTest.bank.classifierNone', { min: minClassifier }) }}</template>
                </div>
              </div>
              <button class="btn btn-secondary btn-sm" :disabled="retraining" @click="retrain">{{ t('admin.iqTest.bank.retrain') }}</button>
            </div>

            <!-- 样本列表 -->
            <div>
              <div class="mb-2 text-sm font-medium">{{ t('admin.iqTest.bank.samples') }} ({{ samples.length }})</div>
              <div v-if="samples.length === 0" class="py-4 text-center text-sm text-gray-500">{{ t('admin.iqTest.noResults') }}</div>
              <div class="max-h-96 space-y-1 overflow-auto">
                <div v-for="s in samples" :key="s.id" class="flex items-center gap-3 rounded border border-gray-100 px-2 py-1 text-xs dark:border-dark-700">
                  <span :class="s.label === 'good' ? 'text-emerald-600' : 'text-red-600'">{{ s.label === 'good' ? t('admin.iqTest.bank.good') : t('admin.iqTest.bank.bad') }}</span>
                  <span class="text-gray-600 dark:text-gray-300">{{ s.model_id || '*' }}</span>
                  <span class="truncate text-gray-400">{{ s.note }}</span>
                  <span class="text-gray-400">{{ s.source }}</span>
                  <span class="ml-auto text-gray-400">{{ formatDateTime(s.created_at) }}</span>
                  <button class="text-red-500 hover:text-red-700" @click="removeSample(s.id)"><Icon name="trash" size="xs" /></button>
                </div>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Input from '@/components/common/Input.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import type { IQClassifierInfo, IQQuestion, IQReferenceSample, IQSampleStat, IQUploadResult } from '@/api/admin/iq'

const { t } = useI18n()
const appStore = useAppStore()

const questions = ref<IQQuestion[]>([])
const stats = ref<IQSampleStat[]>([])
const minRefs = ref(8)
const recommendedRefs = ref(20)
const minClassifier = ref(10)
const selected = ref<IQQuestion | null>(null)
const samples = ref<IQReferenceSample[]>([])
const classifier = ref<IQClassifierInfo | null>(null)

const labelOptions = computed<SelectOption[]>(() => [
  { value: 'good', label: t('admin.iqTest.bank.good') },
  { value: 'bad', label: t('admin.iqTest.bank.bad') }
])
const uploadModel = ref('')
const uploadLabel = ref<'good' | 'bad'>('good')
const pasteContent = ref('')
const uploading = ref(false)
const uploadErrors = ref<string[]>([])
const retraining = ref(false)
const newQuestion = reactive({ slug: '', title: '', prompt: '', keywords: '' })

const countBy = (slug: string, label: string) =>
  stats.value.filter((s) => s.question_slug === slug && s.label === label).reduce((sum, s) => sum + s.count, 0)
const goodCount = (slug: string) => countBy(slug, 'good')
const badCount = (slug: string) => countBy(slug, 'bad')

async function loadQuestions() {
  try {
    const res = await adminAPI.iq.listQuestions()
    questions.value = res.questions ?? []
    stats.value = res.sample_stats ?? []
    minRefs.value = res.min_reference_samples
    recommendedRefs.value = res.recommended_samples
    minClassifier.value = res.min_classifier_samples
    if (selected.value) {
      selected.value = questions.value.find((q) => q.slug === selected.value?.slug) ?? null
    } else {
      const firstDraw = questions.value.find((q) => q.kind === 'svg_draw')
      if (firstDraw) await selectQuestion(firstDraw)
    }
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  }
}

async function selectQuestion(q: IQQuestion) {
  selected.value = q
  samples.value = []
  classifier.value = null
  uploadErrors.value = []
  if (q.kind !== 'svg_draw') return
  try {
    const res = await adminAPI.iq.listSamples(q.slug)
    samples.value = res.samples ?? []
    classifier.value = res.classifier
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  }
}

function handleUploadResult(res: IQUploadResult) {
  const ok = res.created?.length ?? 0
  const failed = res.failed ?? []
  uploadErrors.value = failed.map((f) => `${f.file ?? ''} ${f.error}`.trim())
  appStore.showSuccess(t('admin.iqTest.bank.uploadResult', { ok, fail: failed.length }))
}

async function onFiles(ev: Event) {
  const input = ev.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  if (!selected.value || files.length === 0) return
  uploading.value = true
  try {
    handleUploadResult(await adminAPI.iq.uploadSampleFiles(selected.value.slug, files, uploadModel.value.trim(), uploadLabel.value))
    await refresh()
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function submitPaste() {
  if (!selected.value) return
  uploading.value = true
  try {
    handleUploadResult(await adminAPI.iq.uploadSampleText(selected.value.slug, pasteContent.value, uploadModel.value.trim(), uploadLabel.value, 'paste'))
    pasteContent.value = ''
    await refresh()
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  } finally {
    uploading.value = false
  }
}

async function removeSample(id: number) {
  if (!selected.value) return
  try {
    await adminAPI.iq.deleteSample(selected.value.slug, id)
    await refresh()
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  }
}

async function retrain() {
  if (!selected.value) return
  retraining.value = true
  try {
    classifier.value = await adminAPI.iq.retrainClassifier(selected.value.slug)
    appStore.showSuccess(t('admin.iqTest.bank.retrainSuccess'))
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  } finally {
    retraining.value = false
  }
}

async function createQuestion() {
  try {
    const q = await adminAPI.iq.saveQuestion({
      slug: newQuestion.slug.trim(),
      title: newQuestion.title.trim(),
      prompt: newQuestion.prompt.trim(),
      keywords: newQuestion.keywords.split(/[,，]/).map((s) => s.trim()).filter(Boolean)
    })
    Object.assign(newQuestion, { slug: '', title: '', prompt: '', keywords: '' })
    selected.value = q
    await loadQuestions()
    await selectQuestion(q)
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  }
}

async function removeQuestion(slug: string) {
  if (!window.confirm(t('admin.iqTest.bank.deleteQuestionConfirm'))) return
  try {
    await adminAPI.iq.deleteQuestion(slug)
    selected.value = null
    await loadQuestions()
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  }
}

async function refresh() {
  const current = selected.value
  await loadQuestions()
  if (current) await selectQuestion(current)
}

onMounted(loadQuestions)
</script>
