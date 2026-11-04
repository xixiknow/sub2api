package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestEmailCampaignAudienceWhere(t *testing.T) {
	now := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	days := 14
	minBalance := 0.0
	audience := normalizeEmailCampaignAudience(EmailCampaignAudience{
		InactiveDays: &days,
		NeverUsedAPI: true,
		BalanceMin:   &minBalance,
		UserIDs:      []int64{3, 3, 0, 5},
		Emails:       []string{" A@x.com ", "a@x.com", ""},
	})
	require.Equal(t, []int64{3, 5}, audience.UserIDs)
	require.Equal(t, []string{"a@x.com"}, audience.Emails)

	where, args := emailCampaignAudienceWhere(audience, now)
	require.Contains(t, where, "u.role <> 'admin'")
	require.Contains(t, where, "u.last_active_at < $1")
	require.Contains(t, where, "ul.created_at >= $1")
	require.Contains(t, where, "NOT EXISTS (SELECT 1 FROM usage_logs ul WHERE ul.user_id = u.id)")
	require.Contains(t, where, "u.balance >= $2")
	require.Contains(t, where, "(u.id = ANY($3) OR LOWER(u.email) = ANY($4))")
	require.Len(t, args, 4)
	require.Equal(t, now.Add(-14*24*time.Hour), args[0])

	where, args = emailCampaignAudienceWhere(EmailCampaignAudience{IncludeAdmins: true}, now)
	require.NotContains(t, where, "role")
	require.Empty(t, args)
}

func TestBuildEmailCampaignHTMLAlwaysAddsUnsubscribeFooter(t *testing.T) {
	card := buildEmailCampaignHTML(EmailCampaignContent{Subject: "s", BodyHTML: "<p>hi</p>", Title: "<b>T</b>", AccentColor: "red", Locale: "zh"})
	require.Contains(t, card, "{{unsubscribe_url}}")
	require.Contains(t, card, "点此退订")
	require.Contains(t, card, "&lt;b&gt;T&lt;/b&gt;")
	require.Contains(t, card, emailCampaignDefaultAccent, "invalid accent falls back to default")

	raw := buildEmailCampaignHTML(EmailCampaignContent{LayoutMode: EmailCampaignLayoutRaw, BodyHTML: "<html><body><p>x</p></BODY></html>", Locale: "en"})
	footerAt := strings.Index(raw, emailCampaignFooterMarker)
	require.Greater(t, footerAt, strings.Index(raw, "<p>x</p>"))
	require.Less(t, footerAt, strings.Index(raw, "</BODY>"))
	require.Contains(t, raw, "Unsubscribe")
}

func TestValidateEmailCampaignContentRejectsUnknownPlaceholder(t *testing.T) {
	err := ValidateEmailCampaignContent(EmailCampaignContent{Subject: "Hi {{recipient_name}}", BodyHTML: "<p>{{days_inactive}} {{balance}} {{site_url}}</p>"})
	require.NoError(t, err)
	err = ValidateEmailCampaignContent(EmailCampaignContent{Subject: "Hi", BodyHTML: "<p>{{reset_url}}</p>"})
	require.Error(t, err)
	err = ValidateEmailCampaignContent(EmailCampaignContent{Subject: "Hi", BodyHTML: " "})
	require.Error(t, err)
}

func TestEmailCampaignDaysInactive(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	last := now.Add(-15*24*time.Hour - time.Hour)
	require.Equal(t, "15", emailCampaignDaysInactive(&last, now))
	require.Equal(t, "", emailCampaignDaysInactive(nil, now))
}

type fakeEmailCampaignSender struct {
	sent []string
	errs map[string]error
}

func (f *fakeEmailCampaignSender) SendCampaign(_ context.Context, _ EmailCampaignContent, r EmailCampaignRecipientVars, checkUnsubscribe bool) error {
	if !checkUnsubscribe {
		return errors.New("campaign sends must check unsubscribe")
	}
	if err := f.errs[r.Email]; err != nil {
		return err
	}
	f.sent = append(f.sent, r.Email)
	return nil
}

func (f *fakeEmailCampaignSender) PreviewCampaign(context.Context, EmailCampaignContent) (NotificationEmailPreview, error) {
	return NotificationEmailPreview{}, nil
}

