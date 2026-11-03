/**
 * Admin profit (盈利运营) statistics API endpoints.
 */

import { apiClient } from '../client'

export type ProfitDimension = 'group' | 'account' | 'platform' | 'model'

export interface ProfitQuery {
  start_date: string
  end_date: string
  timezone?: string
  group_id?: number
  account_id?: number
  platform?: string
  include_subscription?: boolean
}

export interface ProfitTotals {
  requests: number
  total_tokens: number
  standard_cost: number
  revenue: number
  cost: number
  profit: number
  margin: number
}

export interface ProfitSummary extends ProfitTotals {
  subscription_consumption: number
  subscription_cost: number
  subscription_requests: number
}

export interface ProfitBreakdownItem extends ProfitTotals {
  key: string
  id: number
  name: string
  platform?: string
  rate_multiplier?: number
}

export interface ProfitTrendPoint extends ProfitTotals {
  date: string
}

function toParams(query: ProfitQuery): Record<string, string | number | boolean> {
  const params: Record<string, string | number | boolean> = {}
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue
    params[key] = value as string | number | boolean
  }
  return params
}

export async function getSummary(query: ProfitQuery): Promise<ProfitSummary> {
  const { data } = await apiClient.get<ProfitSummary>('/admin/profit/summary', { params: toParams(query) })
  return data
}

export async function getBreakdown(query: ProfitQuery, dimension: ProfitDimension, limit = 200): Promise<ProfitBreakdownItem[]> {
  const { data } = await apiClient.get<ProfitBreakdownItem[]>('/admin/profit/breakdown', {
    params: { ...toParams(query), dimension, limit }
  })
  return data
}

export async function getTrend(query: ProfitQuery, granularity: 'day' | 'hour' = 'day'): Promise<ProfitTrendPoint[]> {
  const { data } = await apiClient.get<ProfitTrendPoint[]>('/admin/profit/trend', {
    params: { ...toParams(query), granularity }
  })
  return data
}

export const profitAPI = {
  getSummary,
  getBreakdown,
  getTrend
}

export default profitAPI
