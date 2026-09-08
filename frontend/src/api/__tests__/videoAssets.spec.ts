import { describe, expect, it } from 'vitest'
import { videoAssetToStudio, type VideoAssetRecord } from '@/api/videoAssets'

describe('video assets helpers', () => {
  it('maps a library record to an asset:// studio source', () => {
    const record: VideoAssetRecord = {
      id: 'vidasset_1',
      kind: 'image',
      mime: 'image/png',
      bytes: 12,
      original_name: 'a.png',
      created_at: 1,
      last_used_at: 1,
      preview_path: '/api/v1/video-assets/vidasset_1/content'
    }
    expect(videoAssetToStudio(record)).toEqual({
      id: 'vidasset_1',
      kind: 'image',
      role: 'reference',
      name: 'a.png',
      source: 'asset://vidasset_1',
      previewUrl: '/api/v1/video-assets/vidasset_1/content'
    })
  })
})
