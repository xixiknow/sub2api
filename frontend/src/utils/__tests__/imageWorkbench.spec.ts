import { describe, expect, it } from 'vitest'

import type { ApiKey, Group } from '@/types'
import {
  classifyImageTier,
  estimateImageCost,
  keySupportsWorkbench,
  modelsForPlatform,
  resolveWorkbenchPlatform,
} from '../imageWorkbench'

function group(overrides: Partial<Group> = {}): Group {
  return {
    id: 1,
    platform: 'openai',
    rate_multiplier: 1.5,
    allow_image_generation: true,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    image_price_1k: 0.04,
    image_price_2k: 0.08,
    image_price_4k: null,
    ...overrides,
  } as Group
}

describe('imageWorkbench utils', () => {
  it('classifies size tiers like the backend', () => {
    expect(classifyImageTier('auto')).toBe('1K')
    expect(classifyImageTier('1024x1024')).toBe('1K')
    expect(classifyImageTier('1024x1536')).toBe('2K')
    expect(classifyImageTier('1536x1024')).toBe('2K')
    expect(classifyImageTier('2048x2048')).toBe('2K')
    expect(classifyImageTier('3840x2160')).toBe('4K')
  })

  it('estimates cost with group multiplier, user override and independent image rate', () => {
    expect(estimateImageCost(group(), '1024x1024', 2).total).toBeCloseTo(0.04 * 2 * 1.5)
    expect(estimateImageCost(group(), '1024x1024', 2, 0.5).total).toBeCloseTo(0.04)
    expect(estimateImageCost(group({ image_rate_independent: true, image_rate_multiplier: 2 }), '1536x1024', 1, 0.5).total).toBeCloseTo(0.16)
    expect(estimateImageCost(group(), '3840x2160', 1).total).toBeNull()
  })

  it('only accepts active keys in image-enabled openai/grok/composite groups', () => {
    const key = (g: Partial<Group>, status: ApiKey['status'] = 'active') => ({ status, group: group(g) }) as ApiKey
    expect(keySupportsWorkbench(key({}))).toBe(true)
    expect(keySupportsWorkbench(key({ platform: 'composite' }))).toBe(true)
    expect(keySupportsWorkbench(key({ platform: 'grok' }))).toBe(true)
    expect(keySupportsWorkbench(key({ platform: 'anthropic' }))).toBe(false)
    expect(keySupportsWorkbench(key({ platform: 'gemini' }))).toBe(false)
    expect(keySupportsWorkbench(key({ allow_image_generation: false }))).toBe(false)
    expect(keySupportsWorkbench(key({}, 'inactive'))).toBe(false)
  })

  it('filters models by platform and edit support', () => {
    expect(resolveWorkbenchPlatform(group({ platform: 'grok' }))).toBe('grok')
    const grokEdit = modelsForPlatform('grok', 'edit').map((m) => m.id)
    expect(grokEdit).toEqual(['grok-imagine-image-quality'])
    expect(modelsForPlatform('openai', 'generate').every((m) => m.platform === 'openai')).toBe(true)
  })
})
