package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func ProvideUserHandler(user *service.UserService, auth *service.AuthService, email *service.EmailService, emailCache service.EmailCache, affiliate *service.AffiliateService, quotas service.UserPlatformQuotaRepository, checkin *service.CheckinService) *UserHandler {
	h := NewUserHandler(user, auth, email, emailCache, affiliate, quotas)
	h.checkinService = checkin
	return h
}

func (h *UserHandler) GetCheckin(c *gin.Context)   { h.checkin(c, false) }
func (h *UserHandler) ClaimCheckin(c *gin.Context) { h.checkin(c, true) }
func (h *UserHandler) checkin(c *gin.Context, claim bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var result *service.CheckinStatus
	var err error
	if claim {
		var req struct {
			Token   string `json:"turnstile_token" binding:"max=16384"`
			Ticket  string `json:"tencent_captcha_ticket" binding:"max=16384"`
			Randstr string `json:"tencent_captcha_randstr" binding:"max=1024"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorFrom(c, service.ErrCheckinCaptchaRequired)
			return
		}
		fingerprint, verifyErr := h.authService.VerifyCheckinCaptcha(c.Request.Context(), service.CaptchaProof{TurnstileToken: req.Token, TencentTicket: req.Ticket, TencentRandstr: req.Randstr}, ip.GetClientIP(c))
		if verifyErr != nil {
			response.ErrorFrom(c, verifyErr)
			return
		}
		result, err = h.checkinService.Claim(c.Request.Context(), subject.UserID, fingerprint)
	} else {
		result, err = h.checkinService.Status(c.Request.Context(), subject.UserID)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
