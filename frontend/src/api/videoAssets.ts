import { apiClient } from './client'
import type { VideoStudioAsset, VideoStudioAssetKind } from './videoStudio'

export interface VideoWorkbenchMeta {
  output_retention_days: number
  asset_retention_days: number
  asset_quota_bytes: number
}

export interface VideoAssetRecord {
  id: string
  kind: VideoStudioAssetKind
  mime: string
  bytes: number
  original_name: string
  created_at: number
  last_used_at: number
  preview_path: string
  user_id?: number
}

export interface VideoAssetList {
  items: VideoAssetRecord[]
  used_bytes: number
  quota_bytes: number
  retention_days: number
}

export interface VideoAssetAdminList {
  items: Array<VideoAssetRecord & { user_id: number }>
  total: number
  used_bytes: number
}

export async function getVideoWorkbenchMeta(): Promise<VideoWorkbenchMeta> {
  const { data } = await apiClient.get<VideoWorkbenchMeta>('/video-workbench-meta')
  return data
}

export async function listVideoAssets(kind?: string): Promise<VideoAssetList> {
  const { data } = await apiClient.get<VideoAssetList>('/video-assets', {
    params: kind ? { kind } : undefined
  })
  return data
}

export async function uploadVideoAsset(file: File): Promise<VideoAssetRecord> {
  const form = new FormData()
  form.append('file', file)
  const { data } = await apiClient.post<VideoAssetRecord>('/video-assets', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 120000
  })
  return data
}

export async function deleteVideoAsset(id: string): Promise<void> {
  await apiClient.delete(`/video-assets/${encodeURIComponent(id)}`)
}

export async function getVideoAssetContent(id: string): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/video-assets/${encodeURIComponent(id)}/content`, {
    responseType: 'blob'
  })
  return data
}

export async function adminListVideoAssets(params: {
  user_id?: number
  kind?: string
  limit?: number
  offset?: number
}): Promise<VideoAssetAdminList> {
  const { data } = await apiClient.get<VideoAssetAdminList>('/admin/video-assets', { params })
  return data
}

export async function adminDeleteVideoAsset(id: string): Promise<void> {
  await apiClient.delete(`/admin/video-assets/${encodeURIComponent(id)}`)
}

export function videoAssetToStudio(record: VideoAssetRecord, role: 'reference' | 'first_frame' | 'last_frame' = 'reference'): VideoStudioAsset {
  return {
    id: record.id,
    kind: record.kind,
    role,
    name: record.original_name || record.id,
    source: `asset://${record.id}`,
    previewUrl: record.preview_path
  }
}
