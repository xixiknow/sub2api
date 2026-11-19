import { describe, expect, it } from 'vitest'
import type { MonitorCoverage, MonitorHealth, MonitorMatrixRow, MonitorMetric } from '@/api/channelMonitorV2'
import { buildV2Cards, formatBucketWindow, v2BucketStarts, v2Status } from '../statusBoardMapping'

const t = (key: string, params?: Record<string, unknown>) =>
  params ? `${key}(${Object.values(params).join(',')})` : key

function metric(partial: Partial<MonitorMetric> = {}): MonitorMetric {
  return {
    success_requests: 99,
    error_requests: 1,
    request_count: 100,
    token_count: 0,
    rpm: 0,
    tpm: 0,
    error_rate: 0.01,
    cache_rate: 0.83,
    cache_rate_numerator: 83,
    cache_rate_denominator: 100,
    ttft: { sample_count: 100, p50_ms: 5000, p95_ms: 9000, avg_ms: 6790 },
    duration: { sample_count: 100, p50_ms: 1, p95_ms: 1, avg_ms: 1 },
    ...partial,
  }
}

const health = (overall: MonitorHealth['overall']): MonitorHealth => ({
  overall,
  error_rate: overall,
  ttft: overall,
  minimum_sample: 20,
})

const coverage: MonitorCoverage = {
  requested_start: '2026-11-18T15:00:00.000Z',
  requested_end: '2026-11-19T15:00:00.000Z',
  coverage_start: '2026-11-18T15:00:00.000Z',
  data_through: '2026-11-19T14:59:00.000Z',
  computed_at: '2026-11-19T14:59:00.000Z',
  aggregation_lag_seconds: 0,
  coverage_complete: true,
  bucket_seconds: 3600,
}

describe('channel monitor v2 status board mapping', () => {
  it('maps coarse health and falls back to error-rate thresholds below the sample floor', () => {
    expect(v2Status(health('healthy'), metric())).toBe('operational')
    expect(v2Status(health('critical'), metric())).toBe('failed')
    expect(v2Status(health('unknown'), metric({ error_rate: 0.01 }))).toBe('operational')
    expect(v2Status(health('unknown'), metric({ error_rate: 0.1 }))).toBe('degraded')
    expect(v2Status(health('unknown'), metric({ error_rate: 0.5 }))).toBe('failed')
    expect(v2Status(health('healthy'), metric({ request_count: 0 }))).toBe('unknown')
  })

  it('spans the requested window with one slot per bucket', () => {
    expect(v2BucketStarts(coverage)).toHaveLength(24)
  })

  it('formats hour and day bucket windows', () => {
    const start = new Date(2026, 10, 19, 13, 40).getTime()
    expect(formatBucketWindow(start, 3600)).toBe('11-19 13:40 – 14:40')
    expect(formatBucketWindow(new Date(2026, 10, 19).getTime(), 86400)).toBe('11-19')
  })

  it('builds one card per real group with rates, metrics and aligned bars', () => {
    const rows: MonitorMatrixRow[] = [
      {
        platform: 'openai',
        group_id: 7,
        group_name: 'GPT Plus 福利池',
        metrics: metric(),
        health: health('healthy'),
        buckets: [{ bucket_start: '2026-11-19T14:00:00Z', metrics: metric(), health: health('healthy') }],
      },
      { platform: 'openai', metrics: metric(), health: health('healthy'), buckets: [] },
    ]
    const [card, ...rest] = buildV2Cards(rows, coverage, {
      t,
      rates: { 7: { base: 0.05, user: 0.039 } },
      platformLabel: () => 'OpenAI',
    })
    expect(rest).toHaveLength(0)
    expect(card.title).toBe('GPT Plus 福利池')
    expect(card.rate).toBe(0.039)
    expect(card.baseRate).toBe(0.05)
    expect(card.metrics.map((m) => `${m.value}${m.unit ?? ''}`)).toEqual(['6.79s', '99.00%', '83.00%'])
    expect(card.bars).toHaveLength(24)
    expect(card.bars[23].status).toBe('operational')
    expect(card.bars[0].status).toBe('unknown')
    expect(card.sort).toEqual({ rate: 0.039, success: 0.99, cache: 0.83, latency: 6790 })
  })
})
