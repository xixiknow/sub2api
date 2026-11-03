<template>
  <aside class="flex w-60 flex-shrink-0 flex-col overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
    <div class="flex items-center gap-2 border-b border-gray-100 px-3 py-2 dark:border-dark-700">
      <span class="flex-1 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">{{ t('admin.accountTree.title') }}</span>
      <button
        class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
        :title="t('common.refresh')"
        @click="loadSummary"
      >
        <Icon name="refresh" size="xs" :class="loading ? 'animate-spin' : ''" />
      </button>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto p-1.5 text-sm">
      <button class="tree-node" :class="nodeClass('')" @click="select('')">
        <Icon name="grid" size="xs" class="text-gray-400" />
        <span class="flex-1 truncate text-left">{{ t('admin.accountTree.all') }}</span>
      </button>

      <button
        v-for="node in groupNodes"
        :key="node.value"
        class="tree-node mt-0.5"
        :class="nodeClass(node.value)"
        :title="node.title"
        @click="select(node.value)"
      >
        <PlatformIcon v-if="node.platform" :platform="node.platform" size="xs" class="flex-shrink-0" />
        <span class="flex-1 truncate text-left">{{ node.label }}</span>
        <span class="text-[10px] text-gray-400">×{{ node.rate }}</span>
        <span v-if="node.error" class="h-1.5 w-1.5 rounded-full bg-red-500" :title="t('admin.accountTree.errorCount', { count: node.error })"></span>
        <span class="tree-count">{{ node.total }}</span>
      </button>

      <button v-if="ungrouped" class="tree-node mt-1" :class="nodeClass('ungrouped')" @click="select('ungrouped')">
        <span class="flex-1 truncate text-left italic text-gray-500">{{ t('admin.accounts.ungroupedGroup') }}</span>
        <span class="tree-count">{{ ungrouped.total }}</span>
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AccountTreeSummaryItem } from '@/api/admin/accountInsights'
import type { AdminGroup } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'

const props = defineProps<{
  /** 当前选中节点：''=全部，分组 ID，或 'ungrouped' */
  selected: string
  groups: AdminGroup[]
  platform?: string
  type?: string
  search?: string
}>()

const emit = defineEmits<{
  (e: 'select', value: string): void
}>()

const { t } = useI18n()
const summary = ref<AccountTreeSummaryItem[]>([])
const loading = ref(false)

const countByGroup = computed(() => new Map(summary.value.map((item) => [item.group_id, item])))

// 平台筛选即厂商维度：选了平台只列该平台的分组；同一账号可属于多个分组，会在每个分组下各出现一次。
const groupNodes = computed(() =>
  props.groups
    .filter((group) => !props.platform || group.platform === props.platform)
    .map((group) => {
      const stat = countByGroup.value.get(group.id)
      const total = stat?.total ?? 0
      return {
        value: String(group.id),
        label: group.name,
        platform: group.platform,
        rate: Number(group.rate_multiplier ?? 1),
        total,
        error: stat?.error ?? 0,
        title: `${group.name} · ${t('admin.accountTree.activeCount', { active: stat?.active ?? 0, total })}`
      }
    })
)

const ungrouped = computed(() => countByGroup.value.get(0))

function nodeClass(value: string) {
  return props.selected === value
    ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
    : 'text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-dark-700'
}

function select(value: string) {
  emit('select', value)
}

let timer: ReturnType<typeof setTimeout> | null = null
async function loadSummary() {
  loading.value = true
  try {
    summary.value = await adminAPI.accountInsights.treeSummary({
      platform: props.platform || undefined,
      type: props.type || undefined,
      search: props.search || undefined
    })
  } catch (err) {
    console.error('Failed to load account tree summary:', err)
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.platform, props.type, props.search],
  () => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(loadSummary, 300)
  }
)

onMounted(loadSummary)

defineExpose({ refresh: loadSummary })
</script>

<style scoped>
.tree-node {
  @apply flex w-full items-center gap-2 rounded-md px-2 py-1.5 transition-colors;
}
.tree-count {
  @apply min-w-[1.5rem] rounded-full bg-gray-100 px-1.5 text-center text-[11px] text-gray-500 dark:bg-dark-700 dark:text-dark-300;
}
</style>
