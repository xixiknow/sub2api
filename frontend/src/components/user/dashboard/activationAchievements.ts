import type { UserDashboardStats } from '@/api/usage'
import type { User, UserAffiliateDetail, UserGrowthStatus } from '@/types'

export type AchievementTone =
  | 'aqua'
  | 'emerald'
  | 'gold'
  | 'indigo'
  | 'rose'
  | 'violet'

export type AchievementTier = 'bronze' | 'silver' | 'gold' | 'platinum'

export type AchievementIcon =
  | 'badge'
  | 'bolt'
  | 'chart'
  | 'chat'
  | 'dollar'
  | 'gift'
  | 'key'
  | 'shield'
  | 'sparkles'
  | 'terminal'
  | 'trendingUp'
  | 'userPlus'
  | 'users'

export type AchievementMotif =
  | 'bars'
  | 'coin'
  | 'compass'
  | 'crown'
  | 'engine'
  | 'keyhole'
  | 'network'
  | 'pulse'
  | 'rocket'
  | 'shield'
  | 'singleNode'
  | 'starburst'
  | 'terminalGrid'

export interface AchievementProgress {
  current: number
  target: number
  label: string
}

export interface ActivationContext {
  stats: UserDashboardStats
  user: User | null
  affiliate: UserAffiliateDetail | null
  growth?: UserGrowthStatus | null
}

export interface AchievementDefinition {
  id: string
  name: string
  title: string
  description: string
  condition: string
  reward: string
  tone: AchievementTone
  icon: AchievementIcon
  motif: AchievementMotif
  tier: AchievementTier
  points: number
  evaluate: (ctx: ActivationContext) => boolean
  progress: (ctx: ActivationContext) => AchievementProgress
}

export interface ActivationAchievement extends AchievementDefinition {
  unlocked: boolean
  progressValue: number
}

function clampProgress(current: number, target: number): AchievementProgress {
  return {
    current: Math.min(Math.max(current, 0), target),
    target,
    label: `${Math.min(Math.max(current, 0), target)}/${target}`,
  }
}

function countStarterTasks(ctx: ActivationContext): number {
  return [
    ctx.stats.total_api_keys > 0,
    ctx.stats.total_requests > 0,
    ctx.growth?.community_joined === true,
    ctx.growth?.affiliate_tutorial_done === true || (ctx.affiliate?.aff_count ?? 0) > 0,
  ].filter(Boolean).length
}

