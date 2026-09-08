import { describe, expect, it } from 'vitest'
import type { VideoCatalogFamily } from '@/api/videoCatalog'
import {
  buildCreatePayload,
  clampDuration,
  durationPresets,
  estimateVideoCost,
  inferTaskMode,
  isVideoStudioKeyPlatform,
  missingPlaceholders,
  placeholderToken,
  taskErrorMessage
} from '@/api/videoStudio'

const familyA: VideoCatalogFamily = {
  family: 'seedance2.0-A',
  billing_unit: 'per_second',
  create_path: '/v1/videos',
  min_duration: 4,
  max_duration: 15,
  duration_required: false,
  default_duration: 4,
  default_resolution: '720p',
  default_aspect: '16:9',
  aspect_ratios: ['16:9', 'auto'],
  prices: [{ resolution: '720p', price: 1 }],
  max_images: 9,
  max_videos: 3,
  max_audios: 3,
  max_references: 12,
  allow_video: true,
  allow_audio: true,
  allow_first_last: true,
  allow_generate_audio: false,
  allow_s_optional: false,
  first_last_requires_auto: true,
  require_a_placeholders: true,
  require_image_placeholders: false
}

describe('video studio helpers', () => {
  it('accepts drama and composite keys', () => {
    expect(isVideoStudioKeyPlatform('drama')).toBe(true)
    expect(isVideoStudioKeyPlatform('composite')).toBe(true)
    expect(isVideoStudioKeyPlatform('gemini')).toBe(false)
  })

  it('builds A-series placeholders', () => {
    expect(placeholderToken(familyA, 'image', 0)).toBe('@图1')
    expect(placeholderToken(familyA, 'video', 1)).toBe('@视频2')
    expect(missingPlaceholders(familyA, 'hello @图1', { image: 2, video: 0, audio: 0 })).toEqual(['@图2'])
  })

  it('builds C-series image placeholders', () => {
    const familyC = { ...familyA, family: 'seedance-2.0-C', require_a_placeholders: false, require_image_placeholders: true }
    expect(placeholderToken(familyC, 'image', 0)).toBe('@Image1')
    expect(missingPlaceholders(familyC, 'walk', { image: 1, video: 0, audio: 0 })).toEqual(['@Image1'])
  })

  it('infers B-series task mode', () => {
    expect(inferTaskMode({ allowFirstLast: true, firstLast: false, hasRefs: false })).toBe('text')
    expect(inferTaskMode({ allowFirstLast: true, firstLast: true, hasRefs: false })).toBe('first_last_frame')
    expect(inferTaskMode({ allowFirstLast: true, firstLast: false, hasRefs: true })).toBe('references')
  })

  it('reads task error messages', () => {
    expect(taskErrorMessage(null)).toBe('')
    expect(taskErrorMessage('boom')).toBe('boom')
    expect(taskErrorMessage({ message: 'no account' })).toBe('no account')
  })

  it('sets B-series task_mode from assets', () => {
    const familyB = { ...familyA, family: 'seedance2.0-B', require_a_placeholders: false, first_last_requires_auto: false }
    const text = buildCreatePayload({
      family: familyB,
      prompt: 'walk',
      seconds: 5,
      resolution: '720p',
      aspectRatio: '16:9',
      generateAudio: false,
      references: []
    })
    expect(text.task_mode).toBe('text')
    const refs = buildCreatePayload({
      family: familyB,
      prompt: 'walk',
      seconds: 5,
      resolution: '720p',
      aspectRatio: '16:9',
      generateAudio: false,
      references: [{ id: '1', kind: 'image', role: 'reference', name: 'a.png', source: 'https://example.com/a.png' }]
    })
    expect(refs.task_mode).toBe('references')
  })

  it('locks aspect to auto for A-series first/last frames', () => {
    const payload = buildCreatePayload({
      family: familyA,
      prompt: '@图1 walk',
      seconds: 8,
      resolution: '720p',
      aspectRatio: '16:9',
      generateAudio: false,
      references: [],
      firstFrame: { id: '1', kind: 'image', role: 'first_frame', name: 'a.png', source: 'data:image/png;base64,aa' },
      lastFrame: { id: '2', kind: 'image', role: 'last_frame', name: 'b.png', source: 'data:image/png;base64,bb' }
    })
    expect(payload.aspect_ratio).toBe('auto')
    expect(payload.references).toHaveLength(2)
    expect(payload.references?.[0].role).toBe('first_frame')
  })

  it('keeps asset:// sources in the create payload', () => {
    const payload = buildCreatePayload({
      family: familyA,
      prompt: '@图1 walk',
      seconds: 8,
      resolution: '720p',
      aspectRatio: '16:9',
      generateAudio: false,
      references: [{ id: 'vidasset_1', kind: 'image', role: 'reference', name: 'a.png', source: 'asset://vidasset_1' }]
    })
    expect(payload.references?.[0].source).toBe('asset://vidasset_1')
  })

  it('builds duration presets within the model range', () => {
    expect(durationPresets(4, 15)).toEqual([4, 5, 6, 8, 10, 12, 15])
    expect(durationPresets(5, 15)).toEqual([5, 6, 8, 10, 12, 15])
    expect(durationPresets(30, 30)).toEqual([30])
  })

  it('clamps duration to the model range', () => {
    expect(clampDuration(1, 4, 15)).toBe(4)
    expect(clampDuration(20, 4, 15)).toBe(15)
    expect(clampDuration(8.4, 4, 15)).toBe(8)
    expect(clampDuration(9, 4, 15, 30)).toBe(30)
  })

  it('estimates per-second and per-clip cost', () => {
    expect(estimateVideoCost(familyA, 8, '720p')).toEqual({
      unitPrice: 1,
      total: 8,
      seconds: 8,
      resolution: '720p',
      billingUnit: 'per_second'
    })
    expect(estimateVideoCost(familyA, 3, '720p')).toBeNull()
    const familyB = { ...familyA, billing_unit: 'per_clip' as const, min_duration: 5, max_duration: 15, prices: [{ resolution: '1080p', price: 2.5 }] }
    expect(estimateVideoCost(familyB, 8, '1080p')).toEqual({
      unitPrice: 2.5,
      total: 2.5,
      seconds: 8,
      resolution: '1080p',
      billingUnit: 'per_clip'
    })
  })
})
