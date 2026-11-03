export default {
  accountTree: {
    toggle: 'Group View',
    all: 'All Accounts',
    title: 'Groups',
    activeCount: '{active} active / {total} total',
    errorCount: '{count} with errors'
  },
  upstreamBalance: {
    label: 'Upstream',
    never: 'Not queried',
    unlimited: 'Unlimited',
    used: 'used',
    unsupported: 'Unsupported',
    failed: 'Query failed',
    refresh: 'Query upstream balance',
    refreshFailed: 'Failed to query upstream balance',
    bulkQuery: 'Query Upstream Balance',
    bulkLimit: 'At most 50 accounts per query',
    bulkDone: 'Queried upstream balance for {count} accounts',
    bulkPartial: 'Done: {success} succeeded, {failed} failed'
  },
  profit: {
    title: 'Profit',
    description: 'Revenue, upstream cost and profit by group, account, platform and model.',
    formulaHint: 'Revenue = amount charged to users (group / user-specific / peak multipliers applied); Cost = standard price × account upstream rate; subscription consumption is excluded from cash revenue by default.',
    includeSubscription: 'Include subscriptions',
    allGroups: 'All Groups',
    allPlatforms: 'All Platforms',
    noGroup: 'Ungrouped',
    trend: 'Profit Trend',
    export: 'Export CSV',
    noData: 'No data',
    loadFailed: 'Failed to load profit data',
    dimensions: {
      group: 'Group',
      account: 'Account',
      platform: 'Platform',
      model: 'Model'
    },
    columns: {
      groupRate: 'Group Rate',
      accountRate: 'Upstream Rate',
      requests: 'Requests',
      tokens: 'Tokens',
      revenue: 'Revenue',
      cost: 'Cost',
      profit: 'Profit',
      margin: 'Margin'
    },
    kpi: {
      revenue: 'Revenue',
      cost: 'Upstream Cost',
      profit: 'Profit',
      margin: 'Margin',
      subscription: 'Subscription Usage',
      standardCost: 'Standard {value}',
      requests: '{value} requests',
      subscriptionCost: 'Cost {value}'
    }
  }
}
