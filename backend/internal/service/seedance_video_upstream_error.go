package service

import (
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	seedanceVideoUpstreamErrorReason = "SEEDANCE_VIDEO_UPSTREAM_ERROR"
	seedanceVideoUpstreamDetailLimit = 2048
)

type SeedanceVideoUpstreamError struct {
	*infraerrors.ApplicationError
	upstreamStatusCode   int
	upstreamErrorMessage string
	upstreamErrorDetail  string
}

func NewSeedanceVideoUpstreamError(statusCode int, body []byte) *SeedanceVideoUpstreamError {
	code := strings.TrimSpace(extractSeedanceVideoErrorCode(body))
	message := strings.TrimSpace(extractUpstreamErrorMessage(body))
	if message == "" {
		message = "seedance upstream request failed"
	}
	if code == "" {
		code = seedanceVideoUpstreamErrorReason
	}
	detail := strings.TrimSpace(string(body))
	if len(detail) > seedanceVideoUpstreamDetailLimit {
		detail = detail[:seedanceVideoUpstreamDetailLimit]
	}
	return &SeedanceVideoUpstreamError{
		ApplicationError:     infraerrors.New(statusCode, code, message),
		upstreamStatusCode:   statusCode,
		upstreamErrorMessage: message,
		upstreamErrorDetail:  detail,
	}
}

func extractSeedanceVideoErrorCode(body []byte) string {
	if code := strings.TrimSpace(gjson.GetBytes(body, "code").String()); code != "" {
		return code
	}
	if code := strings.TrimSpace(extractUpstreamErrorCode(body)); code != "" {
		return code
	}
	return ""
}

func (e *SeedanceVideoUpstreamError) Error() string {
	if e == nil || e.ApplicationError == nil {
		return "seedance video upstream error"
	}
	return e.ApplicationError.Error()
}

func (e *SeedanceVideoUpstreamError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.ApplicationError
}

func (e *SeedanceVideoUpstreamError) UpstreamStatusCode() int {
	if e == nil {
		return 0
	}
	return e.upstreamStatusCode
}

func (e *SeedanceVideoUpstreamError) UpstreamErrorMessage() string {
	if e == nil {
		return ""
	}
	return e.upstreamErrorMessage
}

func (e *SeedanceVideoUpstreamError) UpstreamErrorDetail() string {
	if e == nil {
		return ""
	}
	return e.upstreamErrorDetail
}
