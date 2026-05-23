import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type GrowthBenefitType = 'group_rate' | 'affiliate_rebate'

export interface GrowthBadgeDefinition {
  id: string
  name: string
  title: string
  description: string
  condition: string
  reward: string
  category: string
  tier: string
  points: number
  unlock_count: number
  rule_count: number
}

export interface UserBadgeStatus {
  badge_id: string
  name: string
  title: string
  tier: string
  points: number
  unlocked_at: string
}

export interface BadgeBenefitRule {
  id: number
  badge_id: string
  name: string
  benefit_type: GrowthBenefitType
  group_id?: number | null
  group_name?: string
  rate_multiplier?: number | null
  affiliate_rebate_rate_percent?: number | null
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface UpsertBadgeBenefitRuleRequest {
  badge_id: string
  name?: string
  benefit_type: GrowthBenefitType
  group_id?: number | null
  rate_multiplier?: number | null
  affiliate_rebate_rate_percent?: number | null
  enabled?: boolean
}

export interface GrowthUserEntry {
  user_id: number
  email: string
  username: string
  created_at?: string | null
  total_badges: number
  best_affiliate_rebate_rate_percent?: number | null
  best_group_rate_multiplier?: number | null
  badges: UserBadgeStatus[]
}

export interface GrowthRecomputeResult {
  refreshed: number
  removed: number
}

function normalizeGrowthUserEntry(user: GrowthUserEntry): GrowthUserEntry {
  return {
    ...user,
    badges: Array.isArray(user.badges) ? user.badges : []
  }
}

export async function listBadges(): Promise<GrowthBadgeDefinition[]> {
  const { data } = await apiClient.get<GrowthBadgeDefinition[] | null>('/admin/growth/badges')
  return Array.isArray(data) ? data : []
}

export async function listBenefitRules(params?: {
  badge_id?: string
  benefit_type?: GrowthBenefitType | ''
}): Promise<BadgeBenefitRule[]> {
  const { data } = await apiClient.get<BadgeBenefitRule[] | null>('/admin/growth/benefit-rules', {
    params
  })
  return Array.isArray(data) ? data : []
}

export async function createBenefitRule(
  payload: UpsertBadgeBenefitRuleRequest
): Promise<BadgeBenefitRule> {
  const { data } = await apiClient.post<BadgeBenefitRule>('/admin/growth/benefit-rules', payload)
  return data
}

export async function updateBenefitRule(
  id: number,
  payload: UpsertBadgeBenefitRuleRequest
): Promise<BadgeBenefitRule> {
  const { data } = await apiClient.put<BadgeBenefitRule>(
    `/admin/growth/benefit-rules/${id}`,
    payload
  )
  return data
}

export async function deleteBenefitRule(id: number): Promise<{ id: number }> {
  const { data } = await apiClient.delete<{ id: number }>(`/admin/growth/benefit-rules/${id}`)
  return data
}

export async function listUsers(params: {
  page?: number
  page_size?: number
  search?: string
  badge_id?: string
}): Promise<PaginatedResponse<GrowthUserEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<GrowthUserEntry> | null>('/admin/growth/users', {
    params
  })
  return {
    items: Array.isArray(data?.items) ? data.items.map(normalizeGrowthUserEntry) : [],
    total: data?.total ?? 0,
    page: data?.page ?? params.page ?? 1,
    page_size: data?.page_size ?? params.page_size ?? 20,
    pages: data?.pages ?? 0
  }
}

export async function recomputeBadges(): Promise<GrowthRecomputeResult> {
  const { data } = await apiClient.post<GrowthRecomputeResult>('/admin/growth/badges/recompute')
  return data
}

export default {
  listBadges,
  listBenefitRules,
  createBenefitRule,
  updateBenefitRule,
  deleteBenefitRule,
  listUsers,
  recomputeBadges
}
