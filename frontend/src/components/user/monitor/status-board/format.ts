/** Value/unit splitters so cards can render "6.79" large and "s" small. */

export const BOARD_EMPTY = '—'

export function splitLatency(ms: number | null | undefined): { value: string; unit?: string } {
  if (ms == null || !Number.isFinite(ms)) return { value: BOARD_EMPTY }
  if (ms >= 1000) return { value: (ms / 1000).toFixed(2), unit: 's' }
  return { value: String(Math.round(ms)), unit: 'ms' }
}

/** `ratio` is 0..1. */
export function splitRatioPercent(ratio: number | null | undefined): { value: string; unit?: string } {
  if (ratio == null || !Number.isFinite(ratio)) return { value: BOARD_EMPTY }
  return { value: (Math.max(0, Math.min(1, ratio)) * 100).toFixed(2), unit: '%' }
}

/** `percent` is 0..100. */
export function splitPercent(percent: number | null | undefined): { value: string; unit?: string } {
  if (percent == null || !Number.isFinite(percent)) return { value: BOARD_EMPTY }
  return { value: Math.max(0, Math.min(100, percent)).toFixed(2), unit: '%' }
}

/** Multiplier text without trailing zeros: 0.039 → "0.039", 1 → "1". */
export function formatBoardRate(rate: number | null | undefined): string {
  if (rate == null || !Number.isFinite(rate)) return BOARD_EMPTY
  return String(Number(rate.toFixed(4)))
}
