package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	tgshopMaxBody    = 1 << 20 // 1MB
	tgshopSignWindow = 5 * time.Minute
	tgshopStatusOK   = "success"
)

// TGShopWebhookHandler 处理 telegram-shop 的充值回调。
type TGShopWebhookHandler struct {
	tgshopService *service.TGShopService
	secret        string
}

// NewTGShopWebhookHandler 创建 TGShopWebhookHandler。secret 来自配置 tgshop.webhook_secret，
// 必须与 telegram-shop 的 sub2api.webhook_secret 一致。
func NewTGShopWebhookHandler(s *service.TGShopService, cfg *config.Config) *TGShopWebhookHandler {
	return &TGShopWebhookHandler{tgshopService: s, secret: cfg.TGShop.WebhookSecret}
}

type tgshopRechargePayload struct {
	OrderNo string  `json:"order_no"`
	TradeNo string  `json:"trade_no"`
	Email   string  `json:"email"`
	Amount  float64 `json:"amount"`
	// BaseAmount 为实付充值额度（不含活动赠额），用作邀请返利的计提基数。
	// 旧版 telegram-shop 不发送该字段，此时回退为 Amount。
	BaseAmount float64 `json:"base_amount"`
	Status     string  `json:"status"`
}

// resolveRebateBaseAmount 决定邀请返利的计提基数：优先使用实付额度 baseAmount，
// 当其缺失或非正（旧版 telegram-shop 未发送 base_amount）时回退为落账金额 amount。
func resolveRebateBaseAmount(baseAmount, amount float64) float64 {
	if baseAmount > 0 {
		return baseAmount
	}
	return amount
}

// Notify POST /api/v1/payment/webhook/tgshop
func (h *TGShopWebhookHandler) Notify(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, tgshopMaxBody))
	if err != nil {
		c.String(http.StatusBadRequest, "read body failed")
		return
	}

	if h.secret == "" {
		slog.Error("[TGShop] webhook secret not configured, rejecting")
		c.String(http.StatusServiceUnavailable, "tgshop webhook disabled")
		return
	}
	if !h.verifySignature(c, body) {
		slog.Warn("[TGShop] invalid signature")
		c.String(http.StatusUnauthorized, "invalid signature")
		return
	}

	var p tgshopRechargePayload
	if err := json.Unmarshal(body, &p); err != nil {
		c.String(http.StatusBadRequest, "invalid payload")
		return
	}
	if p.Status != tgshopStatusOK || p.Email == "" || p.Amount <= 0 || p.OrderNo == "" {
		c.String(http.StatusBadRequest, "invalid fields")
		return
	}

	// 返利基数取实付额度；旧版回调缺失 base_amount 时回退为落账金额。
	baseAmount := resolveRebateBaseAmount(p.BaseAmount, p.Amount)

	if err := h.tgshopService.Recharge(c.Request.Context(), service.TGShopRechargeInput{
		OrderNo:    p.OrderNo,
		TradeNo:    p.TradeNo,
		Email:      p.Email,
		Amount:     p.Amount,
		BaseAmount: baseAmount,
	}); err != nil {
		slog.Error("[TGShop] recharge failed", "orderNo", p.OrderNo, "error", err)
		c.String(http.StatusInternalServerError, "recharge failed")
		return
	}

	c.String(http.StatusOK, "success")
}

type tgshopBalanceRequest struct {
	Email string `json:"email"`
}

type tgshopBalanceResponse struct {
	Email         string  `json:"email"`
	Balance       float64 `json:"balance"`
	FrozenBalance float64 `json:"frozen_balance"`
	Status        string  `json:"status"`
}

// Balance POST /api/v1/payment/webhook/tgshop/balance
// 只读查询指定 email 用户的余额。复用与 Notify 相同的 HMAC 签名校验（防重放）。
func (h *TGShopWebhookHandler) Balance(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, tgshopMaxBody))
	if err != nil {
		c.String(http.StatusBadRequest, "read body failed")
		return
	}

	if h.secret == "" {
		slog.Error("[TGShop] webhook secret not configured, rejecting")
		c.String(http.StatusServiceUnavailable, "tgshop webhook disabled")
		return
	}
	if !h.verifySignature(c, body) {
		slog.Warn("[TGShop] invalid signature")
		c.String(http.StatusUnauthorized, "invalid signature")
		return
	}

	var p tgshopBalanceRequest
	if err := json.Unmarshal(body, &p); err != nil {
		c.String(http.StatusBadRequest, "invalid payload")
		return
	}
	if p.Email == "" {
		c.String(http.StatusBadRequest, "email is required")
		return
	}

	balance, frozen, err := h.tgshopService.QueryBalance(c.Request.Context(), p.Email)
	if err != nil {
		slog.Warn("[TGShop] query balance failed", "email", p.Email, "error", err)
		c.String(http.StatusNotFound, "user not found")
		return
	}

	c.JSON(http.StatusOK, tgshopBalanceResponse{
		Email:         p.Email,
		Balance:       balance,
		FrozenBalance: frozen,
		Status:        tgshopStatusOK,
	})
}

// verifySignature 校验 HMAC-SHA256 签名 + 5 分钟时间窗，防重放。
// 签名格式：hex(HMAC_SHA256(secret, timestamp + "." + nonce + "." + rawBody))。
func (h *TGShopWebhookHandler) verifySignature(c *gin.Context, body []byte) bool {
	ts := c.GetHeader("X-TGShop-Timestamp")
	nonce := c.GetHeader("X-TGShop-Nonce")
	sig := c.GetHeader("X-TGShop-Signature")
	if ts == "" || nonce == "" || sig == "" {
		return false
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	if d := time.Since(time.Unix(tsInt, 0)); d > tgshopSignWindow || d < -tgshopSignWindow {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write([]byte(nonce))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) == 1
}
