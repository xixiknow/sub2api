package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrAffiliateProfileNotFound        = infraerrors.NotFound("AFFILIATE_PROFILE_NOT_FOUND", "affiliate profile not found")
	ErrAffiliateCodeInvalid            = infraerrors.BadRequest("AFFILIATE_CODE_INVALID", "invalid affiliate code")
	ErrAffiliateCodeTaken              = infraerrors.Conflict("AFFILIATE_CODE_TAKEN", "affiliate code already in use")
	ErrAffiliateAlreadyBound           = infraerrors.Conflict("AFFILIATE_ALREADY_BOUND", "affiliate inviter already bound")
	ErrAffiliateQuotaEmpty             = infraerrors.BadRequest("AFFILIATE_QUOTA_EMPTY", "no affiliate quota available to transfer")
	ErrRegistrationInviteSeatsEmpty    = infraerrors.BadRequest("REGISTRATION_INVITE_SEATS_EMPTY", "registration invite seats are used up")
	ErrRegistrationSeatQuantityInvalid = infraerrors.BadRequest("REGISTRATION_SEAT_QUANTITY_INVALID", "registration seat quantity is invalid")
)

const (
	affiliateInviteesLimit = 100
	// AffiliateCodeMinLength / AffiliateCodeMaxLength bound both system-generated
	// 12-char codes and admin-customized codes (e.g. "VIP2026").
	AffiliateCodeMinLength               = 4
	AffiliateCodeMaxLength               = 32
	AffiliateRegistrationSeatPurchaseMax = 1000
)

// affiliateCodeValidChar accepts uppercase letters, digits, underscore and dash.
// All input passes through strings.ToUpper before validation, so lowercase from
// users is normalized — admins may supply mixed case in their UI.
var affiliateCodeValidChar = func() [256]bool {
	var tbl [256]bool
	for c := byte('A'); c <= 'Z'; c++ {
		tbl[c] = true
	}
	for c := byte('0'); c <= '9'; c++ {
		tbl[c] = true
	}
	tbl['_'] = true
	tbl['-'] = true
	return tbl
}()

// isValidAffiliateCodeFormat validates code format for both binding (user input)
// and admin updates. Caller is expected to upper-case the input first.
func isValidAffiliateCodeFormat(code string) bool {
	if len(code) < AffiliateCodeMinLength || len(code) > AffiliateCodeMaxLength {
		return false
	}
	for i := 0; i < len(code); i++ {
		if !affiliateCodeValidChar[code[i]] {
			return false
		}
	}
	return true
}

type AffiliateSummary struct {
	UserID                int64     `json:"user_id"`
	AffCode               string    `json:"aff_code"`
	AffCodeCustom         bool      `json:"aff_code_custom"`
	AffRebateRatePercent  *float64  `json:"aff_rebate_rate_percent,omitempty"`
	InviterID             *int64    `json:"inviter_id,omitempty"`
	AffCount              int       `json:"aff_count"`
	AffQuota              float64   `json:"aff_quota"`
	AffFrozenQuota        float64   `json:"aff_frozen_quota"`
	AffHistoryQuota       float64   `json:"aff_history_quota"`
	RegistrationSeatTotal int       `json:"registration_seat_total"`
	RegistrationSeatUsed  int       `json:"registration_seat_used"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type AffiliateInvitee struct {
	UserID      int64      `json:"user_id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	TotalRebate float64    `json:"total_rebate"`
}

type AffiliateLevelInvitee struct {
	UserID          int64      `json:"user_id"`
	Email           string     `json:"email"`
	Username        string     `json:"username"`
	JoinedAt        *time.Time `json:"joined_at,omitempty"`
	TotalRebate     float64    `json:"total_rebate"`
	FrozenRebate    float64    `json:"frozen_rebate"`
	AvailableRebate float64    `json:"available_rebate"`
	OrderCount      int        `json:"order_count"`
	LastRebateAt    *time.Time `json:"last_rebate_at,omitempty"`
	ParentUserID    *int64     `json:"parent_user_id,omitempty"`
	ParentEmail     string     `json:"parent_email,omitempty"`
	ParentUsername  string     `json:"parent_username,omitempty"`
}