func TestEmailCampaignProcessSendsSkipsAndCompletes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sender := &fakeEmailCampaignSender{errs: map[string]error{"b@x.com": ErrEmailCampaignRecipientUnsubscribed}}
	svc := &EmailCampaignService{db: db, sender: sender, instanceID: "w1", now: time.Now, stopCh: make(chan struct{})}

	recipientCols := []string{"id", "user_id", "email", "recipient_name", "balance", "last_activity_at"}
	mock.ExpectQuery("FROM email_campaign_recipients WHERE campaign_id=\\$1 AND status='pending'").
		WithArgs(int64(7), emailCampaignBatchSize).
		WillReturnRows(sqlmock.NewRows(recipientCols).
			AddRow(int64(1), int64(11), "a@x.com", "Alice", 1.5, nil).
			AddRow(int64(2), int64(12), "b@x.com", "", 0.0, time.Now().Add(-20*24*time.Hour)))
	mock.ExpectExec("UPDATE email_campaigns SET heartbeat_at=NOW\\(\\)").WithArgs(int64(7), "w1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE email_campaign_recipients SET status").WithArgs(int64(1), EmailCampaignRecipientSent, "", true).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE email_campaigns SET heartbeat_at=NOW\\(\\)").WithArgs(int64(7), "w1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE email_campaign_recipients SET status").WithArgs(int64(2), EmailCampaignRecipientSkipped, "unsubscribed", false).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("FROM email_campaign_recipients WHERE campaign_id=\\$1 AND status='pending'").
		WillReturnRows(sqlmock.NewRows(recipientCols))
	mock.ExpectExec("UPDATE email_campaigns SET status=\\$3").WithArgs(int64(7), "w1", EmailCampaignStatusCompleted, "").WillReturnResult(sqlmock.NewResult(0, 1))

	svc.process(context.Background(), &EmailCampaign{ID: 7, Subject: "s", BodyHTML: "<p>x</p>", ThrottlePerMinute: emailCampaignMaxThrottle})

	require.Equal(t, []string{"a@x.com"}, sender.sent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEmailCampaignProcessStopsWhenCanceled(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sender := &fakeEmailCampaignSender{}
	svc := &EmailCampaignService{db: db, sender: sender, instanceID: "w1", now: time.Now, stopCh: make(chan struct{})}

	mock.ExpectQuery("FROM email_campaign_recipients").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "email", "recipient_name", "balance", "last_activity_at"}).
			AddRow(int64(1), int64(11), "a@x.com", "", 0.0, nil))
	// Zero rows: the campaign was canceled, so nothing is sent.
	mock.ExpectExec("UPDATE email_campaigns SET heartbeat_at=NOW\\(\\)").WillReturnResult(sqlmock.NewResult(0, 0))

	svc.process(context.Background(), &EmailCampaign{ID: 7, Subject: "s", BodyHTML: "<p>x</p>", ThrottlePerMinute: 30})

	require.Empty(t, sender.sent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationEmailServiceCampaignPreviewAndUnsubscribe(t *testing.T) {
	repo := &subscriptionExpirySettingRepoStub{values: map[string]string{SettingKeyFrontendURL: "https://shop.example.com/"}}
	svc := NewNotificationEmailService(repo, nil)
	content := EmailCampaignContent{
		Subject:  "{{recipient_name}}，好久不见",
		BodyHTML: `<p>已经 {{days_inactive}} 天没见了</p><a class="button" href="{{site_url}}">回来看看</a>`,
		Title:    "好久不见",
		Locale:   "zh",
	}

	preview, err := svc.PreviewCampaign(context.Background(), content)
	require.NoError(t, err)
	require.Equal(t, "张三，好久不见", preview.Subject)
	require.Contains(t, preview.HTML, `href="https://shop.example.com"`)
	require.Contains(t, preview.HTML, "已经 14 天没见了")
	require.Contains(t, preview.HTML, "https://example.com/unsubscribe")

	repo.values[notificationEmailPreferenceKey(NotificationEmailEventMarketingCampaign, "u@x.com")] = "unsubscribed"
	err = svc.SendCampaign(context.Background(), content, EmailCampaignRecipientVars{Email: "U@x.com"}, true)
	require.ErrorIs(t, err, ErrEmailCampaignRecipientUnsubscribed)
}
