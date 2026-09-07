/**
 * User-facing video model catalog (prices + capabilities).
 */
import { apiClient } from './client'

export type VideoBillingUnit = 'per_second' | 'per_clip'

export interface VideoCatalogPrice {
  resolution: string
  price: number
}

export interface VideoCatalogFamily {
  family: string
  billing_unit: VideoBillingUnit
  create_path: string
  min_duration: number
  max_duration: number
  duration_required: boolean
  fixed_duration?: number
  default_duration?: number
  default_resolution: string
  default_aspect: string
  aspect_ratios: string[]
  prices: VideoCatalogPrice[]
  max_images: number
  max_videos: number
  max_audios: number
  max_references: number
  max_video_audio?: number
  allow_video: boolean
  allow_audio: boolean
  allow_first_last: boolean
  allow_generate_audio: boolean
  allow_s_optional: boolean
  first_last_requires_auto: boolean
  require_a_placeholders: boolean
  require_image_placeholders: boolean
  tags?: string[]
}

export interface VideoCatalog {
  families: VideoCatalogFamily[]
}

export async function getVideoCatalog(options?: { signal?: AbortSignal }): Promise<VideoCatalog> {
  const { data } = await apiClient.get<VideoCatalog>('/video-catalog', {
    signal: options?.signal
  })
  return data
}
