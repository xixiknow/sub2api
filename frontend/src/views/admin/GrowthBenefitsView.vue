<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-5 px-4 py-4 sm:px-6 lg:px-8">
      <div class="flex flex-col gap-3 border-b border-gray-200 pb-4 dark:border-dark-700 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-950 dark:text-white">成长权益</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">按徽章自动应用专属倍率和邀请返利，用户手动配置优先。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading || recomputing" @click="loadAll">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="recomputing" @click="handleRecompute">
            <Icon name="sparkles" size="sm" :class="recomputing ? 'animate-pulse' : ''" />
            重算徽章
          </button>
        </div>
      </div>

      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div class="metric-tile">
          <span>徽章种类</span>
          <strong>{{ badges.length }}</strong>
        </div>
        <div class="metric-tile">
          <span>权益规则</span>
          <strong>{{ rules.length }}</strong>
        </div>
        <div class="metric-tile">
          <span>启用规则</span>
          <strong>{{ enabledRuleCount }}</strong>
        </div>
        <div class="metric-tile">
          <span>已解锁次数</span>
          <strong>{{ totalUnlocks }}</strong>
        </div>
      </div>

      <div class="flex flex-wrap gap-2 border-b border-gray-200 dark:border-dark-700">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          class="growth-tab"
          :class="{ 'growth-tab-active': activeTab === tab.key }"
          @click="activeTab = tab.key"
        >
          <Icon :name="tab.icon" size="sm" />
          {{ tab.label }}
        </button>
      </div>

      <section v-if="activeTab === 'badges'" class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        <article v-for="badge in badges" :key="badge.id" class="badge-card">
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="flex items-center gap-2">
                <span class="tier-dot" :class="`tier-${badge.tier}`"></span>
                <h2 class="text-base font-semibold text-gray-950 dark:text-white">{{ badge.name }}</h2>
              </div>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ badge.title }}</p>
            </div>
            <span class="badge badge-gray">{{ tierLabel(badge.tier) }}</span>
          </div>
          <dl class="mt-4 grid grid-cols-3 gap-2 text-sm">
            <div>
              <dt class="text-gray-500 dark:text-gray-400">积分</dt>
              <dd class="font-semibold text-gray-900 dark:text-white">{{ badge.points }}</dd>
            </div>
            <div>
              <dt class="text-gray-500 dark:text-gray-400">解锁</dt>
              <dd class="font-semibold text-gray-900 dark:text-white">{{ badge.unlock_count }}</dd>
            </div>
            <div>
              <dt class="text-gray-500 dark:text-gray-400">规则</dt>
              <dd class="font-semibold text-gray-900 dark:text-white">{{ badge.rule_count }}</dd>
            </div>
          </dl>
          <div class="mt-4 rounded-md bg-gray-50 p-3 text-sm text-gray-600 dark:bg-dark-800 dark:text-gray-300">
            {{ badge.condition }}
          </div>
        </article>
      </section>

      <section v-else-if="activeTab === 'rules'" class="space-y-3">
        <div class="flex flex-col gap-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900 md:flex-row md:items-center md:justify-between">
          <div class="flex flex-wrap gap-2">
            <select v-model="ruleFilters.badge_id" class="input h-10 w-56" @change="loadRules">
              <option value="">全部徽章</option>
              <option v-for="badge in badges" :key="badge.id" :value="badge.id">{{ badge.name }}</option>
            </select>
            <select v-model="ruleFilters.benefit_type" class="input h-10 w-44" @change="loadRules">
              <option value="">全部权益</option>
              <option value="group_rate">分组倍率</option>
              <option value="affiliate_rebate">邀请返利</option>
            </select>
          </div>
          <button type="button" class="btn btn-primary" @click="openCreateRule">
            <Icon name="plus" size="sm" />
            新增规则
          </button>
        </div>

        <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3">规则</th>
                <th class="px-4 py-3">徽章</th>
                <th class="px-4 py-3">权益</th>
                <th class="px-4 py-3">状态</th>
                <th class="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
              <tr v-if="rulesLoading">
                <td colspan="5" class="px-4 py-10 text-center text-gray-500">加载中...</td>
              </tr>
              <tr v-else-if="rules.length === 0">
                <td colspan="5" class="px-4 py-10 text-center text-gray-500">暂无规则</td>
              </tr>
              <tr v-for="rule in rules" v-else :key="rule.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-3">
                  <div class="font-medium text-gray-950 dark:text-white">{{ rule.name }}</div>
                  <div class="text-xs text-gray-500">#{{ rule.id }}</div>
                </td>
                <td class="px-4 py-3">{{ badgeName(rule.badge_id) }}</td>
                <td class="px-4 py-3">
                  <span class="benefit-pill" :class="rule.benefit_type === 'group_rate' ? 'benefit-rate' : 'benefit-rebate'">
                    {{ benefitSummary(rule) }}
                  </span>
                </td>
                <td class="px-4 py-3">
                  <span class="badge" :class="rule.enabled ? 'badge-success' : 'badge-gray'">
                    {{ rule.enabled ? '启用' : '停用' }}
                  </span>
                </td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-1">
                    <button type="button" class="icon-action" title="编辑" @click="openEditRule(rule)">
                      <Icon name="edit" size="sm" />
                    </button>
                    <button type="button" class="icon-action danger" title="删除" @click="handleDeleteRule(rule)">
                      <Icon name="trash" size="sm" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-else class="space-y-3">
        <div class="flex flex-col gap-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900 md:flex-row md:items-center">
          <input
            v-model="userFilters.search"
            type="text"
            class="input h-10 md:max-w-xs"
            placeholder="搜索邮箱或用户名"
            @input="handleUserSearch"
          />
          <select v-model="userFilters.badge_id" class="input h-10 md:w-56" @change="reloadUsers">
            <option value="">全部徽章</option>
            <option v-for="badge in badges" :key="badge.id" :value="badge.id">{{ badge.name }}</option>
          </select>
        </div>

        <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3">用户</th>
                <th class="px-4 py-3">徽章</th>
                <th class="px-4 py-3">自动权益</th>
                <th class="px-4 py-3">注册时间</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
              <tr v-if="usersLoading">
                <td colspan="4" class="px-4 py-10 text-center text-gray-500">加载中...</td>
              </tr>
              <tr v-else-if="users.length === 0">
                <td colspan="4" class="px-4 py-10 text-center text-gray-500">暂无匹配用户</td>
              </tr>
              <tr v-for="user in users" v-else :key="user.user_id" class="hover:bg-gray-50 dark:hover:bg-dark-800/70">
                <td class="px-4 py-3">
                  <div class="font-medium text-gray-950 dark:text-white">{{ user.email || user.username || `#${user.user_id}` }}</div>
                  <div class="text-xs text-gray-500">ID {{ user.user_id }}</div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex max-w-xl flex-wrap gap-1">
                    <span v-for="badge in user.badges" :key="badge.badge_id" class="badge badge-gray">
                      {{ badge.name }}
                    </span>
                    <span v-if="user.badges.length === 0" class="text-gray-400">无</span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <div class="space-y-1 text-sm text-gray-700 dark:text-gray-300">
                    <div v-if="user.best_group_rate_multiplier">最低倍率 {{ formatMultiplier(user.best_group_rate_multiplier) }}</div>
                    <div v-if="user.best_affiliate_rebate_rate_percent">最高返利 {{ formatPercent(user.best_affiliate_rebate_rate_percent) }}</div>
                    <div v-if="!user.best_group_rate_multiplier && !user.best_affiliate_rebate_rate_percent" class="text-gray-400">无</div>
                  </div>
                </td>
                <td class="px-4 py-3 text-gray-500">{{ formatDate(user.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="flex items-center justify-between text-sm text-gray-500">
          <span>共 {{ usersPagination.total }} 条</span>
          <div class="flex gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="usersPagination.page <= 1" @click="changeUserPage(usersPagination.page - 1)">上一页</button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="usersPagination.page * usersPagination.page_size >= usersPagination.total" @click="changeUserPage(usersPagination.page + 1)">下一页</button>
          </div>
        </div>
      </section>
    </div>

    <div v-if="ruleDialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4" @click.self="closeRuleDialog">
      <div class="w-full max-w-lg rounded-lg bg-white shadow-xl dark:bg-dark-900">
        <div class="border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-950 dark:text-white">{{ editingRule ? '编辑权益规则' : '新增权益规则' }}</h2>
        </div>
        <form class="space-y-4 px-5 py-4" @submit.prevent="submitRule">
          <div>
            <label class="input-label">徽章</label>
            <select v-model="ruleForm.badge_id" class="input" required>
              <option value="" disabled>选择徽章</option>
              <option v-for="badge in badges" :key="badge.id" :value="badge.id">{{ badge.name }}</option>
            </select>
          </div>
          <div>
            <label class="input-label">规则名称</label>
            <input v-model="ruleForm.name" class="input" placeholder="留空使用徽章名称" />
          </div>
          <div>
            <label class="input-label">权益类型</label>
            <div class="grid grid-cols-2 gap-2">
              <button
                type="button"
                class="segment-btn"
                :class="{ 'segment-btn-active': ruleForm.benefit_type === 'group_rate' }"
                @click="ruleForm.benefit_type = 'group_rate'"
              >
                分组倍率
              </button>
              <button
                type="button"
                class="segment-btn"
                :class="{ 'segment-btn-active': ruleForm.benefit_type === 'affiliate_rebate' }"
                @click="ruleForm.benefit_type = 'affiliate_rebate'"
              >
                邀请返利
              </button>
            </div>
          </div>
          <template v-if="ruleForm.benefit_type === 'group_rate'">
            <div>
              <label class="input-label">目标分组</label>
              <select v-model="ruleForm.group_id" class="input" required>
                <option value="" disabled>选择分组</option>
                <option v-for="group in groups" :key="group.id" :value="String(group.id)">{{ group.name }}</option>
              </select>
            </div>
            <div>
              <label class="input-label">倍率</label>
              <input v-model.number="ruleForm.rate_multiplier" type="number" min="0.0001" step="0.0001" class="input" required />
            </div>
          </template>
          <div v-else>
            <label class="input-label">一级邀请返利比例（%）</label>
            <input v-model.number="ruleForm.affiliate_rebate_rate_percent" type="number" min="0" max="100" step="0.01" class="input" required />
          </div>
          <label class="flex items-center justify-between rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-700">
            <span class="text-sm font-medium text-gray-800 dark:text-gray-200">启用规则</span>
            <input v-model="ruleForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          </label>
          <div class="flex justify-end gap-2 border-t border-gray-200 pt-4 dark:border-dark-700">
            <button type="button" class="btn btn-secondary" @click="closeRuleDialog">取消</button>
            <button type="submit" class="btn btn-primary" :disabled="savingRule">
              {{ savingRule ? '保存中...' : '保存' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type {
  BadgeBenefitRule,
  GrowthBadgeDefinition,
  GrowthBenefitType,
  GrowthUserEntry,
  UpsertBadgeBenefitRuleRequest
} from '@/api/admin'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores'

type TabKey = 'badges' | 'rules' | 'users'
type GrowthTabIcon = 'cog' | 'badge' | 'users'

const appStore = useAppStore()
const loading = ref(false)
const rulesLoading = ref(false)
const usersLoading = ref(false)
const recomputing = ref(false)
const savingRule = ref(false)
const activeTab = ref<TabKey>('rules')
const badges = ref<GrowthBadgeDefinition[]>([])
const rules = ref<BadgeBenefitRule[]>([])
const groups = ref<AdminGroup[]>([])
const users = ref<GrowthUserEntry[]>([])
const editingRule = ref<BadgeBenefitRule | null>(null)
const ruleDialogOpen = ref(false)

const tabs: Array<{ key: TabKey; label: string; icon: GrowthTabIcon }> = [
  { key: 'rules', label: '权益规则', icon: 'cog' },
  { key: 'badges', label: '徽章总览', icon: 'badge' },
  { key: 'users', label: '用户明细', icon: 'users' }
]

const ruleFilters = reactive<{ badge_id: string; benefit_type: GrowthBenefitType | '' }>({
  badge_id: '',
  benefit_type: ''
})

const userFilters = reactive({
  search: '',
  badge_id: ''
})

const usersPagination = reactive({
  page: 1,
  page_size: 20,
  total: 0
})

const ruleForm = reactive<{
  badge_id: string
  name: string
  benefit_type: GrowthBenefitType
  group_id: string
  rate_multiplier: number
  affiliate_rebate_rate_percent: number
  enabled: boolean
}>({
  badge_id: '',
  name: '',
  benefit_type: 'group_rate',
  group_id: '',
  rate_multiplier: 1,
  affiliate_rebate_rate_percent: 5,
  enabled: true
})

const enabledRuleCount = computed(() => rules.value.filter((rule) => rule.enabled).length)
const totalUnlocks = computed(() => badges.value.reduce((sum, badge) => sum + badge.unlock_count, 0))

let userSearchTimer: ReturnType<typeof setTimeout> | null = null

async function loadAll() {
  loading.value = true
  try {
    const [badgeList, groupList] = await Promise.all([
      adminAPI.growth.listBadges(),
      adminAPI.groups.getAll()
    ])
    badges.value = badgeList
    groups.value = groupList
    await Promise.all([loadRules(), loadUsers()])
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || '加载成长权益失败')
  } finally {
    loading.value = false
  }
}

async function loadRules() {
  rulesLoading.value = true
  try {
    rules.value = await adminAPI.growth.listBenefitRules({
      badge_id: ruleFilters.badge_id || undefined,
      benefit_type: ruleFilters.benefit_type || undefined
    })
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || '加载权益规则失败')
  } finally {
    rulesLoading.value = false
  }
}

async function loadUsers() {
  usersLoading.value = true
  try {
    const response = await adminAPI.growth.listUsers({
      page: usersPagination.page,
      page_size: usersPagination.page_size,
      search: userFilters.search || undefined,
      badge_id: userFilters.badge_id || undefined
    })
    users.value = response.items
    usersPagination.total = response.total
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || '加载用户徽章失败')
  } finally {
    usersLoading.value = false
  }
}

function reloadUsers() {
  usersPagination.page = 1
  loadUsers()
}

function handleUserSearch() {
  if (userSearchTimer) clearTimeout(userSearchTimer)
  userSearchTimer = setTimeout(reloadUsers, 300)
}

function changeUserPage(page: number) {
  usersPagination.page = page
  loadUsers()
}

async function handleRecompute() {
  recomputing.value = true
  try {
    const result = await adminAPI.growth.recomputeBadges()
    appStore.showSuccess(`徽章已重算：刷新 ${result.refreshed}，移除 ${result.removed}`)
    await loadAll()
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || '重算徽章失败')
  } finally {
    recomputing.value = false
  }
}

function openCreateRule() {
  editingRule.value = null
  ruleForm.badge_id = badges.value[0]?.id || ''
  ruleForm.name = ''
  ruleForm.benefit_type = 'group_rate'
  ruleForm.group_id = groups.value[0]?.id ? String(groups.value[0].id) : ''
  ruleForm.rate_multiplier = 1
  ruleForm.affiliate_rebate_rate_percent = 5
  ruleForm.enabled = true
  ruleDialogOpen.value = true
}

function openEditRule(rule: BadgeBenefitRule) {
  editingRule.value = rule
  ruleForm.badge_id = rule.badge_id
  ruleForm.name = rule.name
  ruleForm.benefit_type = rule.benefit_type
  ruleForm.group_id = rule.group_id ? String(rule.group_id) : ''
  ruleForm.rate_multiplier = rule.rate_multiplier ?? 1
  ruleForm.affiliate_rebate_rate_percent = rule.affiliate_rebate_rate_percent ?? 5
  ruleForm.enabled = rule.enabled
  ruleDialogOpen.value = true
}

function closeRuleDialog() {
  ruleDialogOpen.value = false
}

async function submitRule() {
  const payload = buildRulePayload()
  if (!payload) return

  savingRule.value = true
  try {
    if (editingRule.value) {
      await adminAPI.growth.updateBenefitRule(editingRule.value.id, payload)
      appStore.showSuccess('权益规则已更新')
    } else {
      await adminAPI.growth.createBenefitRule(payload)
      appStore.showSuccess('权益规则已创建')
    }
    closeRuleDialog()
    await loadRules()
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || '保存权益规则失败')
  } finally {
    savingRule.value = false
  }
}

