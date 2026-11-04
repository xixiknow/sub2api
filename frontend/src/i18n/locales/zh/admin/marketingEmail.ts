export default {
  marketingEmail: {
    title: '营销邮件',
    description: '按人群筛选用户并群发推广、挽留邮件，带限速、退订和发送明细',
    createCampaign: '新建邮件活动',
    editCampaign: '编辑邮件活动',
    search: '搜索名称或主题',
    allStatus: '全部状态',
    empty: '还没有邮件活动，先建一个试试，比如挽留 14 天未活跃的用户',
    failed: '操作失败',
    saved: '已保存',
    contentRequired: '请填写主题和正文',
    testSent: '测试邮件已发送到 {email}',
    sendQueued: '已加入发送队列，进度会自动刷新',
    noAudience: '没有符合条件的用户',
    finishedAt: '结束于',
    progressDetail: '失败 {failed} · 跳过 {skipped}',
    confirmCancel: '确定取消发送吗？已发出的邮件无法撤回，剩余未发送的用户将不再发送。',
    confirmDelete: '确定删除这个邮件活动及其发送记录吗？',
    confirmSend: {
      title: '确认发送',
      counting: '正在统计收件人数…',
      message: '将向约 {count} 位用户发送「{subject}」。限速每分钟 {rate} 封，预计约 {minutes} 分钟发完。发出后无法撤回，只能中途取消。'
    },
    columns: {
      name: '名称 / 主题',
      status: '状态',
      audience: '人群',
      progress: '进度',
      createdAt: '创建时间',
      actions: '操作'
    },
    status: {
      draft: '草稿',
      pending: '排队中',
      running: '发送中',
      completed: '已完成',
      canceled: '已取消',
      failed: '失败'
    },
    recipientStatus: {
      pending: '待发送',
      sent: '已发送',
      failed: '失败',
      skipped: '已跳过（退订）'
    },
    summary: {
      all: '全部活跃用户',
      inactive: '{days} 天未活跃',
      neverUsedApi: '从未调用 API',
      balance: '余额 {min} ~ {max}',
      registered: '按注册时间',
      explicit: '指定 {count} 人'
    },
    actions: {
      send: '发送',
      cancel: '取消发送',
      recipients: '收件人明细',
      duplicate: '复制为新活动',
      sendTest: '发测试邮件给我',
      saveDraft: '保存草稿',
      saveAndSend: '保存并发送…'
    },
    audience: {
      title: '收件人群',
      inactive: '最近 N 天没有任何活动',
      days: '天',
      inactiveHint: '同时满足：N 天内没登录、没打开控制台、没调用 API，且注册已超过 N 天',
      neverUsedApi: '从未调用过 API',
      balanceRange: '余额区间',
      min: '最小',
      max: '最大',
      registeredRange: '注册日期',
      explicit: '只发给指定用户（可选）',
      explicitPlaceholder: '每行一个用户 ID 或邮箱，也可以用逗号分隔',
      explicitHint: '填写后只在这些用户中按上面的条件筛选；只填这里、不勾其他条件就是定向发送',
      includeAdmins: '包含管理员账号',
      calculate: '计算人数',
      matched: '当前匹配 {count} 位用户',
      sample: '样本（最近注册的 10 位）'
    },
    content: {
      title: '邮件内容',
      starter: '起始模板：',
      starterRetention: '好久不见（挽留）',
      starterFeature: '新功能公告',
      name: '活动名称（仅后台可见）',
      namePlaceholder: '不填则使用邮件主题',
      locale: '页脚语言',
      subject: '邮件主题',
      subjectHint: '当前 {count} 字。建议不超过 30 个字，手机上超过约 40 个字会被截断',
      layout: '版式',
      layoutCard: '卡片版式（推荐）',
      layoutRaw: '完整 HTML',
      headerTitle: '顶部标题',
      accent: '主色',
      body: '正文 HTML',
      bodyHintCard: '只写正文部分即可，会自动套上卡片外壳和页脚。按钮可用 <a class="button" href="...">，颜色跟随主色。',
      bodyHintRaw: '粘贴完整的 HTML 邮件。系统会在 </body> 前自动插入退订页脚，请勿自行删除。',
      throttle: '发送速度（封/分钟）',
      throttleHint: '默认 30。速度太快容易被邮件服务商限流或判为垃圾邮件，大批量发送建议不超过 60。'
    },
    placeholders: {
      recipient_name: '用户名（为空时取邮箱前缀）',
      recipient_email: '收件人邮箱',
      site_name: '站点名称',
      site_url: '站点地址（系统设置中的前端地址）',
      days_inactive: '距离最后一次活动的天数',
      balance: '账户余额'
    },
    preview: {
      title: '实时预览（示例数据）'
    },
    tips: {
      title: '写作建议',
      subject: "主题直接点明对用户的好处，可以带上 {'{{'}recipient_name{'}}'} 增加亲切感。",
      spam: '少用“免费”“赚钱”“速领”、全大写和大量感叹号，容易被判为垃圾邮件。',
      cta: '一封邮件只放一个主要按钮，告诉用户下一步该做什么。',
      footer: '页脚的退订链接由系统自动添加，退订只影响营销邮件，不影响账单和安全通知。',
      test: '正式发送前先点“发测试邮件给我”，在手机和电脑上各看一遍。'
    },
    recipients: {
      title: '收件人明细 - {name}',
      all: '全部',
      email: '收件人',
      lastActivity: '最后活动',
      sentAt: '发送时间',
      error: '错误信息'
    }
  }
}
