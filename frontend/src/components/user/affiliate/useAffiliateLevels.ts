import { computed, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AffiliateLevelRateRule, UserAffiliateDetail } from '@/types'

const fallbackLevelUnlockThresholds: Record<number, number> = { 1: 0, 2: 3, 3: 10 }

export interface UnlockProgressRow {
  level: number
  title: string
  source: string
  rateLabel: string
  unlocked: boolean
  remaining: number
  progressPercent: number
  progressText: string
  requirementLabel: string
  statusLabel: string
  rebateAmount: number
}

export function formatCount(value: number): string {
  return (Number(value) || 0).toLocaleString()
}

export function formatPercent(value: number): string {
  const rounded = Math.round((Number(value) || 0) * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
}

function normalizeUnlockThreshold(value: number): number {
  const parsed = Math.floor(Number(value))
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

/** 三级返利比例、解锁进度与各级累计金额的统一计算。 */
export function useAffiliateLevels(detail: Ref<UserAffiliateDetail | null>) {
  const { t } = useI18n()

  const qualifiedInviteCount = computed(() => detail.value?.qualified_aff_count ?? 0)
  const formattedRebateRate = computed(() => formatPercent(detail.value?.effective_rebate_rate_percent ?? 0))

  const levelRebateAmounts = computed(() => {
    const rows = detail.value?.level_rebates ?? []
    return [1, 2, 3].map((level) => rows.find((item) => item.level === level)?.rebate_amount ?? 0)
  })

  const effectiveLevelRates = computed<AffiliateLevelRateRule[]>(() => {
    const rows = detail.value?.effective_level_rates ?? []
    const invitedCount = qualifiedInviteCount.value
    return [1, 2, 3].map((level) => {
      const existing = rows.find((item) => item.level === level)
      const threshold = normalizeUnlockThreshold(existing?.unlock_invite_count ?? fallbackLevelUnlockThresholds[level] ?? 0)
      const unlocked = existing?.unlocked ?? (level <= 1 || invitedCount >= threshold)
      return {
        level,
        rate_percent: existing?.rate_percent ?? (level === 1 ? (detail.value?.effective_rebate_rate_percent ?? 0) : 0),
        source: existing?.source ?? (unlocked ? 'global' : 'locked'),
        unlocked,
        unlock_invite_count: threshold,
      }
    })
  })

  const unlockProgressRows = computed<UnlockProgressRow[]>(() => {
    const invitedCount = qualifiedInviteCount.value
    return effectiveLevelRates.value.map((rule, index) => {
      const threshold = normalizeUnlockThreshold(rule.unlock_invite_count ?? 0)
      const unlocked = rule.level <= 1 || Boolean(rule.unlocked)
      const remaining = Math.max(0, threshold - invitedCount)
      const progressPercent = rule.level <= 1 || threshold <= 0
        ? 100
        : Math.min(100, Math.max(0, (invitedCount / threshold) * 100))
      return {
        level: rule.level,
        title: t('affiliate.levelRules.levelTitle', { level: rule.level }),
        source: rule.source,
        rateLabel: unlocked
          ? `${formatPercent(rule.rate_percent)}%`
          : t('affiliate.rebateGuide.lockedRateLabel'),
        unlocked,
        remaining,
        progressPercent,
        progressText: rule.level <= 1
          ? t('affiliate.rebateGuide.availableNow')
          : t('affiliate.rebateGuide.progressValue', {
              current: formatCount(Math.min(invitedCount, threshold)),
              target: formatCount(threshold),
            }),
        requirementLabel: rule.level <= 1
          ? t('affiliate.rebateGuide.level1Requirement')
          : t('affiliate.rebateGuide.levelUnlockRequirement', { level: rule.level, count: formatCount(threshold) }),
        statusLabel: unlocked ? t('affiliate.rebateGuide.unlocked') : t('affiliate.rebateGuide.locked'),
        rebateAmount: levelRebateAmounts.value[index] ?? 0,
      }
    })
  })

  const nextLockedProgress = computed(() => unlockProgressRows.value.find((row) => !row.unlocked) ?? null)

  function levelRateSourceLabel(source: string): string {
    if (source === 'exclusive') return t('affiliate.levelRules.sourceExclusive')
    if (source === 'locked') return t('affiliate.levelRules.sourceLocked')
    return t('affiliate.levelRules.sourceGlobal')
  }

  return {
    qualifiedInviteCount,
    formattedRebateRate,
    levelRebateAmounts,
    unlockProgressRows,
    nextLockedProgress,
    levelRateSourceLabel,
  }
}
