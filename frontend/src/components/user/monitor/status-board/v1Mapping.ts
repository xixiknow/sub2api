/**
 * Maps V1 (active probe) channel monitor views into the shared status-board
 * card model. V1 has no cache statistics yet, so that metric renders as "—".
 */

import type { UserMonitorView } from '@/api/channelMonitor'
import type { Group } from '@/types'
import { BOARD_EMPTY, splitLatency, splitPercent } from './format'
import type { BoardBar, BoardStatus, StatusCardModel } from './types'

type Translate = (key: string, params?: Record<string, unknown>) => string

export const V1_BAR_COUNT = 30

export interface V1CardContext {
  t: Translate
  windowLabel: string
  availability: (item: UserMonitorView) => number | null
  groups: (item: UserMonitorView) => Group[]
  userGroupRates: Record<number, number>
  platformLabel: (provider: string) => string
  modelLabel: (model: string) => string
}

export function v1Status(status: string | null | undefined): BoardStatus {
  switch (status) {
    case 'operational':
      return 'operational'
    case 'degraded':
      return 'degraded'
    case 'failed':
    case 'error':
      return 'failed'
    default:
      return 'unknown'
  }
}

export function v1AvgLatencyMs(item: UserMonitorView, limit = V1_BAR_COUNT): number | null {
  const samples = (item.timeline || [])
    .slice(0, limit)
    .map((point) => point.latency_ms)
    .filter((ms): ms is number => ms != null && Number.isFinite(ms))
  if (samples.length === 0) return item.primary_latency_ms ?? null
  return samples.reduce((sum, ms) => sum + ms, 0) / samples.length
}

const pad = (n: number) => String(n).padStart(2, '0')
function formatCheckedAt(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function joinSplit(part: { value: string; unit?: string }): string {
  return part.unit ? `${part.value}${part.unit}` : part.value
}

/** Timeline points arrive newest-first; bars render oldest → newest, left-padded. */
export function v1Bars(item: UserMonitorView, t: Translate, count = V1_BAR_COUNT): BoardBar[] {
  const points = [...(item.timeline || [])].slice(0, count).reverse()
  const bars: BoardBar[] = []
  for (let i = points.length; i < count; i += 1) {
    bars.push({ status: 'unknown', lines: [t('channelStatusBoard.noHistory')] })
  }
  for (const point of points) {
    const status = v1Status(point.status)
    const lines = [`${formatCheckedAt(point.checked_at)} · ${t(`channelStatusBoard.status.${status}`)}`]
    const detail = [t('channelStatusBoard.tooltip.latency', { value: joinSplit(splitLatency(point.latency_ms)) })]
    if (point.ping_latency_ms != null) {
      detail.push(t('channelStatusBoard.tooltip.ping', { value: joinSplit(splitLatency(point.ping_latency_ms)) }))
    }
    lines.push(detail.join(' · '))
    bars.push({ status, lines })
  }
  return bars
}

export function buildV1Cards(items: UserMonitorView[], ctx: V1CardContext): StatusCardModel[] {
  const { t } = ctx
  return items.map((item) => {
    const groups = ctx.groups(item)
    const primaryGroup = groups[0]
    const userRate = primaryGroup ? ctx.userGroupRates[primaryGroup.id] ?? null : null
    const baseRate = primaryGroup?.rate_multiplier ?? null
    const rate = userRate ?? baseRate
    const latencyMs = v1AvgLatencyMs(item)
    const availability = ctx.availability(item)
    const platformLabel = ctx.platformLabel(item.provider)
    const models = [item.primary_model, ...(item.extra_models || []).map((m) => m.model)].filter(Boolean)
    const rateTitle = groups.length
      ? groups
          .map((group) => {
            const own = ctx.userGroupRates[group.id]
            return own != null && own !== group.rate_multiplier
              ? `${group.name} x${own} (x${group.rate_multiplier})`
              : `${group.name} x${group.rate_multiplier}`
          })
          .join('\n')
      : undefined
    return {
      key: String(item.id),
      title: item.name,
      platform: item.provider,
      platformLabel: [platformLabel, ctx.modelLabel(item.primary_model)].filter(Boolean).join(' · '),
      rate,
      baseRate,
      rateTitle,
      status: v1Status(item.primary_status),
      metrics: [
        { label: t('channelStatusBoard.metrics.avgLatency'), ...splitLatency(latencyMs), hint: t('channelStatusBoard.metrics.probeAvgHint') },
        { label: t('channelStatusBoard.metrics.availability'), ...splitPercent(availability), hint: ctx.windowLabel },
        { label: t('channelStatusBoard.metrics.cacheRate'), value: BOARD_EMPTY, hint: t('channelStatusBoard.metrics.cacheUnavailable') },
      ],
      bars: v1Bars(item, t),
      barsLabel: t('channelStatusBoard.recentPoints', { n: V1_BAR_COUNT }),
      sort: { rate, success: availability, cache: null, latency: latencyMs },
      searchText: [item.name, item.group_name, platformLabel, ...models, ...groups.map((g) => g.name)]
        .filter(Boolean)
        .join(' ')
        .toLowerCase(),
    }
  })
}
