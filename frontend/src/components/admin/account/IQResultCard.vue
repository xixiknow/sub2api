<template>
  <div class="rounded-lg border border-gray-200 dark:border-dark-600">
    <button type="button" class="flex w-full items-center gap-3 px-3 py-2 text-left text-sm" @click="open = !open">
      <span :class="['rounded px-2 py-0.5 text-xs font-medium', verdictClass(result.verdict)]">
        {{ t(`admin.iqTest.verdict.${result.verdict}`) }}
      </span>
      <span class="text-gray-700 dark:text-gray-200">{{ title }}</span>
      <span v-if="result.score != null" class="text-xs text-gray-500">{{ t('admin.iqTest.score') }} {{ Math.round(result.score) }}</span>
      <span class="text-xs text-gray-400">{{ (result.latency_ms / 1000).toFixed(1) }}s</span>
      <span v-if="result.label" class="text-xs text-primary-600">{{ t('admin.iqTest.labeled') }}: {{ result.label }}</span>
      <Icon :name="open ? 'chevronUp' : 'chevronDown'" size="sm" class="ml-auto text-gray-400" />
    </button>
    <div v-if="open" class="space-y-3 border-t border-gray-100 px-3 py-3 text-xs dark:border-dark-700">
      <p v-if="result.error_message" class="text-red-600 dark:text-red-400">{{ result.error_message }}</p>
      <p v-if="reasonText" class="text-amber-700 dark:text-amber-300">{{ reasonText }}</p>
      <p v-if="result.detail.hard_fails?.length" class="text-red-600 dark:text-red-400">
        {{ t('admin.iqTest.hardFails') }}: {{ result.detail.hard_fails.join(', ') }}
      </p>

      <div v-if="svgScore" class="grid grid-cols-2 gap-x-4 gap-y-1 rounded bg-gray-50 p-2 dark:bg-dark-700 sm:grid-cols-3">
        <div>{{ t('admin.iqTest.distance') }}: <b>{{ svgScore.distance.toFixed(2) }}</b></div>
        <div>{{ t('admin.iqTest.thresholds') }}: {{ svgScore.pass_threshold.toFixed(2) }} / {{ svgScore.fail_threshold.toFixed(2) }}</div>
        <div>{{ t('admin.iqTest.style') }}: {{ svgScore.style_similarity.toFixed(2) }}</div>
        <div>
          {{ t('admin.iqTest.referenceScope') }}:
          {{ svgScore.reference_scope === 'model' ? t('admin.iqTest.scopeModel') : t('admin.iqTest.scopeQuestion') }}
          ({{ svgScore.reference_count }})
        </div>
        <div v-if="svgScore.classifier_prob != null">{{ t('admin.iqTest.classifierProb') }}: {{ (svgScore.classifier_prob * 100).toFixed(0) }}%</div>
        <div v-if="svgScore.warnings?.length" class="col-span-full text-amber-700 dark:text-amber-300">
          {{ svgScore.warnings.map((w) => t(`admin.iqTest.reasons.${w}`)).join('；') }}
        </div>
        <div v-if="worstFeatures" class="col-span-full text-gray-500">{{ t('admin.iqTest.worstFeatures') }}: {{ worstFeatures }}</div>
      </div>

      <div v-if="svgSource" class="space-y-1">
        <div class="font-medium text-gray-600 dark:text-gray-300">{{ t('admin.iqTest.preview') }}</div>
        <iframe
          :srcdoc="previewDoc"
          sandbox=""
          class="h-72 w-full rounded border border-gray-200 bg-white dark:border-dark-600"
          referrerpolicy="no-referrer"
        />
      </div>

      <details>
        <summary class="cursor-pointer text-gray-500">{{ t('admin.iqTest.prompt') }}</summary>
        <pre class="mt-1 max-h-40 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-2 dark:bg-dark-700">{{ result.prompt }}</pre>
      </details>
      <details v-if="result.detail.reference">
        <summary class="cursor-pointer text-gray-500">{{ t('admin.iqTest.reference') }}</summary>
        <pre class="mt-1 whitespace-pre-wrap rounded bg-gray-50 p-2 dark:bg-dark-700">{{ result.detail.reference }}</pre>
      </details>
      <details :open="!svgSource">
        <summary class="cursor-pointer text-gray-500">{{ t('admin.iqTest.response') }}</summary>
        <pre class="mt-1 max-h-64 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-2 dark:bg-dark-700">{{ result.response_text }}</pre>
      </details>

      <div v-if="labelable" class="flex gap-2">
        <button class="btn btn-secondary btn-sm" :disabled="labeling" @click="label('good')">{{ t('admin.iqTest.labelGood') }}</button>
        <button class="btn btn-secondary btn-sm" :disabled="labeling" @click="label('bad')">{{ t('admin.iqTest.labelBad') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { IQTestResult, IQVerdict } from '@/api/admin/iq'

const props = defineProps<{ result: IQTestResult; title: string; kind?: string; defaultOpen?: boolean }>()
const emit = defineEmits<{ (e: 'labeled', label: string): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const open = ref(!!props.defaultOpen)
const labeling = ref(false)

const svgScore = computed(() => props.result.detail?.svg_score ?? null)

const reasonText = computed(() => {
  const r = props.result.detail?.reason
  if (!r) return ''
  const key = `admin.iqTest.reasons.${r}`
  const translated = t(key)
  return translated === key ? r : translated
})

const worstFeatures = computed(() => {
  const w = svgScore.value?.worst_features
  if (!w) return ''
  return Object.entries(w)
    .map(([k, v]) => `${k} ${v > 0 ? '+' : ''}${v}`)
    .join(', ')
})

const svgSource = computed(() => {
  if (props.kind !== 'svg_draw') return ''
  const text = props.result.response_text || ''
  const start = text.search(/<svg[\s>]/i)
  const end = text.toLowerCase().lastIndexOf('</svg>')
  if (start < 0 || end < start) return ''
  return text.slice(start, end + 6)
})

// 无脚本沙箱 iframe + CSP，只允许内联样式（SVG 动画需要）
const previewDoc = computed(
  () =>
    `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:"><style>html,body{margin:0;height:100%;display:flex;align-items:center;justify-content:center;background:#fff}svg{max-width:100%;max-height:100%}</style></head><body>${svgSource.value}</body></html>`
)

const labelable = computed(() => props.kind === 'svg_draw' && props.result.id > 0 && !props.result.label && !!svgSource.value)

function verdictClass(v: IQVerdict) {
  return {
    passed: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
    failed: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    inconclusive: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
    error: 'bg-gray-200 text-gray-700 dark:bg-dark-600 dark:text-gray-300'
  }[v]
}

async function label(l: 'good' | 'bad') {
  labeling.value = true
  try {
    await adminAPI.iq.labelResult(props.result.id, l)
    appStore.showSuccess(t('admin.iqTest.labelSuccess'))
    emit('labeled', l)
  } catch (e: any) {
    appStore.showError(e?.message || 'Failed')
  } finally {
    labeling.value = false
  }
}

defineExpose({ verdictClass })
</script>
