export default {
  accountTree: {
    toggle: '分组视图',
    all: '全部账号',
    title: '分组',
    activeCount: '启用 {active} / 共 {total}',
    errorCount: '{count} 个异常'
  },
  upstreamBalance: {
    label: '上游余额',
    never: '未查询',
    unlimited: '不限额',
    used: '已用',
    unsupported: '不支持',
    failed: '查询失败',
    refresh: '查询上游余额',
    refreshFailed: '查询上游余额失败',
    bulkQuery: '查询上游余额',
    bulkLimit: '一次最多查询 50 个账号',
    bulkDone: '已查询 {count} 个账号的上游余额',
    bulkPartial: '查询完成：成功 {success}，失败 {failed}'
  },
  profit: {
    title: '盈利运营',
    description: '按分组、账号、平台、模型统计收入、上游成本与利润。',
    formulaHint: '收入 = 用户实扣（已含分组/用户专属/高峰倍率）；成本 = 标准价 × 账号上游倍率；订阅套餐消耗默认不计入现金收入。',
    includeSubscription: '计入订阅消耗',
    allGroups: '全部分组',
    allPlatforms: '全部平台',
    noGroup: '未分组',
    trend: '利润趋势',
    export: '导出 CSV',
    noData: '暂无数据',
    loadFailed: '加载盈利数据失败',
    dimensions: {
      group: '分组',
      account: '账号',
      platform: '平台',
      model: '模型'
    },
    columns: {
      groupRate: '分组倍率',
      accountRate: '上游倍率',
      requests: '请求数',
      tokens: 'Tokens',
      revenue: '收入',
      cost: '成本',
      profit: '利润',
      margin: '毛利率'
    },
    kpi: {
      revenue: '收入',
      cost: '上游成本',
      profit: '利润',
      margin: '毛利率',
      subscription: '订阅消耗',
      standardCost: '标准价 {value}',
      requests: '{value} 次请求',
      subscriptionCost: '对应成本 {value}'
    }
  }
}
