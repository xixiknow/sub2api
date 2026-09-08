export const grokVideoPriceResolutions = [
  { key: '480p', label: '480p' },
  { key: '720p', label: '720p' },
  { key: '1080p', label: '1080p' }
] as const

export const dramaVideoPriceResolutions = [
  { key: '480p', label: '480p' },
  { key: '720p', label: '720p' },
  { key: '1080p', label: '1080p' },
  { key: '4k', label: '4K' }
] as const

export const grokVideoPriceFamilies = [
  { key: 'grok-imagine-video', label: 'grok-imagine-video' },
  { key: 'grok-imagine-video-1.5', label: 'grok-imagine-video-1.5' }
] as const

export const dramaVideoPriceFamilies = [
  { key: 'minimax-h3', label: 'minimax-h3' },
  { key: 'seedance2.0-A', label: 'seedance2.0-A' },
  { key: 'seedance2.0-B', label: 'seedance2.0-B' },
  { key: 'seedance2.0-fast-B', label: 'seedance2.0-fast-B' },
  { key: 'seedance-2.0-C', label: 'seedance-2.0-C' },
  { key: 'seedance2.0-E', label: 'seedance2.0-E' },
  { key: 'seedance2.0-F', label: 'seedance2.0-F' },
  { key: 'seedance2.0-fast-F', label: 'seedance2.0-fast-F' },
  { key: 'seedance2.5-A', label: 'seedance2.5-A' },
  { key: 'seedance-2.5-B', label: 'seedance-2.5-B' }
] as const

export type VideoModelPrices = Record<string, Record<string, number>>
export type VideoModelPricesForm = Record<string, Record<string, number | string | null>>

function normalizeFamily(value: string): string {
  return value.trim()
}

function normalizeFamilyKey(value: string): string {
  return value.trim().toLowerCase()
}

function normalizePrice(value: unknown): number | null {
  if (value === null || value === undefined || value === '') return null
  const price = Number(value)
  return Number.isFinite(price) && price >= 0 ? price : null
}

export function videoPriceResolutionsFor(platform?: string) {
  return platform === 'drama' ? dramaVideoPriceResolutions : grokVideoPriceResolutions
}

export function videoPriceFamiliesFor(platform?: string) {
  return platform === 'drama' ? dramaVideoPriceFamilies : grokVideoPriceFamilies
}

const dramaFamilyPricedResolutions: Record<string, readonly string[]> = {
  'minimax-h3': ['480p', '720p', '1080p'],
  'seedance2.0-A': ['480p', '720p', '1080p'],
  'seedance2.0-B': ['480p', '720p', '1080p', '4k'],
  'seedance2.0-fast-B': ['480p', '720p'],
  'seedance-2.0-C': ['720p'],
  'seedance2.0-E': ['720p'],
  'seedance2.0-F': ['720p', '1080p'],
  'seedance2.0-fast-F': ['720p'],
  'seedance2.5-A': ['480p', '720p', '1080p'],
  'seedance-2.5-B': ['720p']
}

const dramaPerClipFamilies = new Set([
  'seedance2.0-B',
  'seedance2.0-fast-B',
  'seedance-2.0-C',
  'seedance2.0-E',
  'seedance2.0-F',
  'seedance2.0-fast-F',
  'seedance-2.5-B'
])

export function videoResolutionEnabledForFamily(family: string, resolution: string, platform?: string): boolean {
  if (platform !== 'drama') {
    return true
  }
  const priced = dramaFamilyPricedResolutions[family]
  if (!priced) {
    return resolution !== '4k' || family === 'seedance2.0-B'
  }
  return priced.includes(resolution)
}

export function videoPriceUnitLabel(family: string, platform?: string): string {
  if (platform === 'drama' && dramaPerClipFamilies.has(family)) {
    return '$/条'
  }
  return '$/s'
}

function emptyTiers(platform?: string): Record<string, number | string | null> {
  return Object.fromEntries(videoPriceResolutionsFor(platform).map(({ key }) => [key, null]))
}

// Keep unknown families from an existing group so a future backend catalog is
// not silently discarded when an operator edits another group setting.
export function createVideoModelPricesForm(
  prices?: VideoModelPrices | null,
  platform?: string
): VideoModelPricesForm {
  const form: VideoModelPricesForm = {}

  for (const [rawFamily, rawTiers] of Object.entries(prices ?? {})) {
    const family = normalizeFamily(rawFamily)
    if (!family || !rawTiers || typeof rawTiers !== 'object') continue
    form[family] = emptyTiers(platform)
    for (const [rawResolution, rawPrice] of Object.entries(rawTiers)) {
      const price = normalizePrice(rawPrice)
      if (price !== null) form[family][rawResolution.trim().toLowerCase()] = price
    }
  }

  for (const { key } of videoPriceFamiliesFor(platform)) {
    form[key] ??= emptyTiers(platform)
  }
  return form
}

export function serializeVideoModelPrices(form: VideoModelPricesForm): VideoModelPrices {
  const result: VideoModelPrices = {}
  for (const [rawFamily, tiers] of Object.entries(form)) {
    const family = normalizeFamily(rawFamily)
    if (!family || !tiers || typeof tiers !== 'object') continue

    const normalizedTiers: Record<string, number> = {}
    for (const [rawResolution, rawPrice] of Object.entries(tiers)) {
      const resolution = rawResolution.trim().toLowerCase()
      const price = normalizePrice(rawPrice)
      if (resolution && price !== null) normalizedTiers[resolution] = price
    }
    if (Object.keys(normalizedTiers).length > 0) result[family] = normalizedTiers
  }
  return result
}

export function videoModelPriceFamilyRows(form: VideoModelPricesForm, platform?: string) {
  const catalog = videoPriceFamiliesFor(platform)
  const known = new Set<string>(catalog.map(({ key }) => normalizeFamilyKey(key)))
  const extra = Object.keys(form)
    .map(normalizeFamily)
    .filter((family) => family && !known.has(normalizeFamilyKey(family)))
    .sort()
    .map((key) => ({ key, label: key }))
  return [...catalog, ...extra]
}
