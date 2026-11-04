// Starter content for marketing campaigns. Placeholders must stay within the
// marketing.campaign allow-list enforced by the backend.

export type StarterTemplateKey = 'retention' | 'feature'

export interface StarterTemplate {
  subject: string
  title: string
  accent: string
  body: string
}

export const CAMPAIGN_PLACEHOLDERS = [
  'recipient_name',
  'recipient_email',
  'site_name',
  'site_url',
  'days_inactive',
  'balance'
] as const

const zh: Record<StarterTemplateKey, StarterTemplate> = {
  retention: {
    subject: '{{recipient_name}}，好久不见，{{site_name}} 有些新变化',
    title: '好久不见',
    accent: '#4f46e5',
    body: `<p>{{recipient_name}}，您好：</p>
<p>您已经有 <strong>{{days_inactive}}</strong> 天没有使用 {{site_name}} 了。这段时间我们做了不少改进，想邀请您回来看看：</p>
<ul>
  <li><strong>更多模型</strong>：在这里写新增的模型或能力</li>
  <li><strong>更稳定</strong>：在这里写可用性、速度方面的提升</li>
  <li><strong>更划算</strong>：在这里写价格或计费上的调整</li>
</ul>
<p>您账户里的余额 <strong>{{balance}}</strong> 仍然可用，原来的 API Key 不用做任何修改就能继续调用。</p>
<p style="text-align:center;margin:28px 0;"><a class="button" href="{{site_url}}">回到控制台</a></p>
<p class="muted">使用中遇到任何问题，欢迎通过站内的联系方式找我们。</p>`
  },
  feature: {
    subject: '【{{site_name}}】新功能上线：在这里写一句话亮点',
    title: '新功能上线',
    accent: '#0ea5e9',
    body: `<p>{{recipient_name}}，您好：</p>
<p>我们刚刚上线了一些新功能，简单介绍一下：</p>
<h3 style="margin:24px 0 8px;font-size:17px;">1. 功能名称</h3>
<p>一两句话说明它能帮用户解决什么问题。</p>
<h3 style="margin:24px 0 8px;font-size:17px;">2. 功能名称</h3>
<p>一两句话说明它能帮用户解决什么问题。</p>
<p style="text-align:center;margin:28px 0;"><a class="button" href="{{site_url}}">立即体验</a></p>`
  }
}

const en: Record<StarterTemplateKey, StarterTemplate> = {
  retention: {
    subject: "{{recipient_name}}, it's been a while — here's what's new at {{site_name}}",
    title: "It's been a while",
    accent: '#4f46e5',
    body: `<p>Hi {{recipient_name}},</p>
<p>It has been <strong>{{days_inactive}}</strong> days since you last used {{site_name}}. We've shipped quite a few improvements since then:</p>
<ul>
  <li><strong>More models</strong>: describe newly added models or capabilities</li>
  <li><strong>More reliable</strong>: describe uptime or latency improvements</li>
  <li><strong>Better pricing</strong>: describe pricing or billing changes</li>
</ul>
<p>Your balance of <strong>{{balance}}</strong> is still available, and your existing API keys keep working without any changes.</p>
<p style="text-align:center;margin:28px 0;"><a class="button" href="{{site_url}}">Back to dashboard</a></p>
<p class="muted">If you run into anything, reach out through the contact options on our site.</p>`
  },
  feature: {
    subject: '[{{site_name}}] New: one-line highlight here',
    title: "What's new",
    accent: '#0ea5e9',
    body: `<p>Hi {{recipient_name}},</p>
<p>We just shipped a few new features:</p>
<h3 style="margin:24px 0 8px;font-size:17px;">1. Feature name</h3>
<p>One or two sentences on the problem it solves.</p>
<h3 style="margin:24px 0 8px;font-size:17px;">2. Feature name</h3>
<p>One or two sentences on the problem it solves.</p>
<p style="text-align:center;margin:28px 0;"><a class="button" href="{{site_url}}">Try it now</a></p>`
  }
}

export function getStarterTemplate(key: StarterTemplateKey, locale: 'zh' | 'en'): StarterTemplate {
  return (locale === 'zh' ? zh : en)[key]
}