type AffiliateLevelDetail struct {
	Level           int                     `json:"level"`
	InviteeCount    int                     `json:"invitee_count"`
	TotalRebate     float64                 `json:"total_rebate"`
	FrozenRebate    float64                 `json:"frozen_rebate"`
	AvailableRebate float64                 `json:"available_rebate"`
	Invitees        []AffiliateLevelInvitee `json:"invitees"`
}

type AffiliateLevelRateRule struct {
	Level             int     `json:"level"`
	RatePercent       float64 `json:"rate_percent"`
	Source            string  `json:"source"`
	Unlocked          bool    `json:"unlocked"`
	UnlockInviteCount int     `json:"unlock_invite_count"`
}

type AffiliateRegistrationInvite struct {
	UserID                    int64  `json:"user_id"`
	AffCode                   string `json:"aff_code"`
	RegistrationSeatTotal     int    `json:"registration_seat_total"`
	RegistrationSeatUsed      int    `json:"registration_seat_used"`
	RegistrationSeatAvailable int    `json:"registration_seat_available"`
}

type AffiliateRegistrationSeatPurchaseResult struct {
	BalanceAfter              float64 `json:"balance"`
	RegistrationSeatCost      float64 `json:"registration_seat_cost"`
	RegistrationSeatTotal     int     `json:"registration_seat_total"`
	RegistrationSeatUsed      int     `json:"registration_seat_used"`
	RegistrationSeatAvailable int     `json:"registration_seat_available"`
}

type AffiliateDetail struct {
	UserID                    int64   `json:"user_id"`
	AffCode                   string  `json:"aff_code"`
	InviterID                 *int64  `json:"inviter_id,omitempty"`
	AffCount                  int     `json:"aff_count"`
	QualifiedAffCount         int     `json:"qualified_aff_count"`
	AffQuota                  float64 `json:"aff_quota"`
	AffFrozenQuota            float64 `json:"aff_frozen_quota"`
	AffHistoryQuota           float64 `json:"aff_history_quota"`
	RegistrationSeatCost      float64 `json:"registration_seat_cost"`
	RegistrationSeatTotal     int     `json:"registration_seat_total"`
	RegistrationSeatUsed      int     `json:"registration_seat_used"`
	RegistrationSeatAvailable int     `json:"registration_seat_available"`
	// EffectiveRebateRatePercent 是当前用户作为邀请人时实际生效的返利比例：
	// 优先用户自己的专属比例（aff_rebate_rate_percent），否则回退到全局比例。
	// 用于在用户的 /affiliate 页面直观展示「分享后能拿到多少」。
	EffectiveRebateRatePercent float64                       `json:"effective_rebate_rate_percent"`
	EffectiveLevelRates        []AffiliateLevelRateRule      `json:"effective_level_rates"`
	LevelRebates               []AffiliateLevelRebateSummary `json:"level_rebates"`
	LevelDetails               []AffiliateLevelDetail        `json:"level_details"`
	Invitees                   []AffiliateInvitee            `json:"invitees"`
}

type AffiliateLevelRebateSummary struct {
	Level        int     `json:"level"`
	RebateAmount float64 `json:"rebate_amount"`
}

type AffiliateClawbackResult struct {
	TotalAmount float64 `json:"total_amount"`
	UserIDs     []int64 `json:"user_ids"`
}

type AffiliateRepository interface {
	EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error)
	GetAffiliateByCode(ctx context.Context, code string) (*AffiliateSummary, error)
	GetRegistrationInviteByCode(ctx context.Context, code string) (*AffiliateRegistrationInvite, error)
	BindInviter(ctx context.Context, userID, inviterID int64) (bool, error)
	ConsumeRegistrationInviteSeat(ctx context.Context, code string, inviteeUserID int64) (*AffiliateRegistrationInvite, error)
	RestoreRegistrationInviteSeat(ctx context.Context, code string, inviteeUserID int64) error
	PurchaseRegistrationSeats(ctx context.Context, userID int64, quantity int, costPerSeat float64) (*AffiliateRegistrationSeatPurchaseResult, error)
	AccrueQuota(ctx context.Context, inviterID, inviteeUserID int64, amount float64, freezeHours int, sourceOrderID *int64, level int, ratePercent float64) (bool, error)
	ClawbackQuotaForOrder(ctx context.Context, sourceOrderID int64, refundRatio float64) (AffiliateClawbackResult, error)
	GetAccruedRebateFromInvitee(ctx context.Context, inviterID, inviteeUserID int64) (float64, error)
	GetLevelRebateSummary(ctx context.Context, userID int64) ([]AffiliateLevelRebateSummary, error)
	GetLevelDetails(ctx context.Context, userID int64, limitPerLevel int) ([]AffiliateLevelDetail, error)
	CountQualifiedInvitees(ctx context.Context, userID int64) (int, error)
	ThawFrozenQuota(ctx context.Context, userID int64) (float64, error)
	TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error)
	ListInvitees(ctx context.Context, inviterID int64, limit int) ([]AffiliateInvitee, error)

	// 管理端：用户级专属配置
	UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error
	ResetUserAffCode(ctx context.Context, userID int64) (string, error)
	SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error
	BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error
	ListUsersWithCustomSettings(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error)
	ListAffiliateInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error)
	ListAffiliateRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error)
	ListAffiliateTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error)
	GetAffiliateUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error)
}

