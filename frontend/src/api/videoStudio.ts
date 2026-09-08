import { buildGatewayUrl } from './client'
import type { VideoCatalogFamily } from './videoCatalog'

export const VIDEO_STUDIO_MAX_IMAGE_BYTES = 10 * 1024 * 1024
export const VIDEO_STUDIO_MAX_VIDEO_BYTES = 40 * 1024 * 1024
export const VIDEO_STUDIO_MAX_AUDIO_BYTES = 20 * 1024 * 1024

export type VideoStudioAssetKind = 'image' | 'video' | 'audio'
export type VideoStudioAssetRole = 'reference' | 'first_frame' | 'last_frame'

export interface VideoStudioAsset {
  id: string
  kind: VideoStudioAssetKind
  role: VideoStudioAssetRole
  name: string
  source: string
  previewUrl?: string
}

export interface VideoStudioTask {
  id: string
  task_id: string
  object: string
  model: string
  status: string
  progress: number
  error?: { code?: string; message?: string } | string | null
  created_at: number
  completed_at?: number | null
  expires_at?: number | null
  seconds?: string
  hold_amount?: number | null
  actual_cost?: number | null
  metadata?: {
    resolution?: string
    aspect_ratio?: string
  }
}

export interface VideoStudioListResponse {
  object: string
  data: VideoStudioTask[]
  has_more: boolean
}

export interface VideoStudioCreatePayload {
  model: string
  prompt: string
  seconds?: number
  resolution?: string
  aspect_ratio?: string
  generate_audio?: boolean
  task_mode?: string
  references?: Array<{
    type: VideoStudioAssetKind
    role: VideoStudioAssetRole
    source: string
  }>
}

function authHeaders(apiKey: string, extra?: HeadersInit): HeadersInit {
  return { Authorization: `Bearer ${apiKey}`, ...extra }
}

async function parseVideoStudioError(response: Response): Promise<Error> {
  try {
    const body = await response.json()
    const message = body?.error?.message || body?.message || response.statusText
    const error = new Error(message)
    ;(error as Error & { code?: string; status?: number }).code = body?.error?.code || String(response.status)
    ;(error as Error & { code?: string; status?: number }).status = response.status
    return error
  } catch {
    return new Error(response.statusText || `HTTP ${response.status}`)
  }
}

export function isVideoStudioKeyPlatform(platform?: string): boolean {
  return platform === 'drama' || platform === 'composite'
}

const PREFERRED_DURATIONS = [4, 5, 6, 8, 10, 12, 15, 20, 25, 30]

export function durationPresets(min: number, max: number): number[] {
  if (!Number.isFinite(min) || !Number.isFinite(max) || max < min) return []
  if (min === max) return [min]
  const values = new Set<number>()
  for (const n of PREFERRED_DURATIONS) {
    if (n >= min && n <= max) values.add(n)
  }
  values.add(min)
  values.add(max)
  return [...values].sort((a, b) => a - b)
}

export function clampDuration(value: number, min: number, max: number, fixed?: number): number {
  if (fixed) return fixed
  const lo = Number.isFinite(min) ? min : 1
  const hi = Number.isFinite(max) ? max : lo
  if (!Number.isFinite(value)) return lo
  return Math.min(hi, Math.max(lo, Math.round(value)))
}

export function isDurationInRange(value: number, min: number, max: number): boolean {
  return Number.isFinite(value) && Number.isInteger(value) && value >= min && value <= max
}

export interface VideoCostEstimate {
  unitPrice: number
  total: number
  seconds: number
  resolution: string
  billingUnit: VideoCatalogFamily['billing_unit']
}

export function estimateVideoCost(
  family: Pick<VideoCatalogFamily, 'billing_unit' | 'prices' | 'fixed_duration' | 'min_duration' | 'max_duration'>,
  seconds: number,
  resolution: string
): VideoCostEstimate | null {
  const duration = family.fixed_duration || seconds
  if (!isDurationInRange(duration, family.min_duration, family.max_duration)) return null
  const priceRow = family.prices.find((item) => item.resolution === resolution) || family.prices[0]
  if (!priceRow || !Number.isFinite(priceRow.price)) return null
  const total = family.billing_unit === 'per_second' ? priceRow.price * duration : priceRow.price
  return {
    unitPrice: priceRow.price,
    total,
    seconds: duration,
    resolution: priceRow.resolution,
    billingUnit: family.billing_unit
  }
}

export function maxBytesForKind(kind: VideoStudioAssetKind): number {
  if (kind === 'video') return VIDEO_STUDIO_MAX_VIDEO_BYTES
  if (kind === 'audio') return VIDEO_STUDIO_MAX_AUDIO_BYTES
  return VIDEO_STUDIO_MAX_IMAGE_BYTES
}

