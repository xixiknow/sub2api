package service

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestCalculateRechargeBonus_UsesHighestMatchedTier(t *testing.T) {
	bonus := calculateRechargeBonus(100, 100, []PaymentRechargeBonusRule{
		{ID: "tier20", MinAmount: 20, BonusAmount: 1},
		{ID: "tier50", MinAmount: 50, BonusAmount: 5},
		{ID: "tier100", MinAmount: 100, BonusAmount: 11},
	})

	if bonus == nil {
		t.Fatal("expected bonus snapshot")
	}
	if bonus.RuleID != "tier100" {
		t.Fatalf("RuleID = %q, want tier100", bonus.RuleID)
	}
	if bonus.BonusAmount != 11 {
		t.Fatalf("BonusAmount = %v, want 11", bonus.BonusAmount)
	}
	if bonus.CreditedAmount != 111 {
		t.Fatalf("CreditedAmount = %v, want 111", bonus.CreditedAmount)
	}
}

func TestCalculateRechargeBonus_SupportsPercentRule(t *testing.T) {
	bonus := calculateRechargeBonus(200, 200, []PaymentRechargeBonusRule{
		{ID: "percent", MinAmount: 100, BonusPercent: 10},
	})

	if bonus == nil {
		t.Fatal("expected bonus snapshot")
	}
	if bonus.BonusAmount != 20 {
		t.Fatalf("BonusAmount = %v, want 20", bonus.BonusAmount)
	}
	if bonus.CreditedAmount != 220 {
		t.Fatalf("CreditedAmount = %v, want 220", bonus.CreditedAmount)
	}
}

func TestAffiliateRebateBaseAmount_ExcludesRechargeBonus(t *testing.T) {
	order := &dbent.PaymentOrder{
		OrderType: payment.OrderTypeBalance,
		Amount:    111,
		ProviderSnapshot: map[string]any{
			"recharge_bonus": map[string]any{
				"base_amount":     100.0,
				"credited_amount": 111.0,
			},
		},
	}

	if got := affiliateRebateBaseAmount(order); got != 100 {
		t.Fatalf("affiliateRebateBaseAmount = %v, want 100", got)
	}
}
