package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAffiliateUserOverviewSQLIncludesMaturedFrozenQuota(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateUserOverviewSQL), " ")

	require.Contains(t, query, "ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0)")
	require.Contains(t, query, "frozen_until <= NOW()")
}

func TestAffiliateRecordQueriesUseLedgerAuditFields(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	content := string(source)

	require.Contains(t, content, "JOIN payment_orders po ON po.id = ual.source_order_id")
	require.Contains(t, content, "ual.amount::double precision")
	require.Contains(t, content, "ual.balance_after::double precision")
	require.NotContains(t, content, "parseAffiliateRebateAmount")
	require.NotContains(t, content, `"current_balance": "u.balance"`)
}

func TestAffiliateLevelDetailsSQLUsesLedgerAsSourceOfTruth(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateLevelDetailsSQL), " ")

	require.Contains(t, query, "FROM user_affiliates ua")
	require.Contains(t, query, "WHERE ua.inviter_id = $1")
	require.Contains(t, query, "FROM user_affiliate_ledger ual")
	require.Contains(t, query, "ual.action IN ('accrue', 'refund_clawback')")
	require.Contains(t, query, "COALESCE(ual.level, 1) BETWEEN 2 AND 3")
	require.Contains(t, query, "GROUP BY COALESCE(ual.level, 1), ual.source_user_id")
	require.Contains(t, query, "UNION ALL")
	require.Contains(t, query, "COUNT(DISTINCT ual.source_order_id)::integer AS order_count")
	require.Contains(t, query, "ROW_NUMBER() OVER")
	require.Contains(t, query, "WHERE ranked.rn <= $2")
	require.Contains(t, query, "LEFT JOIN users parent ON parent.id = source_aff.inviter_id")
	require.Contains(t, query, "ranked.total_rebate - ranked.frozen_rebate")
}