function buildRulePayload(): UpsertBadgeBenefitRuleRequest | null {
  if (!ruleForm.badge_id) {
    appStore.showError('请选择徽章')
    return null
  }
  if (ruleForm.benefit_type === 'group_rate') {
    const groupID = Number(ruleForm.group_id)
    if (!groupID) {
      appStore.showError('请选择目标分组')
      return null
    }
    return {
      badge_id: ruleForm.badge_id,
      name: ruleForm.name || undefined,
      benefit_type: 'group_rate',
      group_id: groupID,
      rate_multiplier: Number(ruleForm.rate_multiplier),
      enabled: ruleForm.enabled
    }
  }
  return {
    badge_id: ruleForm.badge_id,
    name: ruleForm.name || undefined,
    benefit_type: 'affiliate_rebate',
    affiliate_rebate_rate_percent: Number(ruleForm.affiliate_rebate_rate_percent),
    enabled: ruleForm.enabled
  }
}

async function handleDeleteRule(rule: BadgeBenefitRule) {
  if (!window.confirm(`删除规则「${rule.name}」？`)) return
  try {
    await adminAPI.growth.deleteBenefitRule(rule.id)
    appStore.showSuccess('权益规则已删除')
    await loadRules()
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || '删除权益规则失败')
  }
}

