import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ContactView from '../ContactView.vue'

const { getContactPage, copyToClipboard } = vi.hoisted(() => ({ getContactPage: vi.fn(), copyToClipboard: vi.fn() }))
vi.mock('@/api/contact', () => ({ getContactPage }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))

const full = {
  image: 'data:image/png;base64,AAAA',
  link: 'https://qm.qq.com/q/YqYR2U0gMw',
  text: 'Codex-Claude交流\n群号：1102643327',
  contact_info: '1807730267@qq.com'
}

describe('ContactView', () => {
  beforeEach(() => { vi.clearAllMocks() })

  it('renders community image, notes, join link and support email', async () => {
    getContactPage.mockResolvedValue(full)
    const wrapper = mount(ContactView); await flushPromises()
    expect(wrapper.get('img').attributes('src')).toBe(full.image)
    expect(wrapper.get('[data-testid="contact-community"]').text()).toContain('群号：1102643327')
    const join = wrapper.get('a[target="_blank"]')
    expect(join.attributes('href')).toBe(full.link)
    expect(join.attributes('rel')).toBe('noopener noreferrer')
    const support = wrapper.get('[data-testid="contact-support"]')
    expect(support.get('a').attributes('href')).toBe('mailto:1807730267@qq.com')
    await support.get('button').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('1807730267@qq.com', 'contact.copied')
  })

  it('hides unsafe links and shows only configured blocks', async () => {
    getContactPage.mockResolvedValue({ image: 'javascript:alert(1)', link: 'javascript:alert(1)', text: '', contact_info: 'QQ: 123456' })
    const wrapper = mount(ContactView); await flushPromises()
    expect(wrapper.find('[data-testid="contact-community"]').exists()).toBe(false)
    const support = wrapper.get('[data-testid="contact-support"]')
    expect(support.text()).toContain('QQ: 123456')
    expect(support.find('a').exists()).toBe(false)
  })

  it('shows empty state and retryable error', async () => {
    getContactPage.mockResolvedValueOnce({ image: '', link: '', text: '', contact_info: '' })
    const wrapper = mount(ContactView); await flushPromises()
    expect(wrapper.text()).toContain('contact.empty')

    getContactPage.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(full)
    const failing = mount(ContactView); await flushPromises()
    expect(failing.get('[role="alert"]').text()).toContain('contact.loadError')
    await failing.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(failing.find('[data-testid="contact-support"]').exists()).toBe(true)
  })
})
