package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrCheckinCaptchaRequired = infraerrors.BadRequest("CHECKIN_CAPTCHA_REQUIRED", "Complete the check-in verification on the website")

// VerifyCheckinCaptcha always fails closed, even when optional login captcha is off.
// The returned fingerprint is persisted atomically with the award, never the raw proof.
func (s *AuthService) VerifyCheckinCaptcha(ctx context.Context, proof CaptchaProof, remoteIP string) (string, error) {
	if s == nil || s.settingService == nil {
		return "", ErrServiceUnavailable
	}
	proof.TurnstileToken = strings.TrimSpace(proof.TurnstileToken)
	proof.TencentTicket = strings.TrimSpace(proof.TencentTicket)
	proof.TencentRandstr = strings.TrimSpace(proof.TencentRandstr)
	tokenMode := proof.TurnstileToken != ""
	ticketMode := proof.TencentTicket != "" && proof.TencentRandstr != ""
	if tokenMode == ticketMode || (tokenMode && (proof.TencentTicket != "" || proof.TencentRandstr != "")) {
		return "", ErrCheckinCaptchaRequired
	}
	if err := s.verifyCaptcha(ctx, proof, remoteIP, true); err != nil {
		return "", err
	}
	key := "ticket:" + proof.TencentTicket
	if tokenMode {
		token := proof.TurnstileToken
		// Aliyun's proof is JSON: whitespace/key-order changes should not change its identity.
		var value any
		if json.Unmarshal([]byte(token), &value) == nil {
			if canonical, err := json.Marshal(value); err == nil {
				token = string(canonical)
			}
		}
		key = "token:" + token
	}
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:]), nil
}
