export default { qqbot: {
  profile: {
    title: 'QQ Bot', description: 'Once bound, you can check in and view channel status and public groups from QQ groups or private chat.',
    bound: 'QQ bound', boundAt: 'Bound at {time}', notBound: 'QQ is not bound yet',
    howTo: "Send this to the bot in a private chat (or {'@'} the bot in a group):", command: '/绑定 {email}',
    unbind: 'Unbind', unbindConfirm: 'Unbind QQ? You can bind again from QQ later.',
    unbound: 'QQ unbound', loadFailed: 'Failed to load QQ binding', unbindFailed: 'Failed to unbind',
  },
  admin: {
    title: 'QQ Bot', description: 'Connect the official QQ bot (webhook) to offer /签到 /绑定 /状态 /分组 /帮助 commands.',
    enabled: 'Enable QQ bot', enabledHint: 'When off, the callback URL returns 404 and the bot stops responding.',
    appId: 'AppID', appSecret: 'AppSecret', secretConfigured: 'Configured; leave empty to keep', secretPlaceholder: 'Enter AppSecret',
    callbackUrl: 'Callback URL', callbackHint: "Set this URL in the QQ open platform callback settings and subscribe to group {'@'} messages and C2C messages.",
    copy: 'Copy', copied: 'Copied', save: 'Save', saved: 'QQ bot settings saved', loadFailed: 'Failed to load QQ bot settings', saveFailed: 'Failed to save',
    note: 'Check-in also requires Daily Check-in to be enabled; /状态 requires channel monitoring. Links in group messages need domain registration on QQ, so replies contain no links.',
  },
} }
