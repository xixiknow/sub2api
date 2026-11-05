package handler

import (
	"io"
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const qqbotMaxBody = 1 << 20

// QQBotHandler serves the QQ bot webhook, the user's binding and the admin config.
type QQBotHandler struct {
	svc *service.QQBotService
}

func NewQQBotHandler(svc *service.QQBotService) *QQBotHandler {
	return &QQBotHandler{svc: svc}
}

// Webhook POST /api/v1/qqbot/webhook (called by QQ, ed25519-signed, no JWT).
// QQ expects a bare JSON body, not the site's response envelope.
func (h *QQBotHandler) Webhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, qqbotMaxBody+1))
	if err != nil || len(body) > qqbotMaxBody {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	out, err := h.svc.HandleWebhook(c.Request.Context(),
		c.GetHeader("X-Signature-Ed25519"), c.GetHeader("X-Signature-Timestamp"), body)
	if err != nil {
		code := infraerrors.Code(err)
		if code < 400 || code > 599 {
			code = http.StatusInternalServerError
		}
		c.JSON(code, gin.H{"error": infraerrors.Reason(err)})
		return
	}
	c.JSON(http.StatusOK, out)
}

// GetMyBinding GET /api/v1/user/qqbot-binding
func (h *QQBotHandler) GetMyBinding(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	b, err := h.svc.GetUserBinding(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, b)
}

// UnbindMine DELETE /api/v1/user/qqbot-binding
func (h *QQBotHandler) UnbindMine(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.svc.UnbindUser(c.Request.Context(), subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"bound": false})
}

// GetConfig GET /api/v1/admin/qqbot/config
func (h *QQBotHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateConfig PUT /api/v1/admin/qqbot/config
func (h *QQBotHandler) UpdateConfig(c *gin.Context) {
	var req service.QQBotConfigUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	cfg, err := h.svc.UpdateConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}
