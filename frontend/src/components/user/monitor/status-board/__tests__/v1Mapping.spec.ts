import { describe, expect, it } from 'vitest'
import type { UserMonitorView } from '@/api/channelMonitor'
import type { Group } from '@/types'
import { buildV1Cards, v1AvgLatencyMs, v1Bars, v1Status, V1_BAR_COUNT } from '../v1Mapping'

const t = (key: string, params?: Record<string, unknown>) =>
  params ? `${key}(${Object.values(params).join(',')})` : key

function view(partial: Partial<UserMonitorView> = {}): UserMonitorView {
  return {
    id: 1,
    name: 'Claude 主线',
    provider: 'anthropic',
    group_name: '',
    group_ids: [3],
    primary_model: 'claude-sonnet',
    primary_status: 'operational',
    primary_latency_ms: 900,
    primary_ping_latency_ms: 50,
    availability_7d: 99.5,
    extra_models: [{ model: 'claude-haiku', status: 'operational', latency_ms: 300 }],
    timeline: [
      { status: 'operational', latency_ms: 1000, ping_latency_ms: 40, checked_at: '2026-11-19T06:00:00Z' },
      { status: 'failed', latency_ms: null, ping_latency_ms: null, checked_at: '2026-11-19T05:59:00Z' },
      { status: 'degraded', latency_ms: 2000, ping_latency_ms: 60, checked_at: '2026-11-19T05:58:00Z' },
    ],
    ...partial,
  } as UserMonitorView
}

describe('channel monitor v1 status board mapping', () => {
  it('maps probe statuses', () => {
    expect(v1Status('operational')).toBe('operational')
    expect(v1Status('error')).toBe('failed')
    expect(v1Status('')).toBe('unknown')
  })

  it('averages timeline latency and ignores failed probes', () => {
    expect(v1AvgLatencyMs(view())).toBe(1500)
    expect(v1AvgLatencyMs(view({ timeline: [] }))).toBe(900)
  })

  it('renders oldest-to-newest bars padded to a stable count', () => {
    const bars = v1Bars(view(), t)
    expect(bars).toHaveLength(V1_BAR_COUNT)
    expect(bars.slice(-3).map((b) => b.status)).toEqual(['degraded', 'failed', 'operational'])
    expect(bars[0].status).toBe('unknown')
  })

  it('builds cards with group rate, availability and a cache placeholder', () => {
    const group = { id: 3, name: 'Claude Max', rate_multiplier: 0.2 } as Group
    const [card] = buildV1Cards([view()], {
      t,
      windowLabel: '7d',
      availability: (item) => item.availability_7d,
      groups: () => [group],
      userGroupRates: { 3: 0.15 },
      platformLabel: () => 'Anthropic',
      modelLabel: (model) => model,
    })
    expect(card.rate).toBe(0.15)
    expect(card.baseRate).toBe(0.2)
    expect(card.metrics.map((m) => `${m.value}${m.unit ?? ''}`)).toEqual(['1.50s', '99.50%', '—'])
    expect(card.sort.cache).toBeNull()
    expect(card.searchText).toContain('claude-haiku')
    expect(card.searchText).toContain('claude max')
  })
})
