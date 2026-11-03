/**
 * Admin account insights: group tree summary + upstream balance query.
 */

import { apiClient } from '../client'

/** 上游余额结果，对齐后端 service.UpstreamBalanceResult。 */
export interface UpstreamBalanceResult {
  status: 'ok' | 'unsupported' | 'failed'
  source?: 'sub2api' | 'newapi_key'
  balance?: number
  used?: number
  limit?: number
  unlimited?: boolean
  currency?: string
  fetched_at: string
  http_status?: number
  error?: string
}

/** 账号树分组节点统计；group_id 为 0 表示未分组。 */
export interface AccountTreeSummaryItem {
  group_id: number
  total: number
  active: number
  schedulable: number
  error: number
}

export interface UpstreamBalanceAccountResult {
  account_id: number
  result?: UpstreamBalanceResult
  error?: string
}

export async function treeSummary(params: {
  platform?: string
  type?: string
  search?: string
}): Promise<AccountTreeSummaryItem[]> {
  const { data } = await apiClient.get<AccountTreeSummaryItem[]>('/admin/accounts/tree-summary', { params })
  return data
}

export async function refreshAccountBalance(accountId: number): Promise<UpstreamBalanceResult> {
  const { data } = await apiClient.post<UpstreamBalanceResult>(`/admin/accounts/${accountId}/upstream-balance/refresh`)
  return data
}

export async function batchRefreshAccountBalance(accountIds: number[]): Promise<UpstreamBalanceAccountResult[]> {
  const { data } = await apiClient.post<UpstreamBalanceAccountResult[]>('/admin/accounts/upstream-balance/batch', {
    account_ids: accountIds
  })
  return data
}

export const accountInsightsAPI = {
  treeSummary,
  refreshAccountBalance,
  batchRefreshAccountBalance
}

export default accountInsightsAPI
