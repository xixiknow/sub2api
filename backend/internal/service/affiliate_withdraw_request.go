package service

import (
	"context"
	"encoding/base64"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 返利提现申请状态
const (
	AffiliateWithdrawStatusPending   = "pending"
	AffiliateWithdrawStatusPaid      = "paid"
	AffiliateWithdrawStatusRejected  = "rejected"
	AffiliateWithdrawStatusCancelled = "cancelled"

	// AffiliateWithdrawQRImageMaxBytes 是收款码 data URL 的长度上限（约 600KB 原图）。
	AffiliateWithdrawQRImageMaxBytes = 800 * 1024
	AffiliateWithdrawAccountMaxRunes = 128
	AffiliateWithdrawNoteMaxRunes    = 500
)

var (
	ErrAffiliateWithdrawDisabled       = infraerrors.Forbidden("AFFILIATE_WITHDRAW_DISABLED", "affiliate withdrawal is disabled")
	ErrAffiliateWithdrawBelowMin       = infraerrors.BadRequest("AFFILIATE_WITHDRAW_BELOW_MIN", "withdrawal amount is below the minimum")
	ErrAffiliateWithdrawPendingExists  = infraerrors.Conflict("AFFILIATE_WITHDRAW_PENDING_EXISTS", "a pending withdrawal request already exists")
	ErrAffiliateWithdrawAccountInvalid = infraerrors.BadRequest("AFFILIATE_WITHDRAW_ACCOUNT_INVALID", "invalid alipay account")
	ErrAffiliateWithdrawQRInvalid      = infraerrors.BadRequest("AFFILIATE_WITHDRAW_QR_INVALID", "invalid alipay payment QR code image")
	ErrAffiliateWithdrawNotFound       = infraerrors.NotFound("AFFILIATE_WITHDRAW_NOT_FOUND", "withdrawal request not found")
	ErrAffiliateWithdrawNotPending     = infraerrors.Conflict("AFFILIATE_WITHDRAW_NOT_PENDING", "withdrawal request has already been processed")
	ErrAffiliateWithdrawReasonRequired = infraerrors.BadRequest("AFFILIATE_WITHDRAW_REASON_REQUIRED", "a rejection reason is required")
	ErrAffiliateWithdrawNoteTooLong    = infraerrors.BadRequest("AFFILIATE_WITHDRAW_NOTE_TOO_LONG", "note is too long")
	ErrAffiliateWithdrawUnavailable    = infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate withdrawal service unavailable")
)

// AffiliateWithdrawRequest 是一笔支付宝提现申请。QRImage 只在管理端详情中返回。
type AffiliateWithdrawRequest struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"user_id"`
	UserEmail     string     `json:"user_email,omitempty"`
	Username      string     `json:"username,omitempty"`
	Amount        float64    `json:"amount"`
	AlipayAccount string     `json:"alipay_account"`
	QRImage       string     `json:"qr_image,omitempty"`
	Status        string     `json:"status"`
	AdminNote     string     `json:"admin_note,omitempty"`
	ReviewedBy    *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// AffiliatePendingWithdraw 是用户详情里展示的待审核提现摘要。
type AffiliatePendingWithdraw struct {
	ID        int64     `json:"id"`
	Amount    float64   `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

// AffiliateWithdrawRequestFilter 是提现申请列表筛选条件。UserID>0 时只看该用户。
type AffiliateWithdrawRequestFilter struct {
	UserID   int64
	Status   string
	Search   string
	StartAt  *time.Time
	EndAt    *time.Time
	Page     int
	PageSize int
}

// AffiliateWithdrawRequestCreate 是提交提现申请的入参（已校验）。
type AffiliateWithdrawRequestCreate struct {
	UserID        int64
	Amount        float64
	AlipayAccount string
	QRImage       string
}

// AffiliateWithdrawRequestRepository 是提现申请的仓储能力，由 affiliateRepository 实现。
// 单独成接口，避免已有的 AffiliateRepository 测试替身全部需要补方法。
type AffiliateWithdrawRequestRepository interface {
	// CreateWithdrawRequest 在一个事务内解冻到期额度、扣减 aff_quota、写 withdraw_request
	// 流水并插入申请单。额度不足返回 ErrAffiliateQuotaInsufficient，已有待审核申请返回
	// ErrAffiliateWithdrawPendingExists。
	CreateWithdrawRequest(ctx context.Context, in AffiliateWithdrawRequestCreate) (*AffiliateWithdrawRequest, error)
	// ResolveWithdrawRequest 把 pending 申请改为 paid/rejected/cancelled。
	// rejected/cancelled 会退回额度并写 withdraw_reverse 流水。expectUserID>0 时要求申请属于该用户。
	ResolveWithdrawRequest(ctx context.Context, id, expectUserID int64, status, note string, reviewerID *int64) (*AffiliateWithdrawRequest, error)
	GetWithdrawRequest(ctx context.Context, id int64) (*AffiliateWithdrawRequest, error)
	ListWithdrawRequests(ctx context.Context, filter AffiliateWithdrawRequestFilter) ([]AffiliateWithdrawRequest, int64, error)
	GetPendingWithdrawRequest(ctx context.Context, userID int64) (*AffiliatePendingWithdraw, error)
}

func (s *AffiliateService) withdrawRepository() AffiliateWithdrawRequestRepository {
	if s == nil {
		return nil
	}
	if s.withdrawRepo != nil {
		return s.withdrawRepo
	}
	if repo, ok := s.repo.(AffiliateWithdrawRequestRepository); ok {
		return repo
	}
	return nil
}

// IsWithdrawEnabled 返回支付宝提现是否可用：需要返利总开关与提现开关同时开启。
func (s *AffiliateService) IsWithdrawEnabled(ctx context.Context) bool {
	if s == nil || s.settingService == nil {
		return false
	}
	return s.IsEnabled(ctx) && s.settingService.IsAffiliateWithdrawEnabled(ctx)
}

func (s *AffiliateService) withdrawMinAmount(ctx context.Context) float64 {
	if s == nil || s.settingService == nil {
		return AffiliateWithdrawMinAmountDefault
	}
	return s.settingService.GetAffiliateWithdrawMinAmount(ctx)
}

// SubmitWithdrawRequest 提交一笔支付宝提现申请，提交即冻结（扣除）对应额度。
func (s *AffiliateService) SubmitWithdrawRequest(ctx context.Context, userID int64, amount float64, alipayAccount, qrImage string) (*AffiliateWithdrawRequest, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	repo := s.withdrawRepository()
	if repo == nil {
		return nil, ErrAffiliateWithdrawUnavailable
	}
	if !s.IsWithdrawEnabled(ctx) {
		return nil, ErrAffiliateWithdrawDisabled
	}
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	amount = roundTo(amount, 8)
	if amount <= 0 {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	if minAmount := s.withdrawMinAmount(ctx); minAmount > 0 && amount < minAmount {
		return nil, ErrAffiliateWithdrawBelowMin.WithMetadata(map[string]string{
			"min_amount": formatAffiliateAmount(minAmount),
		})
	}
	account, err := normalizeAlipayAccount(alipayAccount)
	if err != nil {
		return nil, err
	}
	image, err := ValidateAffiliateWithdrawQRImage(qrImage)
	if err != nil {
		return nil, err
	}

	req, err := repo.CreateWithdrawRequest(ctx, AffiliateWithdrawRequestCreate{
		UserID:        userID,
		Amount:        amount,
		AlipayAccount: account,
		QRImage:       image,
	})
	if err != nil {
		return nil, err
	}
	s.invalidateAffiliateCaches(ctx, userID)
	req.QRImage = ""
	return req, nil
}

// ListUserWithdrawRequests 返回用户自己的提现申请（不含收款码图片）。
func (s *AffiliateService) ListUserWithdrawRequests(ctx context.Context, userID int64, page, pageSize int) ([]AffiliateWithdrawRequest, int64, error) {
	if userID <= 0 {
		return nil, 0, ErrUserNotFound
	}
	repo := s.withdrawRepository()
	if repo == nil {
		return []AffiliateWithdrawRequest{}, 0, nil
	}
	page, pageSize = normalizeAffiliateWithdrawPage(page, pageSize)
	return repo.ListWithdrawRequests(ctx, AffiliateWithdrawRequestFilter{UserID: userID, Page: page, PageSize: pageSize})
}

// CancelWithdrawRequest 由用户撤销自己的待审核申请，额度退回。
func (s *AffiliateService) CancelWithdrawRequest(ctx context.Context, userID, requestID int64) (*AffiliateWithdrawRequest, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	if requestID <= 0 {
		return nil, ErrAffiliateWithdrawNotFound
	}
	repo := s.withdrawRepository()
	if repo == nil {
		return nil, ErrAffiliateWithdrawUnavailable
	}
	req, err := repo.ResolveWithdrawRequest(ctx, requestID, userID, AffiliateWithdrawStatusCancelled, "", nil)
	if err != nil {
		return nil, err
	}
	s.invalidateAffiliateCaches(ctx, userID)
	req.QRImage = ""
	return req, nil
}

// AdminListWithdrawRequests 返回提现申请列表（不含收款码图片）。
func (s *AffiliateService) AdminListWithdrawRequests(ctx context.Context, filter AffiliateWithdrawRequestFilter) ([]AffiliateWithdrawRequest, int64, error) {
	repo := s.withdrawRepository()
	if repo == nil {
		return []AffiliateWithdrawRequest{}, 0, nil
	}
	filter.Page, filter.PageSize = normalizeAffiliateWithdrawPage(filter.Page, filter.PageSize)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Status != "" && !isAffiliateWithdrawStatus(filter.Status) {
		filter.Status = ""
	}
	return repo.ListWithdrawRequests(ctx, filter)
}

// AdminGetWithdrawRequest 返回提现申请详情（含收款码图片）。
func (s *AffiliateService) AdminGetWithdrawRequest(ctx context.Context, requestID int64) (*AffiliateWithdrawRequest, error) {
	if requestID <= 0 {
		return nil, ErrAffiliateWithdrawNotFound
	}
	repo := s.withdrawRepository()
	if repo == nil {
		return nil, ErrAffiliateWithdrawUnavailable
	}
	return repo.GetWithdrawRequest(ctx, requestID)
}

// AdminMarkWithdrawPaid 管理员确认已线下打款。
func (s *AffiliateService) AdminMarkWithdrawPaid(ctx context.Context, requestID, adminID int64, note string) (*AffiliateWithdrawRequest, error) {
	return s.adminResolveWithdraw(ctx, requestID, adminID, AffiliateWithdrawStatusPaid, note, false)
}

// AdminRejectWithdrawRequest 管理员驳回申请，额度退回给用户。
func (s *AffiliateService) AdminRejectWithdrawRequest(ctx context.Context, requestID, adminID int64, reason string) (*AffiliateWithdrawRequest, error) {
	return s.adminResolveWithdraw(ctx, requestID, adminID, AffiliateWithdrawStatusRejected, reason, true)
}

func (s *AffiliateService) adminResolveWithdraw(ctx context.Context, requestID, adminID int64, status, note string, noteRequired bool) (*AffiliateWithdrawRequest, error) {
	if requestID <= 0 {
		return nil, ErrAffiliateWithdrawNotFound
	}
	repo := s.withdrawRepository()
	if repo == nil {
		return nil, ErrAffiliateWithdrawUnavailable
	}
	note = strings.TrimSpace(note)
	if noteRequired && note == "" {
		return nil, ErrAffiliateWithdrawReasonRequired
	}
	if utf8.RuneCountInString(note) > AffiliateWithdrawNoteMaxRunes {
		return nil, ErrAffiliateWithdrawNoteTooLong
	}
	var reviewer *int64
	if adminID > 0 {
		reviewer = &adminID
	}
	req, err := repo.ResolveWithdrawRequest(ctx, requestID, 0, status, note, reviewer)
	if err != nil {
		return nil, err
	}
	s.invalidateAffiliateCaches(ctx, req.UserID)
	return req, nil
}

func (s *AffiliateService) pendingWithdraw(ctx context.Context, userID int64) *AffiliatePendingWithdraw {
	repo := s.withdrawRepository()
	if repo == nil || userID <= 0 {
		return nil
	}
	pending, err := repo.GetPendingWithdrawRequest(ctx, userID)
	if err != nil {
		return nil
	}
	return pending
}

func normalizeAlipayAccount(raw string) (string, error) {
	account := strings.TrimSpace(raw)
	if account == "" || utf8.RuneCountInString(account) > AffiliateWithdrawAccountMaxRunes {
		return "", ErrAffiliateWithdrawAccountInvalid
	}
	if strings.ContainsAny(account, "\r\n\t") {
		return "", ErrAffiliateWithdrawAccountInvalid
	}
	return account, nil
}

var affiliateWithdrawQRPrefixes = []string{
	"data:image/png;base64,",
	"data:image/jpeg;base64,",
	"data:image/jpg;base64,",
	"data:image/webp;base64,",
}

// ValidateAffiliateWithdrawQRImage 校验收款码：必须是 png/jpeg/webp 的 base64 data URL，
// 长度不超过 AffiliateWithdrawQRImageMaxBytes，且 base64 可解码。
func ValidateAffiliateWithdrawQRImage(raw string) (string, error) {
	image := strings.TrimSpace(raw)
	if image == "" || len(image) > AffiliateWithdrawQRImageMaxBytes {
		return "", ErrAffiliateWithdrawQRInvalid
	}
	lower := strings.ToLower(image[:min(len(image), 32)])
	for _, prefix := range affiliateWithdrawQRPrefixes {
		if strings.HasPrefix(lower, prefix) {
			payload := image[len(prefix):]
			if payload == "" {
				return "", ErrAffiliateWithdrawQRInvalid
			}
			if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
				return "", ErrAffiliateWithdrawQRInvalid
			}
			return image, nil
		}
	}
	return "", ErrAffiliateWithdrawQRInvalid
}

func isAffiliateWithdrawStatus(status string) bool {
	switch status {
	case AffiliateWithdrawStatusPending, AffiliateWithdrawStatusPaid, AffiliateWithdrawStatusRejected, AffiliateWithdrawStatusCancelled:
		return true
	default:
		return false
	}
}

func normalizeAffiliateWithdrawPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func formatAffiliateAmount(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
