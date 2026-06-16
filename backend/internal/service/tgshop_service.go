package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// TGShopService 处理来自 telegram-shop 的充值回调。
//
// 充值通过复用既有「余额兑换码」机制实现：以 telegram-shop 订单号派生出
// 唯一兑换码（tgshop-{order_no}），创建后立即兑换到目标用户账户。兑换码
// 的唯一约束 + 已使用状态共同保证幂等——同一订单重复回调只会充值一次。
type TGShopService struct {
	userRepo         UserRepository
	redeemService    *RedeemService
	affiliateService *AffiliateService
	paymentService   *PaymentService
}

// NewTGShopService 创建 TGShopService。
func NewTGShopService(userRepo UserRepository, redeemService *RedeemService, affiliateService *AffiliateService, paymentService *PaymentService) *TGShopService {
	return &TGShopService{userRepo: userRepo, redeemService: redeemService, affiliateService: affiliateService, paymentService: paymentService}
}

// TGShopRechargeInput 是一次 telegram-shop 充值的入参。
type TGShopRechargeInput struct {
	OrderNo string
	TradeNo string
	Email   string
	// Amount 为落账总额（充值额度 + 活动赠额）。
	Amount float64
	// BaseAmount 为实付充值额度（不含赠额），作为邀请返利的计提基数。
	BaseAmount float64
}

// Recharge 为指定 email 用户充值 Amount 元余额，幂等。
func (s *TGShopService) Recharge(ctx context.Context, in TGShopRechargeInput) error {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" {
		return errors.New("email is required")
	}
	if in.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	if strings.TrimSpace(in.OrderNo) == "" {
		return errors.New("order_no is required")
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("user not found for email %s: %w", email, err)
	}

	// 幂等键：以订单号派生兑换码内容
	code := "tgshop-" + strings.TrimSpace(in.OrderNo)

	// 已存在且已使用 → 此前已充值，直接幂等返回
	existing, lookupErr := s.redeemService.GetByCode(ctx, code)
	if lookupErr == nil && existing != nil && existing.IsUsed() {
		slog.Info("[TGShop] order already fulfilled, skip", "orderNo", in.OrderNo, "email", email)
		return nil
	}

	// 兑换码不存在 → 创建（并发下若被他人抢先创建，下方兑换仍可继续）
	if existing == nil || lookupErr != nil {
		rc := &RedeemCode{
			Code:   code,
			Type:   RedeemTypeBalance,
			Value:  in.Amount,
			Status: StatusUnused,
			Notes:  fmt.Sprintf("telegram-shop recharge, trade_no=%s", in.TradeNo),
		}
		if err := s.redeemService.CreateCode(ctx, rc); err != nil {
			// 并发创建会触发唯一约束冲突；此时另一路已创建，继续尝试兑换即可。
			slog.Warn("[TGShop] create redeem code returned error, will still attempt redeem",
				"orderNo", in.OrderNo, "error", err)
		}
	}

	// 兑换到用户账户（余额入账）。跳过兑换码自带的返利逻辑：该逻辑以兑换码
	// 面值（= 落账总额，含赠额）为基数，而 telegram-shop 返利需以实付额度
	// 为基数，故在兑换成功后由下方按 BaseAmount 单独计提。
	if _, err := s.redeemService.Redeem(ContextSkipRedeemAffiliate(ctx), user.ID, code); err != nil {
		// 兑换码已被使用：说明已充值过（并发竞态下的幂等收敛），视为成功。
		if errors.Is(err, ErrRedeemCodeUsed) {
			slog.Info("[TGShop] redeem code already used, treat as fulfilled", "orderNo", in.OrderNo)
			return nil
		}
		return fmt.Errorf("redeem balance: %w", err)
	}

	// 入账成功后计提邀请返利。本调用仅在兑换码首次成功兑换后到达——重复回调
	// 会在上方「已使用」分支提前返回，故不会重复返利。基数取实付额度 BaseAmount，
	// 与标准支付路径口径一致（赠送金额不产生返利）。best-effort，失败不影响入账。
	s.tryAccrueAffiliateRebate(ctx, user.ID, in.OrderNo, in.BaseAmount)

	// 写入一笔展示用的已完成订单，使本次充值可在后台「订单管理」中查看。
	// 余额已由上方兑换路径入账，此订单不再变更余额、不进履约/退款流程。
	// best-effort，失败不影响入账。
	s.tryRecordDisplayOrder(ctx, user, in)

	slog.Info("[TGShop] recharge success", "orderNo", in.OrderNo, "email", email, "amount", in.Amount)
	return nil
}

// tryRecordDisplayOrder 为本次 telegram-shop 充值写入一笔展示用的已完成订单。
// 金额口径：amount = pay_amount = 实付额度 BaseAmount（不含赠送）。
// 幂等：out_trade_no = "tgshop-{order_no}" 命中唯一索引时视为已存在。
func (s *TGShopService) tryRecordDisplayOrder(ctx context.Context, user *User, in TGShopRechargeInput) {
	if s.paymentService == nil {
		return
	}
	key := "tgshop-" + strings.TrimSpace(in.OrderNo)
	_, err := s.paymentService.CreateExternalCompletedOrder(ctx, ExternalCompletedOrderInput{
		UserID:       user.ID,
		UserEmail:    user.Email,
		UserName:     user.Username,
		Amount:       in.BaseAmount,
		PayAmount:    in.BaseAmount,
		PaymentType:  "tgshop",
		OutTradeNo:   key,
		TradeNo:      in.TradeNo,
		RechargeCode: key,
		SrcHost:      "telegram-shop",
	})
	if err != nil && !errors.Is(err, ErrExternalOrderExists) {
		slog.Warn("[TGShop] record display order failed (balance already credited, ignoring)",
			"orderNo", in.OrderNo, "error", err)
	}
}

// tryAccrueAffiliateRebate 为 telegram-shop 充值计提邀请返利。
func (s *TGShopService) tryAccrueAffiliateRebate(ctx context.Context, userID int64, orderNo string, baseAmount float64) {
	if s.affiliateService == nil || baseAmount <= 0 {
		return
	}
	if !s.affiliateService.IsEnabled(ctx) {
		return
	}
	rebate, err := s.affiliateService.AccrueInviteRebate(ctx, userID, baseAmount)
	if err != nil {
		slog.Warn("[TGShop] affiliate rebate failed", "orderNo", orderNo, "userID", userID, "baseAmount", baseAmount, "error", err)
		return
	}
	if rebate > 0 {
		slog.Info("[TGShop] affiliate rebate accrued", "orderNo", orderNo, "userID", userID, "rebate", rebate)
	}
}
