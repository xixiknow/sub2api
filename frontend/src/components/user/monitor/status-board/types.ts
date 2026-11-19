/**
 * Shared, data-source-agnostic models for the channel status card board.
 * Both the V1 (active probe) and V2 (real traffic) channel status pages map
 * their payloads into these shapes and reuse the same toolbar/card components.
 */

/** Channel status page layout: card board (default) or the V2 detail view. */
export type BoardViewMode = 'cards' | 'detail'

export type BoardStatus = 'operational' | 'degraded' | 'failed' | 'unknown'

/** Platform tabs shown in the toolbar; everything else falls into `other`. */
export const BOARD_PLATFORM_TABS = ['openai', 'anthropic', 'gemini', 'deepseek', 'grok', 'other'] as const
export type BoardPlatformTab = (typeof BOARD_PLATFORM_TABS)[number]
export type BoardPlatformFilter = 'all' | BoardPlatformTab

export type BoardSortKey = 'default' | 'rate' | 'success' | 'cache' | 'latency' | 'status'
export type BoardSortDir = 'asc' | 'desc'
export interface BoardSort {
  key: BoardSortKey
  dir: BoardSortDir
}

/** Natural direction for each sort key (shown as the arrow when first picked). */
export const BOARD_SORT_DEFAULT_DIR: Record<BoardSortKey, BoardSortDir> = {
  default: 'asc',
  rate: 'asc',
  success: 'desc',
  cache: 'desc',
  latency: 'asc',
  status: 'asc',
}

export interface BoardMetric {
  label: string
  /** Numeric part, e.g. "6.79"; "—" when unavailable. */
  value: string
  /** Unit rendered smaller next to the value, e.g. "s" or "%". */
  unit?: string
  hint?: string
  title?: string
}

export interface BoardBar {
  status: BoardStatus
  /** Tooltip lines; the first one is rendered as the title. */
  lines: string[]
}

export interface BoardSortValues {
  rate: number | null
  success: number | null
  cache: number | null
  latency: number | null
}

export interface StatusCardModel {
  key: string
  title: string
  platform: string
  platformLabel: string
  /** Effective multiplier shown on the badge (user-specific when present). */
  rate: number | null
  /** Original group multiplier; rendered struck-through when it differs from `rate`. */
  baseRate: number | null
  rateTitle?: string
  status: BoardStatus
  metrics: BoardMetric[]
  bars: BoardBar[]
  barsLabel: string
  sort: BoardSortValues
  /** Lower-cased text matched by the search box. */
  searchText: string
}
