/**
 * Maps channel-monitor-v2 matrix rows (group_by=platform_group) into the shared
 * status-board card model. Pure functions so they can be unit-tested.
 */

import type {
  MonitorCoverage,
  MonitorHealth,
  MonitorMatrixBucket,
  MonitorMatrixRow,
  MonitorMetric,
} from '@/api/channelMonitorV2'
import type { BoardBar, BoardStatus, StatusCardModel } from '@/components/user/monitor/status-board/types'
import { splitLatency, splitRatioPercent } from '@/components/user/monitor/status-board/format'

type Translate = (key: string, params?: Record<string, unknown>) => string

export interface GroupRateInfo {
  base: number
  user: number | null
}

export interface V2CardContext {
  t: Translate
  rates: Record<number, GroupRateInfo>
  platformLabel: (platform: string) => string
}

const DEFAULT_WARNING_ERROR_RATE = 0.05
const DEFAULT_CRITICAL_ERROR_RATE = 0.2

/**
 * Health → card status. Below the minimum sample the backend reports `unknown`;
 * when there is traffic anyway, fall back to the error-rate thresholds so a
 * quiet-but-working group does not render as "no data".
 */
export function v2Status(health: MonitorHealth | undefined, metrics: MonitorMetric | undefined): BoardStatus {
  if (!metrics || metrics.request_count <= 0) return 'unknown'
  switch (health?.overall) {
    case 'healthy':
      return 'operational'
    case 'warning':
      return 'degraded'
    case 'critical':
      return 'failed'
  }
  const warning = health?.thresholds?.warning_error_rate ?? DEFAULT_WARNING_ERROR_RATE
  const critical = health?.thresholds?.critical_error_rate ?? DEFAULT_CRITICAL_ERROR_RATE
  const errorRate = metrics.error_rate || 0
  if (errorRate >= critical) return 'failed'
  if (errorRate >= warning) return 'degraded'
  return 'operational'
}

export function v2AvgLatencyMs(metrics: MonitorMetric | undefined): number | null {
  if (!metrics) return null
  return metrics.ttft.avg_ms ?? metrics.ttft.p50_ms ?? null
}

/** Success ratio, or null when the slot carried no traffic. */
export function v2SuccessRatio(metrics: MonitorMetric | undefined): number | null {
  if (!metrics || metrics.request_count <= 0) return null
  return 1 - (metrics.error_rate || 0)
}

export function v2CacheRatio(metrics: MonitorMetric | undefined): number | null {
  if (!metrics || metrics.request_count <= 0) return null
  if (!metrics.cache_rate_denominator && !metrics.cache_rate) return null
  return metrics.cache_rate
}

/** Bucket starts spanning the requested window (mirrors RelayPulseMatrix). */
export function v2BucketStarts(coverage: MonitorCoverage): number[] {
  const step = Math.max(60, coverage.bucket_seconds) * 1000
  const start = new Date(coverage.requested_start).getTime()
  const requestedEnd = coverage.requested_end ? new Date(coverage.requested_end).getTime() : NaN
  const end = Number.isFinite(requestedEnd) && requestedEnd > start
    ? requestedEnd
    : new Date(coverage.data_through).getTime()
  if (!Number.isFinite(start) || !Number.isFinite(end) || start >= end) return []
  const starts: number[] = []
  for (let cursor = Math.floor(start / step) * step; cursor < end; cursor += step) starts.push(cursor)
  return starts
}

