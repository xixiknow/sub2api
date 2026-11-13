/**
 * Admin upstream balance maintenance: overview, history, alert settings and records.
 */

import { apiClient } from '../client'

export type UpstreamWalletStatus = 'exhausted' | 'critical' | 'warn' | 'unknown' | 'ok' | 'unlimited'
export type UpstreamBurnSource = 'snapshot' | 'usage'

export interface UpstreamBalanceThresholds {
  warn_days: number
  critical_days: number
  min_balance: number
  target_days: number
}

export interface UpstreamBalanceAlertOverride {
  disabled?: boolean
  warn_days?: number | null
  critical_days?: number | null
  min_balance?: number | null
}

export interface UpstreamWalletMember {
  account_id: number
  name: string
  platform: string
  group_ids?: number[]
  schedulable: boolean
  paused_by_balance: boolean
  cost_24h: number
  cost_7d: number
  override?: UpstreamBalanceAlertOverride
}

export interface UpstreamWallet {
  key: string
  primary_account_id: number
  site: string
  /** 手动钱包标签：相同标签的账号合并为一个钱包。 */
  label?: string
  source?: string
  members: UpstreamWalletMember[]
  fetch_status: string
  fetch_error?: string
  fetched_at?: string
  stale: boolean
  balance?: number
  used?: number
  limit?: number
  unlimited: boolean
  currency?: string
  daily_burn?: number
  daily_burn_source?: UpstreamBurnSource
  burn_7d?: number
  burn_7d_source?: UpstreamBurnSource
  days_left?: number
  exhaust_at?: string
  suggest_recharge?: number
  traffic_share: number
  status: UpstreamWalletStatus
  alert_disabled: boolean
  /** 近 24 小时与 7 天都没有消耗。 */
  idle: boolean
  thresholds: UpstreamBalanceThresholds
}

export interface UpstreamBalanceOverviewSummary {
  wallets: number
  total_balance: number
  daily_burn: number
  days_left?: number
  exhausted: number
  critical: number
  warn: number
  unknown: number
  unlimited: number
}

export interface UpstreamBalanceOverview {
  summary: UpstreamBalanceOverviewSummary
  wallets: UpstreamWallet[]
  thresholds: UpstreamBalanceThresholds
  generated_at: string
}

export interface UpstreamBalanceHistory {
  account_id: number
  points: { t: string; balance: number }[]
  daily_cost: { date: string; cost: number }[]
}

export type SmsProvider = 'aliyun' | 'tencent'

export type UpstreamBalanceAlertChannel = 'serverchan' | 'sms'

export interface UpstreamBalanceServerChanConfig {
  /** 仅提交时填写；留空表示保持原值。 */
  send_keys?: string[]
  send_keys_count?: number
}

export interface UpstreamBalanceSMSConfig {
  provider: SmsProvider
  access_key_id: string
  /** 仅提交时填写；留空表示保持原值。 */
  access_key_secret?: string
  access_key_secret_set?: boolean
  sign_name: string
  template_id: string
  sdk_app_id?: string
  region?: string
  endpoint?: string
}

export interface UpstreamBalanceAlertConfig {
  enabled: boolean
  channel: UpstreamBalanceAlertChannel
  warn_days: number
  critical_days: number
  min_balance: number
  target_days: number
  auto_pause_exhausted: boolean
  refresh_interval_minutes: number
  phones: string[]
  daily_limit: number
  quiet_start: string
  quiet_end: string
  sms: UpstreamBalanceSMSConfig
  serverchan: UpstreamBalanceServerChanConfig
}

export interface UpstreamBalanceAlert {
  id: number
  level: 'warn' | 'critical' | 'exhausted' | 'test'
  account_levels: Record<string, string>
  content: string
  provider: string
  phones: string
  status: 'sent' | 'failed'
  error?: string
  created_at: string
}

export interface UpstreamBalanceOverviewQuery {
  platform?: string
  status?: string
  group_id?: number
  search?: string
}

export async function getOverview(params: UpstreamBalanceOverviewQuery): Promise<UpstreamBalanceOverview> {
  const { data } = await apiClient.get<UpstreamBalanceOverview>('/admin/upstream-balance/overview', { params })
  return data
}

export async function refreshAll(): Promise<void> {
  await apiClient.post('/admin/upstream-balance/refresh-all', undefined, { timeout: 300000 })
}

export async function getHistory(accountId: number, days = 7): Promise<UpstreamBalanceHistory> {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
  const { data } = await apiClient.get<UpstreamBalanceHistory>(`/admin/upstream-balance/accounts/${accountId}/history`, {
    params: { days, timezone }
  })
  return data
}

export async function updateWalletLabel(accountIds: number[], label: string): Promise<void> {
  await apiClient.put('/admin/upstream-balance/wallet-label', { account_ids: accountIds, label })
}

export async function updateAccountAlert(accountId: number, override: UpstreamBalanceAlertOverride): Promise<void> {
  await apiClient.put(`/admin/upstream-balance/accounts/${accountId}/alert`, override)
}

export async function getSettings(): Promise<UpstreamBalanceAlertConfig> {
  const { data } = await apiClient.get<UpstreamBalanceAlertConfig>('/admin/upstream-balance/settings')
  return data
}

export async function updateSettings(config: UpstreamBalanceAlertConfig): Promise<UpstreamBalanceAlertConfig> {
  const { data } = await apiClient.put<UpstreamBalanceAlertConfig>('/admin/upstream-balance/settings', config)
  return data
}

export async function sendTestSMS(): Promise<void> {
  await apiClient.post('/admin/upstream-balance/settings/test-sms')
}

export async function listAlerts(days = 30): Promise<UpstreamBalanceAlert[]> {
  const { data } = await apiClient.get<UpstreamBalanceAlert[]>('/admin/upstream-balance/alerts', { params: { days } })
  return data
}

export const upstreamBalanceAPI = {
  getOverview,
  refreshAll,
  getHistory,
  updateAccountAlert,
  updateWalletLabel,
  getSettings,
  updateSettings,
  sendTestSMS,
  listAlerts
}

export default upstreamBalanceAPI
