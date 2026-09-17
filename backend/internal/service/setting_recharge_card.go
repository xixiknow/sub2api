package service

import (
	"net/url"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func normalizeRechargeCardOpenMode(mode string) string {
	if mode == "" {
		return "iframe"
	}
	return mode
}

// ValidateRechargeCardSettings validates both full settings and merged partial updates.
func ValidateRechargeCardSettings(enabled bool, rawURL, mode string) error {
	if mode != "iframe" && mode != "new_tab" {
		return infraerrors.BadRequest("INVALID_RECHARGE_CARD_MODE", "Recharge card open mode must be iframe or new_tab")
	}
	if enabled && rawURL == "" {
		return infraerrors.BadRequest("INVALID_RECHARGE_CARD_URL", "Recharge card URL is required when enabled")
	}
	if rawURL != "" {
		u, err := url.Parse(rawURL)
		if err != nil || u.Hostname() == "" || u.User != nil || strings.ContainsAny(rawURL, "\r\n\t\\") || (u.Scheme != "http" && u.Scheme != "https") {
			return infraerrors.BadRequest("INVALID_RECHARGE_CARD_URL", "Recharge card URL must be an absolute http(s) URL")
		}
	}
	return nil
}
