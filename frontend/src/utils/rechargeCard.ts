import type { PublicSettings } from '@/types'

export function resolveRechargeCard(settings?: Pick<PublicSettings,
  'recharge_card_enabled' | 'recharge_card_url' | 'recharge_card_open_mode'> | null) {
  let url = ''
  try {
    const parsed = new URL(settings?.recharge_card_url?.trim() || '')
    if (['http:', 'https:'].includes(parsed.protocol) && !parsed.username && !parsed.password) {
      url = parsed.href
    }
  } catch { /* Unconfigured or invalid URLs never become links or iframe sources. */ }
  return {
    enabled: settings?.recharge_card_enabled === true && !!url,
    url,
    mode: settings?.recharge_card_open_mode === 'new_tab' ? 'new_tab' : 'iframe'
  }
}