type badgeAffiliateRebateResolver interface {
	ResolveBadgeAffiliateRebateRate(ctx context.Context, userID int64) (*float64, error)
}

// AffiliateAdminFilter 列表筛选条件
type AffiliateAdminFilter struct {
	Search   string
	Page     int
	PageSize int
}

// AffiliateAdminEntry 专属用户列表条目
type AffiliateAdminEntry struct {
	UserID               int64    `json:"user_id"`
	Email                string   `json:"email"`
	Username             string   `json:"username"`
	AffCode              string   `json:"aff_code"`
	AffCodeCustom        bool     `json:"aff_code_custom"`
	AffRebateRatePercent *float64 `json:"aff_rebate_rate_percent,omitempty"`
	AffCount             int      `json:"aff_count"`
}

type AffiliateRecordFilter struct {
	Search   string
	Page     int
	PageSize int
	StartAt  *time.Time
	EndAt    *time.Time
	Level    int
	SortBy   string
	SortDesc bool
}

type AffiliateInviteRecord struct {
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       int64     `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	AffCode         string    `json:"aff_code"`
	TotalRebate     float64   `json:"total_rebate"`
	CreatedAt       time.Time `json:"created_at"`
}

type AffiliateRebateRecord struct {
	OrderID         int64     `json:"order_id"`
	OutTradeNo      string    `json:"out_trade_no"`
	Action          string    `json:"action"`
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       int64     `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	Level           int       `json:"level"`
	RatePercent     float64   `json:"rate_percent"`
	OrderAmount     float64   `json:"order_amount"`
	PayAmount       float64   `json:"pay_amount"`
	RebateAmount    float64   `json:"rebate_amount"`
	PaymentType     string    `json:"payment_type"`
	OrderStatus     string    `json:"order_status"`
	CreatedAt       time.Time `json:"created_at"`
}

