/**
 * 提示词灵感与风格预设。文案在 i18n 的 imageWorkbench.presets / imageWorkbench.styles 下，
 * 这里只维护 ID 与推荐尺寸。
 */
export interface PromptPreset {
  id: string
  size?: string
  aspectRatio?: string
}

export const PROMPT_PRESETS: PromptPreset[] = [
  { id: 'product', size: '1024x1024', aspectRatio: '1:1' },
  { id: 'poster', size: '1024x1536', aspectRatio: '2:3' },
  { id: 'avatar', size: '1024x1024', aspectRatio: '1:1' },
  { id: 'landscape', size: '1536x1024', aspectRatio: '3:2' },
  { id: 'icon', size: '1024x1024', aspectRatio: '1:1' },
  { id: 'interior', size: '1536x1024', aspectRatio: '3:2' },
  { id: 'food', size: '1024x1024', aspectRatio: '1:1' },
  { id: 'wallpaper', size: '1024x1536', aspectRatio: '9:16' },
]

export const STYLE_PRESETS = ['photo', 'cinematic', 'anime', 'watercolor', 'flat', 'threeD', 'pixel', 'ink'] as const
export type StylePreset = (typeof STYLE_PRESETS)[number]
