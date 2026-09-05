import { describe, expect, it } from 'vitest'

import {
  createVideoModelPricesForm,
  serializeVideoModelPrices,
  videoModelPriceFamilyRows,
  videoResolutionEnabledForFamily
} from '../groupsVideoModelPricing'

describe('Grok video model pricing form', () => {
  it('provides editable rows for both canonical Grok video families', () => {
    const form = createVideoModelPricesForm()

    expect(videoModelPriceFamilyRows(form).map(({ key }) => key)).toEqual([
      'grok-imagine-video',
      'grok-imagine-video-1.5'
    ])
    expect(form['grok-imagine-video']['480p']).toBeNull()
    expect(form['grok-imagine-video-1.5']['1080p']).toBeNull()
  })

  it('serializes only finite non-negative prices and preserves future families', () => {
    const form = createVideoModelPricesForm({
      'grok-imagine-video-2': { '1080p': 0.4 }
    })
    form['grok-imagine-video']['480p'] = 0.05
    form['grok-imagine-video']['720p'] = ''
    form['grok-imagine-video-1.5']['1080p'] = -1

    expect(serializeVideoModelPrices(form)).toEqual({
      'grok-imagine-video': { '480p': 0.05 },
      'grok-imagine-video-2': { '1080p': 0.4 }
    })
  })

  it('round-trips unknown model families so editing does not discard them', () => {
    const form = createVideoModelPricesForm({
      'grok-imagine-video-2': { '480p': 0.2 }
    })

    expect(videoModelPriceFamilyRows(form).map(({ key }) => key)).toContain(
      'grok-imagine-video-2'
    )
    expect(serializeVideoModelPrices(form)).toMatchObject({
      'grok-imagine-video-2': { '480p': 0.2 }
    })
  })
})

describe('Drama video model pricing form', () => {
  it('exposes the 12 public families and a 4K column', () => {
    const form = createVideoModelPricesForm(null, 'drama')
    expect(videoModelPriceFamilyRows(form, 'drama').map(({ key }) => key)).toEqual([
      'minimax-h3',
      'seedance2.0-A',
      'seedance2.0-fast-A',
      'seedance2.0-Mini-A',
      'seedance2.0-B',
      'seedance2.0-fast-B',
      'seedance-2.0-C',
      'seedance2.0-E',
      'seedance2.0-F',
      'seedance2.0-fast-F',
      'seedance2.5-A',
      'seedance-2.5-B'
    ])
    expect(form['seedance2.0-B']['4k']).toBeNull()
    expect(form['seedance2.0-F']['1080p']).toBeNull()
  })

  it('enables 4K only for seedance2.0-B', () => {
    expect(videoResolutionEnabledForFamily('seedance2.0-B', '4k', 'drama')).toBe(true)
    expect(videoResolutionEnabledForFamily('seedance2.0-F', '4k', 'drama')).toBe(false)
    expect(videoResolutionEnabledForFamily('seedance2.0-F', '1080p', 'drama')).toBe(true)
  })
})
