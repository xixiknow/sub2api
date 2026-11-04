package service

import (
	"context"
	"errors"
	"html"
	"regexp"
	"strings"
)

const (
	EmailCampaignLayoutCard = "card"
	EmailCampaignLayoutRaw  = "raw"

	emailCampaignDefaultAccent  = "#4f46e5"
	emailCampaignMaxBodyLength  = 25000
	emailCampaignFooterMarker   = "<!--campaign-footer-->"
	emailCampaignBodyEndPattern = "</body>"
)

var emailCampaignAccentPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// EmailCampaignContent is the admin-authored part of a campaign email.
type EmailCampaignContent struct {
	Subject     string
	BodyHTML    string
	LayoutMode  string
	Title       string
	AccentColor string
	Locale      string
}

// EmailCampaignRecipientVars carries the per-recipient values substituted into a campaign.
type EmailCampaignRecipientVars struct {
	UserID       int64
	Email        string
	Name         string
	DaysInactive string
	Balance      string
}

// ErrEmailCampaignRecipientUnsubscribed marks a recipient that opted out of marketing email.
var ErrEmailCampaignRecipientUnsubscribed = errors.New("recipient unsubscribed from marketing email")

func normalizeEmailCampaignContent(content EmailCampaignContent) EmailCampaignContent {
	content.Subject = strings.TrimSpace(content.Subject)
	content.Title = strings.TrimSpace(content.Title)
	if content.LayoutMode != EmailCampaignLayoutRaw {
		content.LayoutMode = EmailCampaignLayoutCard
	}
	if !emailCampaignAccentPattern.MatchString(strings.TrimSpace(content.AccentColor)) {
		content.AccentColor = emailCampaignDefaultAccent
	}
	content.AccentColor = strings.TrimSpace(content.AccentColor)
	content.Locale = normalizeNotificationLocale(content.Locale)
	return content
}

// ValidateEmailCampaignContent checks subject, body size and placeholders before saving.
func ValidateEmailCampaignContent(content EmailCampaignContent) error {
	content = normalizeEmailCampaignContent(content)
	if strings.TrimSpace(content.BodyHTML) == "" {
		return errors.New("email body cannot be empty")
	}
	if len([]byte(content.BodyHTML)) > emailCampaignMaxBodyLength {
		return errors.New("email body is too large")
	}
	return validateNotificationEmailTemplate(NotificationEmailEventMarketingCampaign, content.Subject, buildEmailCampaignHTML(content))
}

// buildEmailCampaignHTML wraps the body in the chosen layout and always appends
// the unsubscribe footer, which admins cannot remove.
func buildEmailCampaignHTML(content EmailCampaignContent) string {
	content = normalizeEmailCampaignContent(content)
	footer := emailCampaignFooter(content.Locale)
	if content.LayoutMode == EmailCampaignLayoutRaw {
		body := content.BodyHTML
		if idx := strings.LastIndex(strings.ToLower(body), emailCampaignBodyEndPattern); idx >= 0 {
			return body[:idx] + footer + body[idx:]
		}
		return body + footer
	}
	return emailCampaignCard(content.AccentColor, html.EscapeString(content.Title), content.BodyHTML, footer)
}

func emailCampaignFooter(locale string) string {
	if locale == notificationEmailLocaleChinese {
		return emailCampaignFooterMarker + `<div style="padding:18px 32px;color:#a1a1aa;font-size:12px;line-height:1.6;text-align:center;">您收到这封邮件，是因为您是 {{site_name}} 的注册用户。<br>不想再收到此类推广邮件？<a href="{{unsubscribe_url}}" style="color:#71717a;">点此退订</a>（不影响账户安全与账单类通知）。</div>`
	}
	return emailCampaignFooterMarker + `<div style="padding:18px 32px;color:#a1a1aa;font-size:12px;line-height:1.6;text-align:center;">You are receiving this email because you have an account at {{site_name}}.<br>Don't want these updates? <a href="{{unsubscribe_url}}" style="color:#71717a;">Unsubscribe</a> (account and billing notices are not affected).</div>`
}