type AffiliateTransferRecord struct {
	LedgerID            int64     `json:"ledger_id"`
	UserID              int64     `json:"user_id"`
	UserEmail           string    `json:"user_email"`
	Username            string    `json:"username"`
	Amount              float64   `json:"amount"`
	BalanceAfter        *float64  `json:"balance_after,omitempty"`
	AvailableQuotaAfter *float64  `json:"available_quota_after,omitempty"`
	FrozenQuotaAfter    *float64  `json:"frozen_quota_after,omitempty"`
	HistoryQuotaAfter   *float64  `json:"history_quota_after,omitempty"`
	SnapshotAvailable   bool      `json:"snapshot_available"`
	CurrentBalance      float64   `json:"-"`
	RemainingQuota      float64   `json:"-"`
	FrozenQuota         float64   `json:"-"`
	HistoryQuota        float64   `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
}

type AffiliateUserOverview struct {
	UserID              int64   `json:"user_id"`
	Email               string  `json:"email"`
	Username            string  `json:"username"`
	AffCode             string  `json:"aff_code"`
	RebateRatePercent   float64 `json:"rebate_rate_percent"`
	RebateRateCustom    bool    `json:"-"`
	InvitedCount        int     `json:"invited_count"`
	RebatedInviteeCount int     `json:"rebated_invitee_count"`
	AvailableQuota      float64 `json:"available_quota"`
	HistoryQuota        float64 `json:"history_quota"`
}

type AffiliateService struct {
	repo                 AffiliateRepository
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
}

func NewAffiliateService(repo AffiliateRepository, settingService *SettingService, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCacheService *BillingCacheService) *AffiliateService {
	return &AffiliateService{
		repo:                 repo,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
	}
}

// IsEnabled reports whether the affiliate (邀请返利) feature is turned on.
func (s *AffiliateService) IsEnabled(ctx context.Context) bool {
	if s == nil || s.settingService == nil {
		return AffiliateEnabledDefault
	}
	return s.settingService.IsAffiliateEnabled(ctx)
}

func (s *AffiliateService) EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.EnsureUserAffiliate(ctx, userID)
}

func (s *AffiliateService) GetAffiliateDetail(ctx context.Context, userID int64) (*AffiliateDetail, error) {
	// Lazy thaw: move any matured frozen quota to available before reading.
	if s != nil && s.repo != nil {
		// best-effort: thaw failure is non-fatal
		_, _ = s.repo.ThawFrozenQuota(ctx, userID)
	}

	summary, err := s.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}
	invitees, err := s.listInvitees(ctx, userID)
	if err != nil {
		return nil, err
	}
	levelRebates, err := s.repo.GetLevelRebateSummary(ctx, userID)
	if err != nil {
		return nil, err
	}
	levelDetails, err := s.listLevelDetails(ctx, userID)
	if err != nil {
		return nil, err
	}
	qualifiedAffCount, err := s.repo.CountQualifiedInvitees(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &AffiliateDetail{
		UserID:                     summary.UserID,
		AffCode:                    summary.AffCode,
		InviterID:                  summary.InviterID,
		AffCount:                   summary.AffCount,
		QualifiedAffCount:          qualifiedAffCount,
		AffQuota:                   summary.AffQuota,
		AffFrozenQuota:             summary.AffFrozenQuota,
		AffHistoryQuota:            summary.AffHistoryQuota,
		RegistrationSeatCost:       s.registrationSeatCost(ctx),
		RegistrationSeatTotal:      summary.RegistrationSeatTotal,
		RegistrationSeatUsed:       summary.RegistrationSeatUsed,
		RegistrationSeatAvailable:  registrationSeatAvailable(summary.RegistrationSeatTotal, summary.RegistrationSeatUsed),
		EffectiveRebateRatePercent: s.resolveRebateRatePercent(ctx, summary),
		EffectiveLevelRates:        s.effectiveLevelRateRules(ctx, summary, qualifiedAffCount),
		LevelRebates:               levelRebates,
		LevelDetails:               levelDetails,
		Invitees:                   invitees,
	}, nil
}

func (s *AffiliateService) ValidateRegistrationInviteCode(ctx context.Context, rawCode string) (*AffiliateRegistrationInvite, error) {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" || !isValidAffiliateCodeFormat(code) {
		return nil, ErrAffiliateCodeInvalid
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	invite, err := s.repo.GetRegistrationInviteByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if s.registrationSeatCost(ctx) <= 0 {
		return invite, nil
	}
	if invite.RegistrationSeatAvailable <= 0 {
		return nil, ErrRegistrationInviteSeatsEmpty
	}
	return invite, nil
}

func (s *AffiliateService) ConsumeRegistrationInviteSeat(ctx context.Context, rawCode string, inviteeUserID int64) (*AffiliateRegistrationInvite, error) {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" || !isValidAffiliateCodeFormat(code) {
		return nil, ErrAffiliateCodeInvalid
	}
	if inviteeUserID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if s.registrationSeatCost(ctx) <= 0 {
		return s.repo.GetRegistrationInviteByCode(ctx, code)
	}
	return s.repo.ConsumeRegistrationInviteSeat(ctx, code, inviteeUserID)
}

func (s *AffiliateService) RestoreRegistrationInviteSeat(ctx context.Context, rawCode string, inviteeUserID int64) error {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" || inviteeUserID <= 0 || !isValidAffiliateCodeFormat(code) {
		return nil
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if s.registrationSeatCost(ctx) <= 0 {
		return nil
	}
	return s.repo.RestoreRegistrationInviteSeat(ctx, code, inviteeUserID)
}

func (s *AffiliateService) PurchaseRegistrationSeats(ctx context.Context, userID int64, quantity int) (*AffiliateRegistrationSeatPurchaseResult, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if quantity <= 0 || quantity > AffiliateRegistrationSeatPurchaseMax {
		return nil, ErrRegistrationSeatQuantityInvalid
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	cost := s.registrationSeatCost(ctx)
	result, err := s.repo.PurchaseRegistrationSeats(ctx, userID, quantity, cost)
	if err != nil {
		return nil, err
	}
	result.RegistrationSeatCost = cost
	s.invalidateAffiliateCaches(ctx, userID)
	return result, nil
}

func (s *AffiliateService) BindInviterByCode(ctx context.Context, userID int64, rawCode string) error {
	return s.bindInviterByCode(ctx, userID, rawCode, true)
}

func (s *AffiliateService) BindRegistrationInviterByCode(ctx context.Context, userID int64, rawCode string) error {
	return s.bindInviterByCode(ctx, userID, rawCode, false)
}

func (s *AffiliateService) bindInviterByCode(ctx context.Context, userID int64, rawCode string, requireFeatureEnabled bool) error {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	// 总开关关闭时，注册阶段静默忽略 aff 参数（不报错，避免阻断注册流程）
	if requireFeatureEnabled && !s.IsEnabled(ctx) {
		return nil
	}
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}

	selfSummary, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return err
	}
	if selfSummary.InviterID != nil {
		return nil
	}

	inviterSummary, err := s.repo.GetAffiliateByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrAffiliateProfileNotFound) {
			return ErrAffiliateCodeInvalid
		}
		return err
	}
	if inviterSummary == nil || inviterSummary.UserID <= 0 || inviterSummary.UserID == userID {
		return ErrAffiliateCodeInvalid
	}

	bound, err := s.repo.BindInviter(ctx, userID, inviterSummary.UserID)
	if err != nil {
		return err
	}
	if !bound {
		return ErrAffiliateAlreadyBound
	}
	return nil
}

func (s *AffiliateService) AccrueInviteRebate(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64) (float64, error) {
	return s.AccrueInviteRebateForOrder(ctx, inviteeUserID, baseRechargeAmount, nil)
}

func (s *AffiliateService) AccrueInviteRebateForOrder(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64, sourceOrderID *int64) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if inviteeUserID <= 0 || baseRechargeAmount <= 0 || math.IsNaN(baseRechargeAmount) || math.IsInf(baseRechargeAmount, 0) {
		return 0, nil
	}
	// 总开关关闭时，新充值不再产生返利
	if !s.IsEnabled(ctx) {
		return 0, nil
	}

	inviteeSummary, err := s.repo.EnsureUserAffiliate(ctx, inviteeUserID)
	if err != nil {
		return 0, err
	}
	if inviteeSummary.InviterID == nil || *inviteeSummary.InviterID <= 0 {
		return 0, nil
	}

	// 有效期检查：超过返利有效期后不再产生返利
	if s.settingService != nil {
		if durationDays := s.settingService.GetAffiliateRebateDurationDays(ctx); durationDays > 0 {
			if time.Now().After(inviteeSummary.CreatedAt.AddDate(0, 0, durationDays)) {
				return 0, nil
			}
		}
	}

	var freezeHours int
	if s.settingService != nil {
		freezeHours = s.settingService.GetAffiliateRebateFreezeHours(ctx)
	}

	currentInviterID := *inviteeSummary.InviterID
	var totalRebate float64
	for level := 1; level <= AffiliateLevelsMax && currentInviterID > 0; level++ {
		inviterSummary, err := s.repo.EnsureUserAffiliate(ctx, currentInviterID)
		if err != nil {
			return 0, err
		}
		qualifiedAffCount := 0
		if level > 1 {
			qualifiedAffCount, err = s.repo.CountQualifiedInvitees(ctx, currentInviterID)
			if err != nil {
				return 0, err
			}
		}
		rebateRatePercent := s.effectiveRateForLevel(ctx, inviterSummary, level, qualifiedAffCount)
		rebate := roundTo(baseRechargeAmount*(rebateRatePercent/100), 8)
		if rebate > 0 {
			rebate, err = s.applyPerInviteeCap(ctx, currentInviterID, inviteeUserID, rebate)
			if err != nil {
				return 0, err
			}
			if rebate > 0 {
				applied, err := s.repo.AccrueQuota(ctx, currentInviterID, inviteeUserID, rebate, freezeHours, sourceOrderID, level, rebateRatePercent)
				if err != nil {
					return 0, err
				}
				if applied {
					totalRebate = roundTo(totalRebate+rebate, 8)
				}
			}
		}
		if inviterSummary.InviterID == nil {
			break
		}
		currentInviterID = *inviterSummary.InviterID
	}
	return totalRebate, nil
}

func (s *AffiliateService) ClawbackRebateForOrder(ctx context.Context, sourceOrderID int64, refundAmount, orderAmount float64) (AffiliateClawbackResult, error) {
	var zero AffiliateClawbackResult
	if s == nil || s.repo == nil {
		return zero, nil
	}
	if sourceOrderID <= 0 || refundAmount <= 0 || orderAmount <= 0 ||
		math.IsNaN(refundAmount) || math.IsInf(refundAmount, 0) ||
		math.IsNaN(orderAmount) || math.IsInf(orderAmount, 0) {
		return zero, nil
	}
	refundRatio := refundAmount / orderAmount
	if refundRatio <= 0 || math.IsNaN(refundRatio) || math.IsInf(refundRatio, 0) {
		return zero, nil
	}
	if refundRatio > 1 {
		refundRatio = 1
	}

	result, err := s.repo.ClawbackQuotaForOrder(ctx, sourceOrderID, refundRatio)
	if err != nil {
		return zero, err
	}
	for _, userID := range uniquePositiveInt64s(result.UserIDs) {
		s.invalidateAffiliateCaches(ctx, userID)
	}
	return result, nil
}

func (s *AffiliateService) applyPerInviteeCap(ctx context.Context, inviterID, inviteeUserID int64, rebate float64) (float64, error) {
	if s == nil || s.settingService == nil {
		return rebate, nil
	}
	perInviteeCap := s.settingService.GetAffiliateRebatePerInviteeCap(ctx)
	if perInviteeCap <= 0 {
		return rebate, nil
	}
	existing, err := s.repo.GetAccruedRebateFromInvitee(ctx, inviterID, inviteeUserID)
	if err != nil {
		return 0, err
	}
	if existing >= perInviteeCap {
		return 0, nil
	}
	if remaining := perInviteeCap - existing; rebate > remaining {
		return roundTo(remaining, 8), nil
	}
	return rebate, nil
}

// resolveRebateRatePercent returns the inviter's exclusive rate when set,
// otherwise the best automatic badge benefit, then the global setting value.
func (s *AffiliateService) resolveRebateRatePercent(ctx context.Context, inviter *AffiliateSummary) float64 {
	if inviter != nil && inviter.AffRebateRatePercent != nil {
		v := *inviter.AffRebateRatePercent
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return s.globalRebateRatePercent(ctx)
		}
		return clampAffiliateRebateRate(v)
	}
	if inviter != nil {
		if resolver, ok := s.repo.(badgeAffiliateRebateResolver); ok {
			if badgeRate, err := resolver.ResolveBadgeAffiliateRebateRate(ctx, inviter.UserID); err == nil && badgeRate != nil {
				return clampAffiliateRebateRate(*badgeRate)
			} else if err != nil {
				logger.LegacyPrintf("service.affiliate", "resolve badge affiliate rebate failed, fallback to global: user=%d err=%v", inviter.UserID, err)
			}
		}
	}
	return s.globalRebateRatePercent(ctx)
}

func (s *AffiliateService) hasBadgeRebateRate(ctx context.Context, inviter *AffiliateSummary) bool {
	if inviter == nil || inviter.AffRebateRatePercent != nil {
		return false
	}
	resolver, ok := s.repo.(badgeAffiliateRebateResolver)
	if !ok {
		return false
	}
	badgeRate, err := resolver.ResolveBadgeAffiliateRebateRate(ctx, inviter.UserID)
	if err != nil {
		logger.LegacyPrintf("service.affiliate", "resolve badge affiliate rebate source failed: user=%d err=%v", inviter.UserID, err)
		return false
	}
	return badgeRate != nil
}

// globalRebateRatePercent reads the system-wide rebate rate via SettingService,
// returning the documented default when SettingService is unavailable.
func (s *AffiliateService) globalRebateRatePercent(ctx context.Context) float64 {
	if s == nil || s.settingService == nil {
		return AffiliateRebateRateDefault
	}
	return s.settingService.GetAffiliateRebateRatePercent(ctx)
}

func (s *AffiliateService) resolveLevelRebateRates(ctx context.Context) []float64 {
	if s == nil || s.settingService == nil {
		return defaultAffiliateLevelRates()
	}
	return s.settingService.GetAffiliateLevelRates(ctx)
}

func (s *AffiliateService) effectiveRateForLevel(ctx context.Context, summary *AffiliateSummary, level, qualifiedAffCount int) float64 {
	if level <= 1 {
		return s.resolveRebateRatePercent(ctx, summary)
	}
	levelRates := s.resolveLevelRebateRates(ctx)
	if level > len(levelRates) || !affiliateLevelUnlocked(qualifiedAffCount, level) {
		return 0
	}
	return clampAffiliateRebateRate(levelRates[level-1])
}

func (s *AffiliateService) registrationSeatCost(ctx context.Context) float64 {
	if s == nil || s.settingService == nil {
		return AffiliateRegistrationSeatCostDefault
	}
	return s.settingService.GetAffiliateRegistrationSeatCost(ctx)
}

func (s *AffiliateService) effectiveLevelRateRules(ctx context.Context, summary *AffiliateSummary, qualifiedAffCount int) []AffiliateLevelRateRule {
	out := make([]AffiliateLevelRateRule, AffiliateLevelsMax)
	for i := 0; i < AffiliateLevelsMax; i++ {
		level := i + 1
		unlocked := affiliateLevelUnlocked(qualifiedAffCount, level)
		rate := s.effectiveRateForLevel(ctx, summary, level, qualifiedAffCount)
		source := "global"
		if i == 0 {
			if summary != nil && summary.AffRebateRatePercent != nil {
				source = "exclusive"
			} else if s.hasBadgeRebateRate(ctx, summary) {
				source = "badge"
			}
		} else if !unlocked {
			source = "locked"
		}
		out[i] = AffiliateLevelRateRule{
			Level:             level,
			RatePercent:       clampAffiliateRebateRate(rate),
			Source:            source,
			Unlocked:          unlocked,
			UnlockInviteCount: affiliateLevelUnlockInviteCount(level),
		}
	}
	return out
}

func affiliateLevelUnlockInviteCount(level int) int {
	switch level {
	case 2:
		return AffiliateLevel2UnlockInviteCount
	case 3:
		return AffiliateLevel3UnlockInviteCount
	default:
		return 0
	}
}

func affiliateLevelUnlocked(qualifiedAffCount, level int) bool {
	if level <= 1 {
		return true
	}
	switch level {
	case 2:
		return qualifiedAffCount >= AffiliateLevel2UnlockInviteCount
	case 3:
		return qualifiedAffCount >= AffiliateLevel3UnlockInviteCount
	default:
		return false
	}
}

func registrationSeatAvailable(total, used int) int {
	available := total - used
	if available < 0 {
		return 0
	}
	return available
}

func defaultAffiliateLevelRates() []float64 {
	return []float64{5, 1, 0.5}
}

func parseAffiliateLevelRates(raw string) []float64 {
	var values []float64
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &values); err != nil {
		return defaultAffiliateLevelRates()
	}
	out := defaultAffiliateLevelRates()
	for i := 0; i < len(values) && i < AffiliateLevelsMax; i++ {
		out[i] = clampAffiliateRebateRate(values[i])
	}
	return out
}

func (s *AffiliateService) TransferAffiliateQuota(ctx context.Context, userID int64) (float64, float64, error) {
	if s == nil || s.repo == nil {
		return 0, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}

	transferred, balance, err := s.repo.TransferQuotaToBalance(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if transferred > 0 {
		s.invalidateAffiliateCaches(ctx, userID)
	}
	return transferred, balance, nil
}

func (s *AffiliateService) listInvitees(ctx context.Context, inviterID int64) ([]AffiliateInvitee, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	invitees, err := s.repo.ListInvitees(ctx, inviterID, affiliateInviteesLimit)
	if err != nil {
		return nil, err
	}
	for i := range invitees {
		invitees[i].Email = maskEmail(invitees[i].Email)
	}
	return invitees, nil
}

func (s *AffiliateService) listLevelDetails(ctx context.Context, userID int64) ([]AffiliateLevelDetail, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	details, err := s.repo.GetLevelDetails(ctx, userID, affiliateInviteesLimit)
	if err != nil {
		return nil, err
	}
	return normalizeAffiliateLevelDetails(details), nil
}

func normalizeAffiliateLevelDetails(details []AffiliateLevelDetail) []AffiliateLevelDetail {
	out := make([]AffiliateLevelDetail, AffiliateLevelsMax)
	for i := range out {
		out[i] = AffiliateLevelDetail{
			Level:    i + 1,
			Invitees: make([]AffiliateLevelInvitee, 0),
		}
	}
	for _, detail := range details {
		if detail.Level <= 0 || detail.Level > AffiliateLevelsMax {
			continue
		}
		if detail.Invitees == nil {
			detail.Invitees = make([]AffiliateLevelInvitee, 0)
		}
		for i := range detail.Invitees {
			detail.Invitees[i].Email = maskEmail(detail.Invitees[i].Email)
			detail.Invitees[i].ParentEmail = maskEmail(detail.Invitees[i].ParentEmail)
		}
		out[detail.Level-1] = detail
	}
	return out
}

func roundTo(v float64, scale int) float64 {
	factor := math.Pow10(scale)
	return math.Round(v*factor) / factor
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	at := strings.Index(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return "***"
	}

	local := email[:at]
	domain := email[at+1:]
	dot := strings.LastIndex(domain, ".")

	maskedLocal := maskSegment(local)
	if dot <= 0 || dot >= len(domain)-1 {
		return maskedLocal + "@" + maskSegment(domain)
	}

	domainName := domain[:dot]
	tld := domain[dot:]
	return maskedLocal + "@" + maskSegment(domainName) + tld
}

func maskSegment(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return "***"
	}
	if len(r) == 1 {
		return string(r[0]) + "***"
	}
	return string(r[0]) + "***"
}

func (s *AffiliateService) invalidateAffiliateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.affiliate", "[Affiliate] Failed to invalidate billing cache for user %d: %v", userID, err)
		}
	}
}

func uniquePositiveInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// =========================
// Admin: 专属配置管理
// =========================

// validateExclusiveRate ensures a per-user override is finite and within
// [Min, Max]. nil is always valid (means "clear / fall back to global").
func validateExclusiveRate(ratePercent *float64) error {
	if ratePercent == nil {
		return nil
	}
	v := *ratePercent
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return infraerrors.BadRequest("INVALID_RATE", "invalid rebate rate")
	}
	if v < AffiliateRebateRateMin || v > AffiliateRebateRateMax {
		return infraerrors.BadRequest("INVALID_RATE", "rebate rate out of range")
	}
	return nil
}

// AdminUpdateUserAffCode 管理员改写用户的邀请码（专属邀请码）。
func (s *AffiliateService) AdminUpdateUserAffCode(ctx context.Context, userID int64, rawCode string) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}
	return s.repo.UpdateUserAffCode(ctx, userID, code)
}

// AdminResetUserAffCode 重置用户邀请码为系统随机码。
func (s *AffiliateService) AdminResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if s == nil || s.repo == nil {
		return "", infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ResetUserAffCode(ctx, userID)
}

// AdminSetUserRebateRate 设置/清除用户专属返利比例。ratePercent==nil 表示清除。
func (s *AffiliateService) AdminSetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	return s.repo.SetUserRebateRate(ctx, userID, ratePercent)
}

// AdminBatchSetUserRebateRate 批量设置/清除用户专属返利比例。
func (s *AffiliateService) AdminBatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	cleaned := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 {
			cleaned = append(cleaned, uid)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return s.repo.BatchSetUserRebateRate(ctx, cleaned, ratePercent)
}

// AdminListCustomUsers 列出有专属配置的用户。
func (s *AffiliateService) AdminListCustomUsers(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListUsersWithCustomSettings(ctx, filter)
}

func (s *AffiliateService) AdminListInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateInviteRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateRebateRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateTransferRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminGetUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	overview, err := s.repo.GetAffiliateUserOverview(ctx, userID)
	if err != nil {
		return nil, err
	}
	if overview != nil {
		if !overview.RebateRateCustom {
			overview.RebateRatePercent = s.globalRebateRatePercent(ctx)
		}
		overview.RebateRatePercent = clampAffiliateRebateRate(overview.RebateRatePercent)
	}
	return overview, nil
}

func normalizeAffiliateRecordFilter(filter AffiliateRecordFilter) AffiliateRecordFilter {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.SortBy = strings.TrimSpace(filter.SortBy)
	return filter
}
