export default { qqbot: {
  profile: {
    title: 'QQ 机器人', description: '绑定后可在 QQ 群或私聊中通过机器人签到、查看渠道状态与公开分组。',
    bound: '已绑定 QQ', boundAt: '绑定于 {time}', notBound: '尚未绑定 QQ',
    howTo: '在 QQ 中私聊机器人发送以下内容（绑定码仅可使用一次）：',
    generate: '生成绑定码', regenerate: '重新生成绑定码', generateHint: '绑定码 10 分钟内有效，请勿发给他人。',
    generateFailed: '生成绑定码失败', codeExpires: '{seconds} 秒后过期，绑定成功后本页会自动刷新', copy: '复制', copied: '已复制',
    unbind: '解除绑定', unbindConfirm: '确定解除 QQ 绑定吗？解除后需重新生成绑定码才能再次绑定。',
    unbound: '已解除 QQ 绑定', loadFailed: '加载 QQ 绑定状态失败', unbindFailed: '解除绑定失败',
  },
  admin: {
    title: 'QQ 机器人', description: '接入 QQ 官方机器人（Webhook 回调），为用户提供 /签到 /绑定 /状态 /分组 /帮助 等指令。',
    enabled: '启用 QQ 机器人', enabledHint: '关闭后回调地址返回 404，机器人不再响应指令。',
    appId: 'AppID', appSecret: 'AppSecret', secretConfigured: '已配置，留空则不修改', secretPlaceholder: '输入 AppSecret',
    callbackUrl: '回调地址', callbackHint: "在 QQ 开放平台「开发 → 回调配置」中填写此地址，并订阅群聊 {'@'} 消息与单聊消息事件。如需群内免 {'@'}：再订阅「群全量消息」事件，并由群主在群设置中把机器人可获取的消息范围设为「获取群内全部消息」，之后群成员可直接发送 /指令（只响应以 / 开头的消息）。",
    copy: '复制', copied: '已复制', save: '保存', saved: 'QQ 机器人配置已保存', loadFailed: '加载 QQ 机器人配置失败', saveFailed: '保存失败',
    poster: {
      title: '邀请海报（/邀请）', description: '用本站图片模型为每张海报生成独特插画，站点名、Logo、二维码由程序叠加。',
      apiKey: '本站 API Key', apiKeyPlaceholder: '填写一个本站 API Key（sk-...）', apiKeyHint: '生图通过本站网关调用，费用计入该 Key 所属账号；留空则只用插画池或程序背景。',
      model: '生图模型', dailyLimit: '每人每日生成上限',
      hint: '超出上限或排队较多时复用最近生成的插画；生成失败时使用程序绘制的背景。建议在网站设置中上传 PNG 格式的站点 Logo。',
    },
    note: '签到需要同时开启「每日签到」；/状态 需要开启渠道监控。群消息中的链接需在 QQ 后台报备域名，机器人回复默认不带链接。',
  },
} }
