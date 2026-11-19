import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminAffiliateWithdrawalsView from '../AdminAffiliateWithdrawalsView.vue'

const { listWithdrawRequests, getWithdrawRequest, markWithdrawPaid, rejectWithdrawRequest, showError, showSuccess } = vi.hoisted(() => ({
  listWithdrawRequests: vi.fn(),
  getWithdrawRequest: vi.fn(),
  markWithdrawPaid: vi.fn(),
  rejectWithdrawRequest: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin/affiliates', () => {
  const affiliatesAPI = { listWithdrawRequests, getWithdrawRequest, markWithdrawPaid, rejectWithdrawRequest }
  return { affiliatesAPI, default: affiliatesAPI }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() }),
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

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <table>
      <tr v-for="(row, index) in data" :key="index">
        <td v-for="column in columns" :key="column.key">
          <slot :name="'cell-' + column.key" :row="row" />
        </td>
      </tr>
    </table>
  `,
}

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show" data-test="dialog"><slot /><slot name="footer" /></div>',
}

const pendingRow = {
  id: 11,
  user_id: 7,
  user_email: 'u@example.com',
  username: 'u',
  amount: 60,
  alipay_account: 'alipay@example.com',
  status: 'pending',
  created_at: '2026-11-19T10:00:00Z',
  updated_at: '2026-11-19T10:00:00Z',
}

function mountView() {
  return mount(AdminAffiliateWithdrawalsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        DataTable: DataTableStub,
        Pagination: true,
        Icon: true,
        BaseDialog: BaseDialogStub,
      },
    },
  })
}

describe('AdminAffiliateWithdrawalsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listWithdrawRequests.mockResolvedValue({ items: [pendingRow], total: 1, page: 1, page_size: 20, pages: 1 })
    getWithdrawRequest.mockResolvedValue({ ...pendingRow, qr_image: 'data:image/png;base64,AAAA' })
    markWithdrawPaid.mockResolvedValue({ ...pendingRow, status: 'paid' })
    rejectWithdrawRequest.mockResolvedValue({ ...pendingRow, status: 'rejected' })
  })

  it('defaults to pending requests and shows the QR code in the detail dialog', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listWithdrawRequests).toHaveBeenCalledWith(expect.objectContaining({ status: 'pending', page: 1 }))

    await wrapper.get('[data-test="withdraw-view"]').trigger('click')
    await flushPromises()

    expect(getWithdrawRequest).toHaveBeenCalledWith(11)
    expect(wrapper.get('[data-test="withdraw-qr"]').attributes('src')).toBe('data:image/png;base64,AAAA')
    expect(wrapper.get('[data-test="withdraw-account"]').text()).toBe('alipay@example.com')
  })

  it('requires a reason before rejecting', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="withdraw-view"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="withdraw-reject"]').trigger('click')
    await wrapper.get('[data-test="withdraw-confirm"]').trigger('click')
    await flushPromises()
    expect(rejectWithdrawRequest).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.affiliates.withdrawals.rejectReasonRequired')

    await wrapper.get('[data-test="withdraw-note"]').setValue('  收款码无法识别 ')
    await wrapper.get('[data-test="withdraw-confirm"]').trigger('click')
    await flushPromises()
    expect(rejectWithdrawRequest).toHaveBeenCalledWith(11, '收款码无法识别')
    expect(listWithdrawRequests).toHaveBeenCalledTimes(2)
  })

  it('marks a request as paid with an optional note', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="withdraw-view"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="withdraw-paid"]').trigger('click')
    await wrapper.get('[data-test="withdraw-confirm"]').trigger('click')
    await flushPromises()

    expect(markWithdrawPaid).toHaveBeenCalledWith(11, '')
    expect(showSuccess).toHaveBeenCalledWith('admin.affiliates.withdrawals.paidSuccess')
  })
})
