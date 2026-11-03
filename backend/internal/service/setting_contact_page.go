package service

import (
	"context"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrContactPageDisabled = infraerrors.Forbidden("CONTACT_PAGE_DISABLED", "Contact page is disabled")

// ContactPage is the admin-configured content of the user-side "contact us" page.
// The image is served here instead of public settings to keep injected HTML small.
type ContactPage struct {
	Image       string `json:"image"`
	Link        string `json:"link"`
	Text        string `json:"text"`
	ContactInfo string `json:"contact_info"`
}

// IsCheckinEnabled 检查是否启用每日签到（默认关闭）
func (s *SettingService) IsCheckinEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyCheckinEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// GetContactPage returns the contact page content, or ErrContactPageDisabled.
func (s *SettingService) GetContactPage(ctx context.Context) (*ContactPage, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyContactPageEnabled,
		SettingKeyContactPageImage,
		SettingKeyContactPageLink,
		SettingKeyContactPageText,
		SettingKeyContactInfo,
	})
	if err != nil {
		return nil, err
	}
	if values[SettingKeyContactPageEnabled] != "true" {
		return nil, ErrContactPageDisabled
	}
	return &ContactPage{
		Image:       strings.TrimSpace(values[SettingKeyContactPageImage]),
		Link:        strings.TrimSpace(values[SettingKeyContactPageLink]),
		Text:        values[SettingKeyContactPageText],
		ContactInfo: strings.TrimSpace(values[SettingKeyContactInfo]),
	}, nil
}
