import type { ApiKey, Group } from '@/types'

/**
 * 生图工作台的模型目录、尺寸档位与费用预估。
 * 模型 ID 与后端 backend/internal/pkg/openai/constants.go、pkg/xai/models.go 保持一致；
 * 尺寸档位口径与后端 ClassifyImageBillingTier 一致（最长边 ≤1024 为 1K，≤2048 为 2K，其余 4K）。
 */

export type WorkbenchPlatform = 'openai' | 'grok'
export type WorkbenchMode = 'generate' | 'edit'
export type ImageBillingTier = '1K' | '2K' | '4K'

export interface WorkbenchModel {
  id: string
  label: string
  platform: WorkbenchPlatform
  sizes: string[]
  supportsQuality: boolean
  supportsBackground: boolean
  supportsEdit: boolean
  maxCount: number
}

const GPT_IMAGE_BASE_SIZES = ['auto', '1024x1024', '1536x1024', '1024x1536']
const GPT_IMAGE_HD_SIZES = [...GPT_IMAGE_BASE_SIZES, '2048x2048', '2048x1152', '1152x2048', '3840x2160', '2160x3840']

function gptImage(id: string, label: string, hd: boolean): WorkbenchModel {
  return {
    id,
    label,
    platform: 'openai',
    sizes: hd ? GPT_IMAGE_HD_SIZES : GPT_IMAGE_BASE_SIZES,
    supportsQuality: true,
    supportsBackground: true,
    supportsEdit: true,
    maxCount: 4,
  }
}

export const GROK_ASPECT_RATIOS = ['1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3']

function grokImage(id: string, label: string, supportsEdit: boolean): WorkbenchModel {
  return {
    id,
    label,
    platform: 'grok',
    sizes: [],
    supportsQuality: false,
    supportsBackground: false,
    supportsEdit,
    maxCount: 4,
  }
}

export const WORKBENCH_MODELS: WorkbenchModel[] = [
  gptImage('gpt-image-2', 'GPT Image 2', true),
  gptImage('gpt-image-2.6-sunburst', 'GPT Image 2.6 Sunburst', true),
  gptImage('gpt-image-2.6-flare', 'GPT Image 2.6 Flare', true),
  gptImage('gpt-image-2.5-sunburst', 'GPT Image 2.5 Sunburst', true),
  gptImage('gpt-image-2.5-flare', 'GPT Image 2.5 Flare', true),
  gptImage('gpt-image-1.5', 'GPT Image 1.5', false),
  gptImage('gpt-image-1', 'GPT Image 1', false),
  grokImage('grok-imagine-image-quality', 'Grok Imagine Quality', true),
  grokImage('grok-imagine-image', 'Grok Imagine', false),
  grokImage('grok-imagine-image-2.0', 'Grok Imagine 2.0', false),
]

/** composite 分组按生图路由转发，这里按 OpenAI 模型目录处理。 */
export function resolveWorkbenchPlatform(group: Group | undefined | null): WorkbenchPlatform | null {
  if (!group) return null
  if (group.platform === 'openai' || group.platform === 'composite') return 'openai'
  if (group.platform === 'grok') return 'grok'
  return null
}

export function keySupportsWorkbench(key: ApiKey): boolean {
  return key.status === 'active' && key.group?.allow_image_generation === true && resolveWorkbenchPlatform(key.group) !== null
}

export function modelsForPlatform(platform: WorkbenchPlatform | null, mode: WorkbenchMode): WorkbenchModel[] {
  if (!platform) return []
  return WORKBENCH_MODELS.filter((model) => model.platform === platform && (mode === 'generate' || model.supportsEdit))
}

export function findWorkbenchModel(id: string): WorkbenchModel | undefined {
  return WORKBENCH_MODELS.find((model) => model.id === id)
}

export function classifyImageTier(size: string | undefined): ImageBillingTier {
  const normalized = String(size || '').trim().toLowerCase()
  const match = /^(\d+)x(\d+)$/.exec(normalized)
  if (!match) return '1K'
  const maxEdge = Math.max(Number(match[1]), Number(match[2]))
  if (maxEdge <= 1024) return '1K'
  if (maxEdge <= 2048) return '2K'
  return '4K'
}

export interface CostEstimate {
  /** 预估总价（USD）；分组未配置单价时为 null，表示按站点默认价计费 */
  total: number | null
  unitPrice: number | null
  multiplier: number
  tier: ImageBillingTier
}

export function estimateImageCost(
  group: Group | undefined | null,
  size: string | undefined,
  count: number,
  userRateOverride?: number,
): CostEstimate {
  const tier = classifyImageTier(size)
  const effectiveRate = typeof userRateOverride === 'number' ? userRateOverride : group?.rate_multiplier ?? 1
  const multiplier = group?.image_rate_independent ? Math.max(0, group.image_rate_multiplier ?? 1) : effectiveRate
  const priceByTier: Record<ImageBillingTier, number | null | undefined> = {
    '1K': group?.image_price_1k,
    '2K': group?.image_price_2k,
    '4K': group?.image_price_4k,
  }
  const unit = priceByTier[tier]
  const unitPrice = typeof unit === 'number' && unit >= 0 ? unit : null
  const safeCount = Math.max(1, Math.floor(count || 1))
  return {
    total: unitPrice === null ? null : unitPrice * safeCount * multiplier,
    unitPrice,
    multiplier,
    tier,
  }
}

export function formatUsd(value: number): string {
  if (value === 0) return '$0'
  if (value < 0.01) return `$${value.toFixed(4)}`
  return `$${value.toFixed(value < 1 ? 3 : 2)}`
}
