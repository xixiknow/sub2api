/**
 * Admin marketing email campaigns API endpoints
 */

import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'

export type EmailCampaignStatus = 'draft' | 'pending' | 'running' | 'completed' | 'canceled' | 'failed'
export type EmailCampaignRecipientStatus = 'pending' | 'sent' | 'failed' | 'skipped'
export type EmailCampaignLayoutMode = 'card' | 'raw'

export interface EmailCampaignAudience {
  inactive_days?: number
  never_used_api?: boolean
  balance_min?: number
  balance_max?: number
  /** RFC3339 timestamps */
  registered_from?: string
  registered_to?: string
  user_ids?: number[]
  emails?: string[]
  include_admins?: boolean
}

export interface EmailCampaignInput {
  name: string
  subject: string
  body_html: string
  layout_mode: EmailCampaignLayoutMode
  title: string
  accent_color: string
  locale: 'zh' | 'en'
  audience: EmailCampaignAudience
  throttle_per_minute: number
}

export interface EmailCampaign extends EmailCampaignInput {
  id: number
  status: EmailCampaignStatus
  total_count: number
  sent_count: number
  failed_count: number
  skipped_count: number
  error_message: string
  created_by?: number
  started_at?: string
  finished_at?: string
  created_at: string
  updated_at: string
}

export interface EmailCampaignRecipient {
  id: number
  user_id: number
  email: string
  recipient_name: string
  last_activity_at?: string
  status: EmailCampaignRecipientStatus
  error: string
  sent_at?: string
}

export interface EmailCampaignAudienceUser {
  user_id: number
  email: string
  username: string
  balance: number
  last_activity_at?: string
}

export interface EmailCampaignAudiencePreview {
  count: number
  sample: EmailCampaignAudienceUser[]
}

export interface EmailCampaignPreview {
  subject: string
  html: string
}

export async function list(
  page = 1,
  pageSize = 20,
  filters?: { status?: string; search?: string },
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<EmailCampaign>> {
  const { data } = await apiClient.get<BasePaginationResponse<EmailCampaign>>('/admin/email-campaigns', {
    params: { page, page_size: pageSize, ...filters },
    signal: options?.signal
  })
  return data
}

export async function getById(id: number): Promise<EmailCampaign> {
  const { data } = await apiClient.get<EmailCampaign>(`/admin/email-campaigns/${id}`)
  return data
}

export async function create(request: EmailCampaignInput): Promise<EmailCampaign> {
  const { data } = await apiClient.post<EmailCampaign>('/admin/email-campaigns', request)
  return data
}

export async function update(id: number, request: EmailCampaignInput): Promise<EmailCampaign> {
  const { data } = await apiClient.put<EmailCampaign>(`/admin/email-campaigns/${id}`, request)
  return data
}

export async function deleteCampaign(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/email-campaigns/${id}`)
  return data
}

export async function previewAudience(audience: EmailCampaignAudience): Promise<EmailCampaignAudiencePreview> {
  const { data } = await apiClient.post<EmailCampaignAudiencePreview>('/admin/email-campaigns/audience/preview', audience)
  return data
}

export async function preview(request: EmailCampaignInput): Promise<EmailCampaignPreview> {
  const { data } = await apiClient.post<EmailCampaignPreview>('/admin/email-campaigns/preview', request)
  return data
}

export async function sendTest(request: EmailCampaignInput): Promise<{ email: string }> {
  const { data } = await apiClient.post<{ email: string }>('/admin/email-campaigns/test', request)
  return data
}

export async function send(id: number): Promise<EmailCampaign> {
  const { data } = await apiClient.post<EmailCampaign>(`/admin/email-campaigns/${id}/send`)
  return data
}

export async function cancel(id: number): Promise<EmailCampaign> {
  const { data } = await apiClient.post<EmailCampaign>(`/admin/email-campaigns/${id}/cancel`)
  return data
}

export async function listRecipients(
  id: number,
  page = 1,
  pageSize = 20,
  status?: string
): Promise<BasePaginationResponse<EmailCampaignRecipient>> {
  const { data } = await apiClient.get<BasePaginationResponse<EmailCampaignRecipient>>(
    `/admin/email-campaigns/${id}/recipients`,
    { params: { page, page_size: pageSize, status: status || undefined } }
  )
  return data
}

const emailCampaignsAPI = {
  list,
  getById,
  create,
  update,
  delete: deleteCampaign,
  previewAudience,
  preview,
  sendTest,
  send,
  cancel,
  listRecipients
}

export default emailCampaignsAPI
