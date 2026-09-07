<template>
  <section id="video-docs" class="card overflow-hidden">
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('videoCatalog.docs') }}</h2>
      <button type="button" class="btn btn-secondary btn-sm" @click="copyMarkdown">
        <Icon :name="copied ? 'check' : 'copy'" size="sm" class="mr-1.5" />
        {{ copied ? t('videoCatalog.copied') : t('videoCatalog.copyMarkdown') }}
      </button>
    </div>
    <div
      ref="docRoot"
      class="video-catalog-md px-5 py-6"
      v-html="renderedHtml"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import zhDoc from '@/docs/video-api.zh.md?raw'
import enDoc from '@/docs/video-api.en.md?raw'
import type { VideoCatalogFamily } from '@/api/videoCatalog'

const props = defineProps<{
  families: VideoCatalogFamily[]
}>()

const { t, locale } = useI18n()
const appStore = useAppStore()
const copied = ref(false)
const docRoot = ref<HTMLElement | null>(null)

marked.setOptions({ breaks: true, gfm: true })

const sourceMarkdown = computed(() => (String(locale.value).startsWith('zh') ? zhDoc : enDoc))

const resolvedMarkdown = computed(() => {
  const origin = typeof window !== 'undefined' ? window.location.origin : '<BASE_URL>'
  return sourceMarkdown.value.replaceAll('<BASE_URL>', origin)
})

const renderedHtml = computed(() => {
  const html = marked.parse(resolvedMarkdown.value) as string
  return DOMPurify.sanitize(html)
})

watch([renderedHtml, () => props.families], () => {
  void nextTick(applyHeadingIds)
})

onMounted(() => {
  applyHeadingIds()
})

function applyHeadingIds() {
  const root = docRoot.value
  if (!root) return
  const headings = root.querySelectorAll('h3')
  for (const heading of headings) {
    const text = heading.textContent || ''
    const family = props.families.find((item) => text.includes(item.family))
    if (family) {
      heading.id = `doc-${family.family}`
      heading.classList.add('scroll-mt-24')
    }
  }
}

async function copyMarkdown() {
  try {
    await navigator.clipboard.writeText(resolvedMarkdown.value)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    appStore.showError(t('videoCatalog.copyFailed'))
  }
}

defineExpose({
  scrollToFamily(family: string) {
    document.getElementById(`doc-${family}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
})
</script>

<style>
.video-catalog-md {
  line-height: 1.7;
  color: inherit;
}
.video-catalog-md h1 { @apply mb-4 mt-2 border-b border-gray-200 pb-2 text-2xl font-bold dark:border-dark-600; }
.video-catalog-md h2 { @apply mb-3 mt-8 text-xl font-bold; }
.video-catalog-md h3 { @apply mb-2 mt-6 text-lg font-semibold; }
.video-catalog-md p { @apply mb-4; }
.video-catalog-md ul { @apply mb-4 list-disc pl-6; }
.video-catalog-md ol { @apply mb-4 list-decimal pl-6; }
.video-catalog-md li { @apply mb-1; }
.video-catalog-md a { @apply text-primary-500 underline hover:text-primary-600; }
.video-catalog-md table { @apply my-4 w-full border-collapse text-sm; }
.video-catalog-md th { @apply border border-gray-200 bg-gray-50 px-3 py-2 text-left font-semibold dark:border-dark-600 dark:bg-dark-800; }
.video-catalog-md td { @apply border border-gray-200 px-3 py-2 dark:border-dark-600; }
.video-catalog-md code { @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-sm dark:bg-dark-700; }
.video-catalog-md pre { @apply my-4 overflow-x-auto rounded-lg bg-gray-900 p-4 text-gray-100 dark:bg-dark-900; }
.video-catalog-md pre code { @apply bg-transparent p-0 text-inherit; }
.video-catalog-md hr { @apply my-6 border-gray-200 dark:border-dark-600; }
</style>