func emailCampaignCard(accent, title, content, footer string) string {
	header := ""
	if title != "" {
		header = `<div style="background:` + accent + `;color:#ffffff;padding:28px 32px;"><h1 style="margin:0;font-size:24px;line-height:1.3;">` + title + `</h1></div>`
	}
	return `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    .button { display: inline-block; padding: 12px 22px; border-radius: 8px; background: ` + accent + `; color: #ffffff !important; text-decoration: none; font-weight: 600; }
    .muted { color: #71717a; font-size: 13px; }
  </style>
</head>
<body style="margin:0;padding:24px 12px;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif;color:#18181b;">
  <div style="max-width:600px;margin:0 auto;">
    <div style="background:#ffffff;border-radius:12px;overflow:hidden;">
      ` + header + `
      <div style="padding:32px;font-size:15px;line-height:1.75;">` + content + `</div>
    </div>
    ` + footer + `
  </div>
</body>
</html>`
}

func emailCampaignSampleVariables() map[string]string {
	return map[string]string{"site_url": "https://example.com", "days_inactive": "14", "balance": "12.34"}
}

func (s *NotificationEmailService) siteURL(ctx context.Context) string {
	if s == nil || s.settingRepo == nil {
		return ""
	}
	for _, key := range []string{SettingKeyFrontendURL, SettingKeyAPIBaseURL} {
		value, err := s.settingRepo.GetValue(ctx, key)
		if err == nil && strings.TrimSpace(value) != "" {
			return strings.TrimRight(strings.TrimSpace(value), "/")
		}
	}
	return ""
}

// PreviewCampaign renders a campaign with sample recipient values.
func (s *NotificationEmailService) PreviewCampaign(ctx context.Context, content EmailCampaignContent) (NotificationEmailPreview, error) {
	content = normalizeEmailCampaignContent(content)
	if err := ValidateEmailCampaignContent(content); err != nil {
		return NotificationEmailPreview{}, err
	}
	variables := s.sampleVariables(ctx, NotificationEmailEventMarketingCampaign, content.Locale)
	for key, value := range emailCampaignSampleVariables() {
		variables[key] = value
	}
	if siteURL := s.siteURL(ctx); siteURL != "" {
		variables["site_url"] = siteURL
	}
	return renderNotificationEmail(NotificationEmailEventMarketingCampaign, content.Subject, buildEmailCampaignHTML(content), variables, nil)
}

// SendCampaign renders and sends one campaign email. Recipients who unsubscribed
// from marketing email yield ErrEmailCampaignRecipientUnsubscribed. Delivery
// dedupe is handled by the campaign recipient table, not the settings store.
func (s *NotificationEmailService) SendCampaign(ctx context.Context, content EmailCampaignContent, recipient EmailCampaignRecipientVars, checkUnsubscribe bool) error {
	email := strings.TrimSpace(recipient.Email)
	if email == "" {
		return errors.New("recipient email is empty")
	}
	if checkUnsubscribe {
		unsubscribed, err := s.IsUnsubscribed(ctx, email, NotificationEmailEventMarketingCampaign)
		if err != nil {
			return err
		}
		if unsubscribed {
			return ErrEmailCampaignRecipientUnsubscribed
		}
	}
	content = normalizeEmailCampaignContent(content)
	name := strings.TrimSpace(recipient.Name)
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	variables := s.runtimeVariables(ctx, NotificationEmailEventMarketingCampaign, content.Locale, NotificationEmailSendInput{
		Event:          NotificationEmailEventMarketingCampaign,
		RecipientEmail: email,
		RecipientName:  name,
		UserID:         recipient.UserID,
		Variables: map[string]string{
			"site_url":      s.siteURL(ctx),
			"days_inactive": recipient.DaysInactive,
			"balance":       recipient.Balance,
		},
	})
	rendered, err := renderNotificationEmail(NotificationEmailEventMarketingCampaign, content.Subject, buildEmailCampaignHTML(content), variables, nil)
	if err != nil {
		return notificationEmailTemplateErr(err)
	}
	if s.emailService == nil {
		return notificationEmailConfigErr(errors.New("email service is not configured"))
	}
	if err := s.emailService.SendEmail(ctx, email, rendered.Subject, rendered.HTML); err != nil {
		return notificationEmailDeliveryErr(err)
	}
	return nil
}