export const ACHIEVEMENT_DEFINITIONS: AchievementDefinition[] = [
  {
    id: 'api_key_master',
    name: '密钥掌控者',
    title: '创建第一个 API Key',
    description: '完成调用前的关键准备动作。',
    condition: 'API Key 数量达到 1',
    reward: '新人任务进度 +1',
    tone: 'aqua',
    icon: 'key',
    motif: 'keyhole',
    tier: 'bronze',
    points: 15,
    evaluate: (ctx) => ctx.stats.total_api_keys > 0,
    progress: (ctx) => clampProgress(ctx.stats.total_api_keys, 1),
  },
  {
    id: 'first_request',
    name: '首调成功',
    title: '完成第一次 API 调用',
    description: '从注册转为真实使用，进入可持续转化阶段。',
    condition: '累计调用达到 1 次',
    reward: '新人任务进度 +1',
    tone: 'emerald',
    icon: 'terminal',
    motif: 'terminalGrid',
    tier: 'bronze',
    points: 20,
    evaluate: (ctx) => ctx.stats.total_requests > 0,
    progress: (ctx) => clampProgress(ctx.stats.total_requests, 1),
  },
  {
    id: 'task_explorer',
    name: '新手毕业',
    title: '完成 4 个新人任务',
    description: '完成创建 API Key、首次调用、邀请返利教学、兑换群福利这 4 个入门动作。',
    condition: '新人任务达到 4 个',
    reward: '成长进度加速',
    tone: 'violet',
    icon: 'badge',
    motif: 'compass',
    tier: 'silver',
    points: 30,
    evaluate: (ctx) => countStarterTasks(ctx) >= 4,
    progress: (ctx) => clampProgress(countStarterTasks(ctx), 4),
  },
  {
    id: 'call_100',
    name: '调用先锋',
    title: '累计 100 次调用',
    description: '开始形成稳定使用习惯。',
    condition: '累计调用达到 100 次',
    reward: '进阶使用徽章',
    tone: 'indigo',
    icon: 'bolt',
    motif: 'pulse',
    tier: 'silver',
    points: 40,
    evaluate: (ctx) => ctx.stats.total_requests >= 100,
    progress: (ctx) => clampProgress(ctx.stats.total_requests, 100),
  },
  {
    id: 'call_1000',
    name: '千次调用',
    title: '累计 1,000 次调用',
    description: '进入高频使用阶段。',
    condition: '累计调用达到 1,000 次',
    reward: '高频用户徽章',
    tone: 'aqua',
    icon: 'chart',
    motif: 'bars',
    tier: 'gold',
    points: 80,
    evaluate: (ctx) => ctx.stats.total_requests >= 1000,
    progress: (ctx) => clampProgress(ctx.stats.total_requests, 1000),
  },
  {
    id: 'call_10000',
    name: '万次引擎',
    title: '累计 10,000 次调用',
    description: '已经具备核心用户特征。',
    condition: '累计调用达到 10,000 次',
    reward: '核心用户徽章',
    tone: 'gold',
    icon: 'sparkles',
    motif: 'engine',
    tier: 'platinum',
    points: 160,
    evaluate: (ctx) => ctx.stats.total_requests >= 10000,
    progress: (ctx) => clampProgress(ctx.stats.total_requests, 10000),
  },
  {
    id: 'invite_1',
    name: '破冰邀请官',
    title: '完成 1 个有效邀请',
    description: '用邀请链接带来第一位新用户。',
    condition: '邀请人数达到 1',
    reward: '邀请成长进度 +1',
    tone: 'emerald',
    icon: 'userPlus',
    motif: 'singleNode',
    tier: 'bronze',
    points: 30,
    evaluate: (ctx) => (ctx.affiliate?.aff_count ?? 0) >= 1,
    progress: (ctx) => clampProgress(ctx.affiliate?.aff_count ?? 0, 1),
  },
  {
    id: 'invite_3',
    name: '增长伙伴',
    title: '完成 3 个有效邀请',
    description: '达到二级返现解锁门槛，下级邀请带来的充值也能产生返利。',
    condition: '邀请人数达到 3',
    reward: '二级返现解锁资格',
    tone: 'indigo',
    icon: 'users',
    motif: 'network',
    tier: 'silver',
    points: 60,
    evaluate: (ctx) => (ctx.affiliate?.aff_count ?? 0) >= 3,
    progress: (ctx) => clampProgress(ctx.affiliate?.aff_count ?? 0, 3),
  },
  {
    id: 'invite_10',
    name: '渠道先锋',
    title: '完成 10 个有效邀请',
    description: '进入渠道型增长阶段，解锁三级返现资格。',
    condition: '邀请人数达到 10',
    reward: '三级返现解锁资格',
    tone: 'gold',
    icon: 'trendingUp',
    motif: 'rocket',
    tier: 'gold',
    points: 120,
    evaluate: (ctx) => (ctx.affiliate?.aff_count ?? 0) >= 10,
    progress: (ctx) => clampProgress(ctx.affiliate?.aff_count ?? 0, 10),
  },
  {
    id: 'invite_30',
    name: '分销高手',
    title: '完成 30 个有效邀请',
    description: '具备长期返利合伙人价值。',
    condition: '邀请人数达到 30',
    reward: '三级返现升级资格',
    tone: 'rose',
    icon: 'dollar',
    motif: 'crown',
    tier: 'platinum',
    points: 240,
    evaluate: (ctx) => (ctx.affiliate?.aff_count ?? 0) >= 30,
    progress: (ctx) => clampProgress(ctx.affiliate?.aff_count ?? 0, 30),
  },
]

export function resolveActivationAchievements(ctx: ActivationContext): ActivationAchievement[] {
  return ACHIEVEMENT_DEFINITIONS.map((definition) => {
    const unlocked = definition.evaluate(ctx)
    const progress = definition.progress(ctx)
    const progressValue = progress.target > 0 ? Math.min(progress.current / progress.target, 1) : 0
    return {
      ...definition,
      unlocked,
      progressValue,
    }
  })
}

export function formatPercent(value: number): string {
  const rounded = Math.round(value * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
}