export function placeholderToken(family: Pick<VideoCatalogFamily, 'require_image_placeholders' | 'require_a_placeholders'>, kind: VideoStudioAssetKind, index: number): string {
  const n = index + 1
  if (family.require_image_placeholders && kind === 'image') return `@Image${n}`
  if (family.require_a_placeholders) {
    if (kind === 'image') return `@图${n}`
    if (kind === 'video') return `@视频${n}`
    return `@音频${n}`
  }
  if (kind === 'image') return `@图${n}`
  if (kind === 'video') return `@视频${n}`
  return `@音频${n}`
}

export function missingPlaceholders(
  family: Pick<VideoCatalogFamily, 'require_image_placeholders' | 'require_a_placeholders'>,
  prompt: string,
  counts: { image: number; video: number; audio: number }
): string[] {
  const missing: string[] = []
  const check = (kind: VideoStudioAssetKind, count: number) => {
    for (let i = 0; i < count; i++) {
      const token = placeholderToken(family, kind, i)
      if (!prompt.includes(token)) missing.push(token)
    }
  }
  if (family.require_image_placeholders) {
    check('image', counts.image)
  }
  if (family.require_a_placeholders) {
    check('image', counts.image)
    check('video', counts.video)
    check('audio', counts.audio)
  }
  return missing
}

export function inferTaskMode(args: {
  allowFirstLast: boolean
  firstLast: boolean
  hasRefs: boolean
}): string | undefined {
  if (args.firstLast) return 'first_last_frame'
  if (args.hasRefs) return 'references'
  return 'text'
}

export function buildCreatePayload(input: {
  family: VideoCatalogFamily
  prompt: string
  seconds: number
  resolution: string
  aspectRatio: string
  generateAudio: boolean
  references: VideoStudioAsset[]
  firstFrame?: VideoStudioAsset | null
  lastFrame?: VideoStudioAsset | null
}): VideoStudioCreatePayload {
  const payload: VideoStudioCreatePayload = {
    model: input.family.family,
    prompt: input.prompt.trim()
  }
  if (input.family.fixed_duration) {
    payload.seconds = input.family.fixed_duration
  } else if (input.seconds > 0) {
    payload.seconds = input.seconds
  }
  if (input.resolution) payload.resolution = input.resolution
  const aspect = input.family.first_last_requires_auto && input.firstFrame && input.lastFrame
    ? 'auto'
    : input.aspectRatio
  if (aspect) payload.aspect_ratio = aspect
  if (input.family.allow_generate_audio) payload.generate_audio = input.generateAudio

  const refs: VideoStudioCreatePayload['references'] = []
  if (input.firstFrame && input.lastFrame) {
    refs.push(
      { type: 'image', role: 'first_frame', source: input.firstFrame.source },
      { type: 'image', role: 'last_frame', source: input.lastFrame.source }
    )
  } else {
    for (const asset of input.references) {
      refs.push({ type: asset.kind, role: 'reference', source: asset.source })
    }
  }
  if (refs.length) payload.references = refs

  const familyName = input.family.family
  if (familyName.includes('2.0-B') || familyName.includes('fast-B')) {
    payload.task_mode = inferTaskMode({
      allowFirstLast: input.family.allow_first_last,
      firstLast: Boolean(input.firstFrame && input.lastFrame),
      hasRefs: (payload.references || []).some((item) => item.role === 'reference')
    })
  }
  return payload
}

export async function listVideoStudioTasks(apiKey: string, limit = 50, offset = 0): Promise<VideoStudioListResponse> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
  const response = await fetch(buildGatewayUrl(`/v1/videos?${params.toString()}`), {
    headers: authHeaders(apiKey)
  })
  if (!response.ok) throw await parseVideoStudioError(response)
  return response.json()
}

export async function createVideoStudioTask(apiKey: string, createPath: string, payload: VideoStudioCreatePayload): Promise<VideoStudioTask> {
  const path = createPath.startsWith('/v1/') ? createPath : `/v1${createPath.startsWith('/') ? '' : '/'}${createPath}`
  const response = await fetch(buildGatewayUrl(path), {
    method: 'POST',
    headers: authHeaders(apiKey, { 'Content-Type': 'application/json' }),
    body: JSON.stringify(payload)
  })
  if (!response.ok) throw await parseVideoStudioError(response)
  return response.json()
}

export async function getVideoStudioTask(apiKey: string, id: string): Promise<VideoStudioTask> {
  const response = await fetch(buildGatewayUrl(`/v1/videos/${encodeURIComponent(id)}`), {
    headers: authHeaders(apiKey)
  })
  if (!response.ok) throw await parseVideoStudioError(response)
  return response.json()
}

export async function getVideoStudioContent(apiKey: string, id: string): Promise<Blob> {
  const response = await fetch(buildGatewayUrl(`/v1/videos/${encodeURIComponent(id)}/content`), {
    headers: authHeaders(apiKey)
  })
  if (!response.ok) throw await parseVideoStudioError(response)
  return response.blob()
}

export function taskErrorMessage(error: VideoStudioTask['error']): string {
  if (!error) return ''
  if (typeof error === 'string') return error
  return String(error.message || error.code || '').trim()
}

export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

export function fileToDataURI(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(new Error('failed to read file'))
    reader.readAsDataURL(file)
  })
}
