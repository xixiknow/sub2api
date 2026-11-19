import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'

const { copyToClipboard, getAffiliateDetail, listAffiliateWithdraws } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getAffiliateDetail: vi.fn(),
  listAffiliateWithdraws: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    listAffiliateWithdraws,
    transferAffiliateQuota: vi.fn(),
    submitAffiliateWithdraw: vi.fn(),
    cancelAffiliateWithdraw: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('AffiliateView', () => {
  const affiliateCode = 'affiliate-code-that-is-long-enough-to-overflow-a-mobile-viewport'

  beforeEach(() => {
    vi.clearAllMocks()
    copyToClipboard.mockResolvedValue(true)
    listAffiliateWithdraws.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 0 })
    getAffiliateDetail.mockResolvedValue({
      user_id: 1,
      aff_code: affiliateCode,
      inviter_id: null,
      aff_count: 0,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 10,
      invitees: [],
    })
  })

  it('stacks long values and copy controls on mobile while retaining desktop rows', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const values = wrapper.findAll('code')
    expect(values).toHaveLength(2)
    for (const value of values) {
      expect(value.classes()).toEqual(expect.arrayContaining([
        'min-w-0',
        'break-all',
        'sm:flex-1',
        'sm:truncate',
      ]))
      expect(Array.from(value.element.parentElement?.classList ?? [])).toEqual(expect.arrayContaining([
        'flex-col',
        'items-stretch',
        'sm:flex-row',
        'sm:items-center',
      ]))
    }

    const copyButtons = wrapper.findAll('button').filter((button) =>
      ['affiliate.copyCode', 'affiliate.copyLink'].includes(button.text()),
    )
    expect(copyButtons).toHaveLength(2)
    for (const button of copyButtons) {
      expect(button.classes()).toEqual(expect.arrayContaining([
        'w-full',
        'sm:w-auto',
        'sm:shrink-0',
      ]))
    }

    await copyButtons[0].trigger('click')
    await copyButtons[1].trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenNthCalledWith(1, affiliateCode, 'affiliate.codeCopied')
    expect(copyToClipboard).toHaveBeenNthCalledWith(
      2,
      `${window.location.origin}/register?aff=${encodeURIComponent(affiliateCode)}`,
      'affiliate.linkCopied',
    )
  })

  const mountView = () => mount(AffiliateView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true,
        Teleport: true,
      },
    },
  })

  const baseDetail = {
    user_id: 1,
    aff_code: 'CODE1234',
    inviter_id: null,
    aff_count: 3,
    aff_quota: 80,
    aff_frozen_quota: 5,
    aff_history_quota: 120,
    effective_rebate_rate_percent: 8,
    invitees: [],
  }

  it('enables Alipay withdraw when allowed and shows history', async () => {
    getAffiliateDetail.mockResolvedValue({ ...baseDetail, withdraw_enabled: true, withdraw_min_amount: 50 })
    const wrapper = mountView()
    await flushPromises()

    const withdraw = wrapper.get('[data-test="assets-withdraw"]')
    expect(withdraw.attributes('disabled')).toBeUndefined()
    expect(wrapper.find('[data-test="assets-withdraw-hint"]').exists()).toBe(false)
    expect(listAffiliateWithdraws).toHaveBeenCalledWith(1, 10)
  })

  it('disables withdraw below the minimum amount', async () => {
    getAffiliateDetail.mockResolvedValue({ ...baseDetail, aff_quota: 20, withdraw_enabled: true, withdraw_min_amount: 50 })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="assets-withdraw"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="assets-withdraw-hint"]').text()).toBe('affiliate.assets.withdrawMinHint')
  })

  it('disables withdraw while a request is pending or the feature is off', async () => {
    getAffiliateDetail.mockResolvedValue({
      ...baseDetail,
      withdraw_enabled: true,
      withdraw_min_amount: 0,
      pending_withdraw: { id: 7, amount: 60, created_at: '2026-11-19T10:00:00Z' },
    })
    const pending = mountView()
    await flushPromises()
    expect(pending.get('[data-test="assets-withdraw"]').attributes('disabled')).toBeDefined()
    expect(pending.find('[data-test="assets-pending"]').exists()).toBe(true)

    getAffiliateDetail.mockResolvedValue({ ...baseDetail, withdraw_enabled: false })
    listAffiliateWithdraws.mockClear()
    const off = mountView()
    await flushPromises()
    expect(off.get('[data-test="assets-withdraw"]').attributes('disabled')).toBeDefined()
    expect(off.get('[data-test="assets-withdraw-hint"]').text()).toBe('affiliate.assets.withdrawDisabled')
    expect(listAffiliateWithdraws).not.toHaveBeenCalled()
  })
})