function badgeName(id: string) {
  return badges.value.find((badge) => badge.id === id)?.name || id
}

function benefitSummary(rule: BadgeBenefitRule) {
  if (rule.benefit_type === 'group_rate') {
    return `${rule.group_name || `分组 ${rule.group_id}`} · ${formatMultiplier(rule.rate_multiplier)}`
  }
  return `一级返利 ${formatPercent(rule.affiliate_rebate_rate_percent)}`
}

function formatMultiplier(value?: number | null) {
  if (value === null || value === undefined) return '-'
  return `${Number(value).toFixed(4).replace(/0+$/, '').replace(/\.$/, '')}x`
}

function formatPercent(value?: number | null) {
  if (value === null || value === undefined) return '-'
  return `${Number(value).toFixed(2).replace(/0+$/, '').replace(/\.$/, '')}%`
}

function formatDate(value?: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function tierLabel(tier: string) {
  const labels: Record<string, string> = {
    bronze: '青铜',
    silver: '白银',
    gold: '黄金',
    platinum: '铂金'
  }
  return labels[tier] || tier
}

onMounted(loadAll)
</script>

<style scoped>
.metric-tile {
  display: flex;
  min-height: 84px;
  flex-direction: column;
  justify-content: space-between;
  border: 1px solid rgb(229 231 235);
  border-radius: 8px;
  background: white;
  padding: 16px;
}

.dark .metric-tile {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
}

.metric-tile span {
  color: rgb(107 114 128);
  font-size: 0.875rem;
}

.metric-tile strong {
  color: rgb(17 24 39);
  font-size: 1.75rem;
  line-height: 1;
}

.dark .metric-tile strong {
  color: white;
}

.growth-tab {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  border-bottom: 2px solid transparent;
  padding: 0.75rem 0.25rem;
  color: rgb(107 114 128);
  font-size: 0.875rem;
  font-weight: 600;
}

.growth-tab-active {
  border-color: rgb(37 99 235);
  color: rgb(37 99 235);
}

.badge-card {
  border: 1px solid rgb(229 231 235);
  border-radius: 8px;
  background: white;
  padding: 16px;
}

.dark .badge-card {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
}

.tier-dot {
  height: 0.625rem;
  width: 0.625rem;
  flex: 0 0 0.625rem;
  border-radius: 999px;
}

.tier-bronze {
  background: rgb(180 83 9);
}

.tier-silver {
  background: rgb(100 116 139);
}

.tier-gold {
  background: rgb(217 119 6);
}

.tier-platinum {
  background: rgb(14 165 233);
}

.benefit-pill {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 0.25rem 0.625rem;
  font-size: 0.8125rem;
  font-weight: 600;
}

.benefit-rate {
  background: rgb(239 246 255);
  color: rgb(30 64 175);
}

.benefit-rebate {
  background: rgb(236 253 245);
  color: rgb(4 120 87);
}

.icon-action {
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: rgb(107 114 128);
}

.icon-action:hover {
  background: rgb(243 244 246);
  color: rgb(31 41 55);
}

.icon-action.danger:hover {
  background: rgb(254 242 242);
  color: rgb(220 38 38);
}

.segment-btn {
  border: 1px solid rgb(209 213 219);
  border-radius: 8px;
  padding: 0.625rem 0.75rem;
  color: rgb(75 85 99);
  font-size: 0.875rem;
  font-weight: 600;
}

.segment-btn-active {
  border-color: rgb(37 99 235);
  background: rgb(239 246 255);
  color: rgb(30 64 175);
}
</style>
