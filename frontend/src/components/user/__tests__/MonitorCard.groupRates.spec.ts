import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import type { UserMonitorView } from '@/api/channelMonitor'
import type { Group } from '@/types'
import MonitorCard from '../monitor/MonitorCard.vue'

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorQuotaVisible: () => false,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, te: () => true }),
  }
})

const item: UserMonitorView = {
  id: 1,
  name: 'claude-main',
  provider: 'anthropic',
  group_name: '',
  group_ids: [7],
  primary_model: 'claude-sonnet',
  primary_status: 'operational',
  primary_latency_ms: null,
  primary_ping_latency_ms: null,
  availability_7d: 100,
  extra_models: [],
  timeline: [],
}

const group = { id: 7, name: 'Claude 优质', platform: 'anthropic', subscription_type: 'standard', rate_multiplier: 1.5 } as Group

function mountCard(props: Record<string, unknown>) {
  return mount(MonitorCard, {
    props: { item, window: '7d', availabilityValue: 100, countdownSeconds: 0, ...props },
    global: {
      stubs: { MonitorMetricPair: true, MonitorAvailabilityRow: true, MonitorTimeline: true },
    },
  })
}

describe('MonitorCard group rates', () => {
  beforeEach(() => { setActivePinia(createPinia()) })

  it('shows linked group badges with the user-specific rate', () => {
    const wrapper = mountCard({ groups: [group], userGroupRates: { 7: 0.8 } })
    const badges = wrapper.get('[data-testid="monitor-group-rates"]')
    expect(badges.text()).toContain('Claude 优质')
    expect(badges.text()).toContain('0.8')
  })

  it('renders nothing when no usable groups are linked', () => {
    const wrapper = mountCard({ groups: [] })
    expect(wrapper.find('[data-testid="monitor-group-rates"]').exists()).toBe(false)
  })
})
