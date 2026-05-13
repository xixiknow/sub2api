/**
 * Redeem code API endpoints
 * Handles redeem code redemption for users
 */

import { apiClient } from './client'
import type { RedeemCodeRequest } from '@/types'

export interface RedeemResult {
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
}

export interface RedeemHistoryItem {
  id: number
  code: string
  type: string
  value: number
  status: string
  used_at: string
  created_at: string
  // Notes from admin for admin_balance/admin_concurrency types
  notes?: string
  // Subscription-specific fields
  group_id?: number
  validity_days?: number
  group?: {
    id: number
    name: string
  }
}

/**
 * Redeem a code
 * @param code - Redeem code string
 * @returns Redemption result with updated balance or concurrency
 */
export async function redeem(code: string): Promise<{
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
}> {
  const payload: RedeemCodeRequest = { code }

  const { data } = await apiClient.post<{
    message: string
    type: string
    value: number
    new_balance?: number
    new_concurrency?: number
  }>('/redeem', payload)

  return data
}

/**
 * Redeem a reusable promo/welfare code.
 * Used for permanent QQ group codes where each user can redeem once.
 */
export async function redeemPromo(code: string): Promise<RedeemResult> {
  const payload: RedeemCodeRequest = { code }
  const { data } = await apiClient.post<{
    bonus_amount: number
    balance_after: number
  }>('/redeem/promo', payload)

  return {
    message: 'OK',
    type: 'balance',
    value: data.bonus_amount,
    new_balance: data.balance_after,
  }
}

/**
 * Get user's redemption history
 * @returns List of redeemed codes
 */
export async function getHistory(): Promise<RedeemHistoryItem[]> {
  const { data } = await apiClient.get<RedeemHistoryItem[]>('/redeem/history')
  return data
}

export const redeemAPI = {
  redeem,
  redeemPromo,
  getHistory
}

export default redeemAPI
