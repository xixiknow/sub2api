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
	userRepo      UserRepository
	redeemService *RedeemService
}

// NewTGShopService 创建 TGShopService。
func NewTGShopService(userRepo UserRepository, redeemService *RedeemService) *TGShopService {
	return &TGShopService{userRepo: userRepo, redeemService: redeemService}
}

// TGShopRechargeInput 是一次 telegram-shop 充值的入参。
type TGShopRechargeInput struct {
	OrderNo string
	TradeNo string
	Email   string
	Amount  float64
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

	// 兑换到用户账户（余额入账）。跳过兑换码自带的返利逻辑，避免与
	// 其它充值返利路径重复计提。
	if _, err := s.redeemService.Redeem(ContextSkipRedeemAffiliate(ctx), user.ID, code); err != nil {
		// 兑换码已被使用：说明已充值过（并发竞态下的幂等收敛），视为成功。
		if errors.Is(err, ErrRedeemCodeUsed) {
			slog.Info("[TGShop] redeem code already used, treat as fulfilled", "orderNo", in.OrderNo)
			return nil
		}
		return fmt.Errorf("redeem balance: %w", err)
	}

	slog.Info("[TGShop] recharge success", "orderNo", in.OrderNo, "email", email, "amount", in.Amount)
	return nil
}
