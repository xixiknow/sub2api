package service

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const defaultBalanceRechargeMultiplier = 1.0

func normalizeBalanceRechargeMultiplier(multiplier float64) float64 {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return defaultBalanceRechargeMultiplier
	}
	return multiplier
}

func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	return decimal.NewFromFloat(paymentAmount).
		Mul(decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(multiplier))).
		Round(2).
		InexactFloat64()
}

func calculateRechargeBonus(paymentAmount, baseAmount float64, rules []PaymentRechargeBonusRule) *PaymentRechargeBonusSnapshot {
	if paymentAmount <= 0 || baseAmount <= 0 || len(rules) == 0 {
		return nil
	}

	var matched *PaymentRechargeBonusRule
	for i := range rules {
		rule := rules[i]
		if rule.Disabled || rule.MinAmount <= 0 || paymentAmount < rule.MinAmount {
			continue
		}
		if matched == nil || rule.MinAmount > matched.MinAmount {
			matched = &rules[i]
		}
	}
	if matched == nil {
		return nil
	}

	bonus := decimal.NewFromFloat(matched.BonusAmount)
	if matched.BonusPercent > 0 {
		bonus = bonus.Add(decimal.NewFromFloat(baseAmount).
			Mul(decimal.NewFromFloat(matched.BonusPercent)).
			Div(decimal.NewFromInt(100)))
	}
	bonusAmount := bonus.Round(2).InexactFloat64()
	if bonusAmount <= 0 {
		return nil
	}
	creditedAmount := decimal.NewFromFloat(baseAmount).Add(decimal.NewFromFloat(bonusAmount)).Round(2).InexactFloat64()
	return &PaymentRechargeBonusSnapshot{
		RuleID:         matched.ID,
		RuleName:       matched.Name,
		MinAmount:      matched.MinAmount,
		BonusAmount:    bonusAmount,
		BonusPercent:   matched.BonusPercent,
		PaymentAmount:  paymentAmount,
		BaseAmount:     baseAmount,
		CreditedAmount: creditedAmount,
	}
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= paymentAmountToleranceForCurrency(currency) {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}
