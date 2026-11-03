import { apiClient } from './client'
import type { ActionCaptchaRequestProof } from '@/types'

export interface CheckinPrize { cents: number; probability: number }
export interface CheckinRecord { date: string; reward_cents: number; created_at: string }
export interface CheckinStatus {
  date: string
  next_reset_at: string
  checked_in: boolean
  reward_cents: number
  snapshot: {
    version: number; active_days: number; streak_days: number; spend: number; has_recharge: boolean
    activity_score: number; streak_score: number; spend_score: number; recharge_score: number
    score: number; prizes: CheckinPrize[]
  }
  history: CheckinRecord[]
}

export async function getCheckin(): Promise<CheckinStatus> {
  return (await apiClient.get<CheckinStatus>('/user/checkin')).data
}
export async function claimCheckin(proof: ActionCaptchaRequestProof): Promise<CheckinStatus> {
  return (await apiClient.post<CheckinStatus>('/user/checkin', proof)).data
}
