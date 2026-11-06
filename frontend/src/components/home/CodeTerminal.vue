<template>
  <div ref="rootRef" class="code-terminal">
    <div class="flex items-center gap-3 border-b border-white/[0.06] bg-white/[0.03] px-4 py-3">
      <div class="flex gap-1.5">
        <span class="h-3 w-3 rounded-full bg-rose-400/80"></span>
        <span class="h-3 w-3 rounded-full bg-amber-300/80"></span>
        <span class="h-3 w-3 rounded-full bg-emerald-400/80"></span>
      </div>
      <div class="ml-2 flex gap-1 overflow-x-auto" role="tablist">
        <button
          v-for="(tab, i) in tabs"
          :key="tab.key"
          type="button"
          role="tab"
          :aria-selected="active === i"
          class="whitespace-nowrap rounded-lg px-3 py-1 text-xs font-medium transition-colors"
          :class="active === i ? 'bg-white/10 text-white' : 'text-slate-400 hover:text-slate-200'"
          @click="select(i)"
        >
          {{ tab.label }}
        </button>
      </div>
      <button
        type="button"
        class="ml-auto inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1 text-xs text-slate-400 transition-colors hover:bg-white/10 hover:text-white"
        @click="copy"
      >
        <Icon :name="copied ? 'check' : 'copy'" size="xs" />
        {{ copied ? copiedLabel : copyLabel }}
      </button>
    </div>
    <pre class="code-terminal__body"><code><span
      v-for="(line, i) in rendered"
      :key="`${active}-${i}`"
      class="block"
      :class="line.kind === 'out' ? 'tok-out' : ''"
      v-html="line.html || '&#8203;'"
    ></span><span v-if="typing" class="code-terminal__caret"></span></code></pre>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useIntersectionObserver } from '@vueuse/core'
import Icon from '@/components/icons/Icon.vue'

export interface TerminalLine {
  text: string
  kind?: 'code' | 'out'
}

export interface TerminalTab {
  key: string
  label: string
  lines: TerminalLine[]
}

const props = withDefaults(
  defineProps<{
    tabs: TerminalTab[]
    copyLabel: string
    copiedLabel: string
    reducedMotion?: boolean
  }>(),
  { reducedMotion: false }
)

const rootRef = ref<HTMLElement | null>(null)
const active = ref(0)
const typed = ref(0)
const copied = ref(false)
let timer: ReturnType<typeof setInterval> | null = null
let started = false

const lines = computed(() => props.tabs[active.value]?.lines || [])
const totalChars = computed(() => lines.value.reduce((sum, l) => sum + l.text.length + 1, 0))
const typing = computed(() => typed.value < totalChars.value)

function escapeHtml(s: string) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

/** 轻量高亮：注释 / 字符串 / 提示符 / 关键字 */
function highlight(raw: string) {
  const s = escapeHtml(raw)
  if (/^\s*#/.test(raw)) return `<span class="tok-comment">${s}</span>`
  return s
    .replace(/("[^"]*"?|'[^']*'?)/g, '<span class="tok-str">$1</span>')
    .replace(/^(\$)(\s)/, '<span class="tok-prompt">$1</span>$2')
    .replace(/\b(from|import|print|export|curl)\b/g, '<span class="tok-kw">$1</span>')
    .replace(/(\s)(-[A-Za-z]{1,2})(\s)/g, '$1<span class="tok-flag">$2</span>$3')
}

const rendered = computed(() => {
  let budget = typed.value
  const out: { html: string; kind?: string }[] = []
  for (const line of lines.value) {
    if (budget <= 0) break
    const part = line.text.slice(0, budget)
    budget -= line.text.length + 1
    out.push({ html: line.kind === 'out' ? escapeHtml(part) : highlight(part), kind: line.kind })
  }
  return out
})

function run() {
  if (timer) clearInterval(timer)
  if (props.reducedMotion) {
    typed.value = totalChars.value
    return
  }
  typed.value = 0
  timer = setInterval(() => {
    typed.value += 3
    if (typed.value >= totalChars.value && timer) {
      clearInterval(timer)
      timer = null
    }
  }, 22)
}

function select(i: number) {
  active.value = i
  copied.value = false
  run()
}

async function copy() {
  const text = lines.value
    .filter((l) => l.kind !== 'out')
    .map((l) => l.text.replace(/^\$ /, ''))
    .join('\n')
  try {
    await navigator.clipboard.writeText(text)
    copied.value = true
    setTimeout(() => (copied.value = false), 1800)
  } catch {
    copied.value = false
  }
}

useIntersectionObserver(
  rootRef,
  ([entry]) => {
    if (entry?.isIntersecting && !started) {
      started = true
      run()
    }
  },
  { threshold: 0.3 }
)

watch(
  () => props.tabs,
  () => {
    if (started) run()
  }
)

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.code-terminal {
  overflow: hidden;
  border-radius: 1.25rem;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: linear-gradient(160deg, rgba(15, 23, 42, 0.92), rgba(2, 6, 23, 0.95));
  box-shadow:
    0 30px 80px -30px rgba(99, 102, 241, 0.55),
    0 0 0 1px rgba(255, 255, 255, 0.03) inset;
}

.code-terminal__body {
  min-height: 260px;
  margin: 0;
  overflow-x: auto;
  padding: 1.25rem 1.5rem 1.5rem;
  font-family: ui-monospace, 'JetBrains Mono', 'Fira Code', Menlo, monospace;
  font-size: 13px;
  line-height: 1.85;
  color: #e2e8f0;
}

.code-terminal__caret {
  display: inline-block;
  width: 8px;
  height: 15px;
  margin-left: 2px;
  vertical-align: -2px;
  background: #fde68a;
  animation: caret-blink 1s step-end infinite;
}

.code-terminal__body :deep(.tok-comment) {
  color: #64748b;
  font-style: italic;
}
.code-terminal__body :deep(.tok-str) {
  color: #86efac;
}
.code-terminal__body :deep(.tok-prompt) {
  color: #fbbf24;
  font-weight: 700;
}
.code-terminal__body :deep(.tok-kw) {
  color: #c4b5fd;
}
.code-terminal__body :deep(.tok-flag) {
  color: #7dd3fc;
}
.code-terminal__body .tok-out {
  color: #fcd34d;
}

@keyframes caret-blink {
  50% {
    opacity: 0;
  }
}
</style>
