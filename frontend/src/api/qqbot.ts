import { apiClient } from './client'

export interface QQBotBinding {
  enabled: boolean
  bound: boolean
  openid?: string
  created_at?: string
}

export interface QQBotConfig {
  enabled: boolean
  app_id: string
  secret_configured: boolean
}

export interface QQBotConfigUpdate {
  enabled: boolean
  app_id: string
  app_secret?: string
}

export async function getMyQQBinding(): Promise<QQBotBinding> {
  return (await apiClient.get<QQBotBinding>('/user/qqbot-binding')).data
}

export async function unbindMyQQ(): Promise<void> {
  await apiClient.delete('/user/qqbot-binding')
}

export async function getQQBotConfig(): Promise<QQBotConfig> {
  return (await apiClient.get<QQBotConfig>('/admin/qqbot/config')).data
}

export async function updateQQBotConfig(payload: QQBotConfigUpdate): Promise<QQBotConfig> {
  return (await apiClient.put<QQBotConfig>('/admin/qqbot/config', payload)).data
}