const pad = (n: number) => String(n).padStart(2, '0')
const fmtDay = (d: Date) => `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
const fmtTime = (d: Date) => `${pad(d.getHours())}:${pad(d.getMinutes())}`

/** "11-19 13:40 – 14:40"; day-sized buckets collapse to "11-19". */
export function formatBucketWindow(startMs: number, bucketSeconds: number): string {
  const start = new Date(startMs)
  const end = new Date(startMs + bucketSeconds * 1000)
  if (bucketSeconds >= 86400) return fmtDay(start)
  const sameDay = fmtDay(start) === fmtDay(end) || (end.getHours() === 0 && end.getMinutes() === 0)
  return `${fmtDay(start)} ${fmtTime(start)} – ${sameDay ? fmtTime(end) : `${fmtDay(end)} ${fmtTime(end)}`}`
}

function joinSplit(part: { value: string; unit?: string }): string {
  return part.unit ? `${part.value}${part.unit}` : part.value
}

export function v2Bars(row: MonitorMatrixRow, coverage: MonitorCoverage, t: Translate): BoardBar[] {
  const byStart = new Map<number, MonitorMatrixBucket>()
  for (const bucket of row.buckets || []) byStart.set(new Date(bucket.bucket_start).getTime(), bucket)
  return v2BucketStarts(coverage).map((start) => {
    const bucket = byStart.get(start)
    const status = v2Status(bucket?.health, bucket?.metrics)
    const head = `${formatBucketWindow(start, coverage.bucket_seconds)} · ${t(`channelStatusBoard.status.${status}`)}`
    if (!bucket || status === 'unknown') {
      return { status, lines: [head, t('channelStatusBoard.tooltip.noTraffic')] }
    }
    const lines = [
      head,
      `${t('channelStatusBoard.tooltip.successRate', { value: joinSplit(splitRatioPercent(v2SuccessRatio(bucket.metrics))) })} · ${t('channelStatusBoard.tooltip.avgLatency', { value: joinSplit(splitLatency(v2AvgLatencyMs(bucket.metrics))) })}`,
      t('channelStatusBoard.tooltip.cacheRate', { value: joinSplit(splitRatioPercent(v2CacheRatio(bucket.metrics))) }),
    ]
    return { status, lines }
  })
}

export function v2BarsLabel(bucketSeconds: number, count: number, t: Translate): string {
  const totalHours = Math.round((bucketSeconds * count) / 3600)
  if (totalHours <= 48) return t('channelStatusBoard.recentHours', { n: totalHours })
  return t('channelStatusBoard.recentDays', { n: Math.round(totalHours / 24) })
}

export function buildV2Cards(
  rows: MonitorMatrixRow[],
  coverage: MonitorCoverage,
  ctx: V2CardContext,
): StatusCardModel[] {
  const { t } = ctx
  return rows
    .filter((row) => row.group_id != null && Number(row.group_id) > 0)
    .map((row) => {
      const groupId = Number(row.group_id)
      const title = row.group_name || `#${groupId}`
      const rateInfo = ctx.rates[groupId]
      const rate = rateInfo ? (rateInfo.user ?? rateInfo.base) : null
      const latencyMs = v2AvgLatencyMs(row.metrics)
      const success = v2SuccessRatio(row.metrics)
      const cache = v2CacheRatio(row.metrics)
      const bars = v2Bars(row, coverage, t)
      const platformLabel = ctx.platformLabel(row.platform)
      return {
        key: `${row.platform}:${groupId}`,
        title,
        platform: row.platform,
        platformLabel,
        rate,
        baseRate: rateInfo ? rateInfo.base : null,
        rateTitle: rateInfo
          ? rateInfo.user != null && rateInfo.user !== rateInfo.base
            ? t('channelStatusBoard.rateTitleCustom', { base: rateInfo.base, user: rateInfo.user })
            : t('channelStatusBoard.rateTitle', { base: rateInfo.base })
          : undefined,
        status: v2Status(row.health, row.metrics),
        metrics: [
          { label: t('channelStatusBoard.metrics.avgLatency'), ...splitLatency(latencyMs), hint: t('channelStatusBoard.metrics.ttftAvgHint') },
          { label: t('channelStatusBoard.metrics.successRate'), ...splitRatioPercent(success), hint: v2BarsLabel(coverage.bucket_seconds, bars.length, t) },
          { label: t('channelStatusBoard.metrics.cacheRate'), ...splitRatioPercent(cache), hint: v2BarsLabel(coverage.bucket_seconds, bars.length, t) },
        ],
        bars,
        barsLabel: v2BarsLabel(coverage.bucket_seconds, bars.length, t),
        sort: { rate, success, cache, latency: latencyMs },
        searchText: [title, row.platform, platformLabel].join(' ').toLowerCase(),
      }
    })
}
