package service

import (
	"net/url"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func NormalizeDramaVideoPublicBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || strings.TrimSpace(u.Host) == "" {
		return "", infraerrors.BadRequest("DRAMA_VIDEO_PUBLIC_BASE_URL", "public_base_url must be an absolute HTTPS origin")
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", infraerrors.BadRequest("DRAMA_VIDEO_PUBLIC_BASE_URL", "public_base_url must use HTTPS so upstream can fetch assets")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", infraerrors.BadRequest("DRAMA_VIDEO_PUBLIC_BASE_URL", "public_base_url must be an origin only, without userinfo, query, or fragment")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return "", infraerrors.BadRequest("DRAMA_VIDEO_PUBLIC_BASE_URL", "public_base_url must be a public host; localhost is not reachable by upstream")
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host, "/"), nil
}

func DramaVideoPublicBaseURLUsable(raw string) bool {
	normalized, err := NormalizeDramaVideoPublicBaseURL(raw)
	return err == nil && normalized != ""
}
