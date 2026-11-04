export default {
  marketingEmail: {
    title: 'Marketing Email',
    description: 'Send promotional and win-back emails to a filtered audience, with throttling, unsubscribe and delivery details',
    createCampaign: 'New campaign',
    editCampaign: 'Edit campaign',
    search: 'Search name or subject',
    allStatus: 'All statuses',
    empty: 'No campaigns yet. Try one for users inactive for 14 days.',
    failed: 'Operation failed',
    saved: 'Saved',
    contentRequired: 'Subject and body are required',
    testSent: 'Test email sent to {email}',
    sendQueued: 'Queued for sending. Progress refreshes automatically.',
    noAudience: 'No users match the audience',
    finishedAt: 'Finished',
    progressDetail: 'Failed {failed} · Skipped {skipped}',
    confirmCancel: 'Cancel sending? Emails already sent cannot be recalled; remaining recipients will not receive it.',
    confirmDelete: 'Delete this campaign and its delivery records?',
    confirmSend: {
      title: 'Confirm send',
      counting: 'Counting recipients…',
      message: 'This will send "{subject}" to about {count} users at {rate} emails per minute, taking about {minutes} minutes. Sent emails cannot be recalled; you can only cancel midway.'
    },
    columns: {
      name: 'Name / Subject',
      status: 'Status',
      audience: 'Audience',
      progress: 'Progress',
      createdAt: 'Created',
      actions: 'Actions'
    },
    status: {
      draft: 'Draft',
      pending: 'Queued',
      running: 'Sending',
      completed: 'Completed',
      canceled: 'Canceled',
      failed: 'Failed'
    },
    recipientStatus: {
      pending: 'Pending',
      sent: 'Sent',
      failed: 'Failed',
      skipped: 'Skipped (unsubscribed)'
    },
    summary: {
      all: 'All active users',
      inactive: 'Inactive {days} days',
      neverUsedApi: 'Never used API',
      balance: 'Balance {min} ~ {max}',
      registered: 'By signup date',
      explicit: '{count} selected users'
    },
    actions: {
      send: 'Send',
      cancel: 'Cancel sending',
      recipients: 'Recipients',
      duplicate: 'Duplicate',
      sendTest: 'Send test to me',
      saveDraft: 'Save draft',
      saveAndSend: 'Save and send…'
    },
    audience: {
      title: 'Audience',
      inactive: 'No activity in the last N days',
      days: 'days',
      inactiveHint: 'No login, no dashboard visit and no API call within N days, and signed up more than N days ago',
      neverUsedApi: 'Never called the API',
      balanceRange: 'Balance range',
      min: 'Min',
      max: 'Max',
      registeredRange: 'Signup date',
      explicit: 'Only these users (optional)',
      explicitPlaceholder: 'One user ID or email per line, or comma separated',
      explicitHint: 'When filled, the conditions above apply only within these users. Fill only this to target specific users.',
      includeAdmins: 'Include admin accounts',
      calculate: 'Count users',
      matched: '{count} users match',
      sample: 'Sample (10 most recent signups)'
    },
    content: {
      title: 'Content',
      starter: 'Starter:',
      starterRetention: 'We miss you (win-back)',
      starterFeature: 'Feature announcement',
      name: 'Campaign name (admin only)',
      namePlaceholder: 'Defaults to the subject',
      locale: 'Footer language',
      subject: 'Subject',
      subjectHint: '{count} characters. Keep it short; long subjects get truncated on mobile.',
      layout: 'Layout',
      layoutCard: 'Card layout (recommended)',
      layoutRaw: 'Full HTML',
      headerTitle: 'Header title',
      accent: 'Accent color',
      body: 'Body HTML',
      bodyHintCard: 'Write only the body; the card wrapper and footer are added automatically. Use <a class="button" href="..."> for a button in the accent color.',
      bodyHintRaw: 'Paste a full HTML email. An unsubscribe footer is inserted before </body> automatically; do not remove it.',
      throttle: 'Send rate (emails/minute)',
      throttleHint: 'Default 30. Sending too fast may get throttled or flagged as spam; stay at or below 60 for large sends.'
    },
    placeholders: {
      recipient_name: 'Username (falls back to the email prefix)',
      recipient_email: 'Recipient email',
      site_name: 'Site name',
      site_url: 'Site URL (frontend URL from settings)',
      days_inactive: 'Days since last activity',
      balance: 'Account balance'
    },
    preview: {
      title: 'Live preview (sample data)'
    },
    tips: {
      title: 'Writing tips',
      subject: "Lead with the benefit to the user; {'{{'}recipient_name{'}}'} makes it more personal.",
      spam: 'Avoid words like "free" or "earn money", ALL CAPS and lots of exclamation marks; they trigger spam filters.',
      cta: 'Use one primary button per email that tells users what to do next.',
      footer: 'The unsubscribe link is added automatically and only affects marketing email, not billing or security notices.',
      test: 'Send a test to yourself first and check it on both mobile and desktop.'
    },
    recipients: {
      title: 'Recipients - {name}',
      all: 'All',
      email: 'Recipient',
      lastActivity: 'Last activity',
      sentAt: 'Sent at',
      error: 'Error'
    }
  }
}
