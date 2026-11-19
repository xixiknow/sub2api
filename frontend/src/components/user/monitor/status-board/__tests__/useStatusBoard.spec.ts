import { describe, expect, it } from 'vitest'
import {
  boardPlatformTab,
  countBoardPlatforms,
  filterAndSortBoard,
  nextBoardSort,
} from '../useStatusBoard'
import { formatBoardRate, splitLatency, splitPercent, splitRatioPercent } from '../format'
import type { StatusCardModel } from '../types'

function card(partial: Partial<StatusCardModel> & { key: string }): StatusCardModel {
  return {
    title: partial.key,
    platform: 'openai',
    platformLabel: 'OpenAI',
    rate: null,
    baseRate: null,
    status: 'operational',
    metrics: [],
    bars: [],
    barsLabel: '',
    sort: { rate: null, success: null, cache: null, latency: null },
    searchText: partial.key.toLowerCase(),
    ...partial,
  }
}

describe('status board helpers', () => {
  it('maps platforms onto fixed tabs with aliases and an other bucket', () => {
    expect(boardPlatformTab('openai')).toBe('openai')
    expect(boardPlatformTab('xai')).toBe('grok')
    expect(boardPlatformTab('antigravity')).toBe('other')
    expect(boardPlatformTab('')).toBe('other')
  })

  it('counts every tab, including empty ones', () => {
    const counts = countBoardPlatforms([
      card({ key: 'a', platform: 'openai' }),
      card({ key: 'b', platform: 'anthropic' }),
      card({ key: 'c', platform: 'kimi' }),
    ])
    expect(counts).toMatchObject({ all: 3, openai: 1, anthropic: 1, gemini: 0, deepseek: 0, grok: 0, other: 1 })
  })

  it('filters by platform and keyword', () => {
    const cards = [
      card({ key: 'plus', platform: 'openai', searchText: 'gpt plus 福利池' }),
      card({ key: 'claude', platform: 'anthropic', searchText: 'claude max' }),
    ]
    expect(filterAndSortBoard(cards, 'anthropic', '', { key: 'default', dir: 'asc' }).map((c) => c.key)).toEqual(['claude'])
    expect(filterAndSortBoard(cards, 'all', '  福利 ', { key: 'default', dir: 'asc' }).map((c) => c.key)).toEqual(['plus'])
  })

  it('sorts with nulls last and stable ties', () => {
    const cards = [
      card({ key: 'a', sort: { rate: 1, success: null, cache: null, latency: 300 } }),
      card({ key: 'b', sort: { rate: null, success: null, cache: null, latency: 100 } }),
      card({ key: 'c', sort: { rate: 0.5, success: null, cache: null, latency: null } }),
      card({ key: 'd', sort: { rate: 0.5, success: null, cache: null, latency: 200 } }),
    ]
    const by = (key: 'rate' | 'latency', dir: 'asc' | 'desc') =>
      filterAndSortBoard(cards, 'all', '', { key, dir }).map((c) => c.key)
    expect(by('rate', 'asc')).toEqual(['c', 'd', 'a', 'b'])
    expect(by('rate', 'desc')).toEqual(['a', 'c', 'd', 'b'])
    expect(by('latency', 'asc')).toEqual(['b', 'd', 'a', 'c'])
  })

  it('sorts status from healthy to unknown', () => {
    const cards = [
      card({ key: 'x', status: 'unknown' }),
      card({ key: 'y', status: 'failed' }),
      card({ key: 'z', status: 'operational' }),
    ]
    expect(filterAndSortBoard(cards, 'all', '', { key: 'status', dir: 'asc' }).map((c) => c.key)).toEqual(['z', 'y', 'x'])
  })

  it('starts each key at its natural direction and flips on repeat', () => {
    expect(nextBoardSort({ key: 'default', dir: 'asc' }, 'success')).toEqual({ key: 'success', dir: 'desc' })
    expect(nextBoardSort({ key: 'success', dir: 'desc' }, 'success')).toEqual({ key: 'success', dir: 'asc' })
    expect(nextBoardSort({ key: 'success', dir: 'asc' }, 'latency')).toEqual({ key: 'latency', dir: 'asc' })
  })

  it('splits values for large-number rendering', () => {
    expect(splitLatency(6790)).toEqual({ value: '6.79', unit: 's' })
    expect(splitLatency(420.4)).toEqual({ value: '420', unit: 'ms' })
    expect(splitLatency(null)).toEqual({ value: '—' })
    expect(splitRatioPercent(0.99492)).toEqual({ value: '99.49', unit: '%' })
    expect(splitPercent(83.04)).toEqual({ value: '83.04', unit: '%' })
    expect(formatBoardRate(0.039)).toBe('0.039')
    expect(formatBoardRate(1)).toBe('1')
  })
})
