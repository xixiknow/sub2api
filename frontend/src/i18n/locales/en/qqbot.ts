export default { qqbot: {
  profile: {
    title: 'QQ Bot', description: 'Once bound, you can check in and view channel status and public groups from QQ groups or private chat.',
    bound: 'QQ bound', boundAt: 'Bound at {time}', notBound: 'QQ is not bound yet',
    howTo: 'Send this to the bot in a private chat (the code works only once):',
    generate: 'Generate binding code', regenerate: 'Generate a new code', generateHint: 'The code is valid for 10 minutes. Do not share it.',
    generateFailed: 'Failed to generate binding code', codeExpires: 'Expires in {seconds}s; this page refreshes once binding succeeds', copy: 'Copy', copied: 'Copied',
    unbind: 'Unbind', unbindConfirm: 'Unbind QQ? You will need a new binding code to bind again.',
    unbound: 'QQ unbound', loadFailed: 'Failed to load QQ binding', unbindFailed: 'Failed to unbind',
  },
  admin: {
    title: 'QQ Bot', description: 'Connect the official QQ bot (webhook) to offer /签到 /绑定 /状态 /分组 /帮助 commands.',
    enabled: 'Enable QQ bot', enabledHint: 'When off, the callback URL returns 404 and the bot stops responding.',
    appId: 'AppID', appSecret: 'AppSecret', secretConfigured: 'Configured; leave empty to keep', secretPlaceholder: 'Enter AppSecret',
    callbackUrl: 'Callback URL', callbackHint: "Set this URL in the QQ open platform callback settings and subscribe to group {'@'} messages and C2C messages. To allow commands without {'@'}, also subscribe to the group full-message event and have the group owner set the bot's message scope to \"all group messages\"; members can then send /commands directly (only messages starting with / are handled).",
    copy: 'Copy', copied: 'Copied', save: 'Save', saved: 'QQ bot settings saved', loadFailed: 'Failed to load QQ bot settings', saveFailed: 'Failed to save',
    poster: {
      title: 'Invite poster (/邀请)', description: 'Each poster gets a unique illustration from the image model of this site; site name, logo and QR code are drawn by the server.',
      apiKey: 'Site API key', apiKeyPlaceholder: 'An API key of this site (sk-...)', apiKeyHint: 'Images are generated through this gateway and billed to the key owner; leave empty to use only the reuse pool or generated backgrounds.',
      model: 'Image model', dailyLimit: 'Daily generations per user',
      hint: 'Over the limit or under heavy load, recent illustrations are reused; on failure a generated background is used. Upload a PNG site logo for best results.',
    },
    note: 'Check-in also requires Daily Check-in to be enabled; /状态 requires channel monitoring. Links in group messages need domain registration on QQ, so replies contain no links.',
  },
} }
