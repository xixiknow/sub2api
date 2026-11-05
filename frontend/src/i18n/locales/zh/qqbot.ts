export default { qqbot: {
  profile: {
    title: 'QQ 机器人', description: '绑定后可在 QQ 群或私聊中通过机器人签到、查看渠道状态与公开分组。',
    bound: '已绑定 QQ', boundAt: '绑定于 {time}', notBound: '尚未绑定 QQ',
    howTo: "在 QQ 中私聊机器人（或在群里 {'@'}机器人）发送：", command: '/绑定 {email}',
    unbind: '解除绑定', unbindConfirm: '确定解除 QQ 绑定吗？解除后可在 QQ 中重新绑定。',
    unbound: '已解除 QQ 绑定', loadFailed: '加载 QQ 绑定状态失败', unbindFailed: '解除绑定失败',
  },
  admin: {
    title: 'QQ 机器人', description: '接入 QQ 官方机器人（Webhook 回调），为用户提供 /签到 /绑定 /状态 /分组 /帮助 等指令。',
    enabled: '启用 QQ 机器人', enabledHint: '关闭后回调地址返回 404，机器人不再响应指令。',
    appId: 'AppID', appSecret: 'AppSecret', secretConfigured: '已配置，留空则不修改', secretPlaceholder: '输入 AppSecret',
    callbackUrl: '回调地址', callbackHint: "在 QQ 开放平台「开发 → 回调配置」中填写此地址，并订阅群聊 {'@'} 消息与单聊消息事件。",
    copy: '复制', copied: '已复制', save: '保存', saved: 'QQ 机器人配置已保存', loadFailed: '加载 QQ 机器人配置失败', saveFailed: '保存失败',
    note: '签到需要同时开启「每日签到」；/状态 需要开启渠道监控。群消息中的链接需在 QQ 后台报备域名，机器人回复默认不带链接。',
  },
} }
