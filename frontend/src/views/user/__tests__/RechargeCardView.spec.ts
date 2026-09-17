import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RechargeCardView from '../RechargeCardView.vue'

const { store } = vi.hoisted(() => ({ store: {
  publicSettingsLoaded: true,
  cachedPublicSettings: { recharge_card_enabled: true, recharge_card_url: 'https://shop.example.com/cards', recharge_card_open_mode: 'iframe' },
  fetchPublicSettings: vi.fn().mockResolvedValue(null)
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => store }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const render = () => mount(RechargeCardView, { global: { stubs: {
  AppLayout: { template: '<main><slot /></main>' }, RouterLink: { template: '<a><slot /></a>' }
} } })

describe('RechargeCardView', () => {
  beforeEach(() => {
    store.cachedPublicSettings = { recharge_card_enabled: true, recharge_card_url: 'https://shop.example.com/cards', recharge_card_open_mode: 'iframe' }
  })
  it('embeds the configured URL and offers a new window and redemption link', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('iframe').attributes('src')).toBe('https://shop.example.com/cards')
    const link = wrapper.get('a[target="_blank"]')
    expect(link.attributes('href')).toBe('https://shop.example.com/cards')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
    expect(wrapper.text()).toContain('rechargeCard.redeem')
    await wrapper.get('iframe').trigger('load')
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
  })
  it('renders only an external link in new-window mode without an automatic popup', () => {
    store.cachedPublicSettings.recharge_card_open_mode = 'new_tab'
    const wrapper = render()
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.get('a[target="_blank"]').attributes('href')).toBe('https://shop.example.com/cards')
  })
  it.each([false, true])('hides the shop when disabled or the URL is invalid (%s)', (enabled) => {
    store.cachedPublicSettings.recharge_card_enabled = enabled
    if (enabled) store.cachedPublicSettings.recharge_card_url = 'javascript:alert(1)'
    const wrapper = render()
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('a[target="_blank"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('rechargeCard.unavailable')
  })
})
