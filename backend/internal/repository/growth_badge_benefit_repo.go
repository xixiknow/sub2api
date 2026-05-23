package repository

import (
	"context"
	"database/sql"
)

const eligibleUserBadgesCTE = `
WITH metrics AS (
    SELECT u.id AS user_id,
           COALESCE(api_key_stats.total_api_keys, 0)::bigint AS total_api_keys,
           COALESCE(usage_stats.total_requests, 0)::bigint AS total_requests,
           COALESCE(aff.aff_count, 0)::bigint AS aff_count,
           (community.used_at IS NOT NULL) AS community_joined,
           (tutorial.created_at IS NOT NULL OR COALESCE(aff.aff_count, 0) > 0) AS affiliate_tutorial_done
    FROM users u
    LEFT JOIN LATERAL (
        SELECT COUNT(*)::bigint AS total_api_keys
        FROM api_keys ak
        WHERE ak.user_id = u.id AND ak.deleted_at IS NULL
    ) api_key_stats ON TRUE
    LEFT JOIN LATERAL (
        SELECT COUNT(*)::bigint AS total_requests
        FROM usage_logs ul
        WHERE ul.user_id = u.id
    ) usage_stats ON TRUE
    LEFT JOIN user_affiliates aff ON aff.user_id = u.id
    LEFT JOIN LATERAL (
        SELECT pcu.used_at
        FROM promo_code_usages pcu
        JOIN promo_codes pc ON pc.id = pcu.promo_code_id
        WHERE pcu.user_id = u.id AND pc.purpose = 'community_join'
        ORDER BY pcu.used_at DESC
        LIMIT 1
    ) community ON TRUE
    LEFT JOIN LATERAL (
        SELECT created_at
        FROM user_growth_events uge
        WHERE uge.user_id = u.id AND uge.event_key = 'affiliate_tutorial_done'
        ORDER BY created_at DESC
        LIMIT 1
    ) tutorial ON TRUE
    WHERE u.id = $1 AND u.deleted_at IS NULL
), unlocked AS (
    SELECT user_id, 'api_key_master'::varchar AS badge_id
    FROM metrics WHERE total_api_keys >= 1
    UNION ALL
    SELECT user_id, 'first_request'
    FROM metrics WHERE total_requests >= 1
    UNION ALL
    SELECT user_id, 'task_explorer'
    FROM metrics
    WHERE ((CASE WHEN total_api_keys >= 1 THEN 1 ELSE 0 END) +
           (CASE WHEN total_requests >= 1 THEN 1 ELSE 0 END) +
           (CASE WHEN community_joined THEN 1 ELSE 0 END) +
           (CASE WHEN affiliate_tutorial_done THEN 1 ELSE 0 END)) >= 4
    UNION ALL
    SELECT user_id, 'call_100'
    FROM metrics WHERE total_requests >= 100
    UNION ALL
    SELECT user_id, 'call_1000'
    FROM metrics WHERE total_requests >= 1000
    UNION ALL
    SELECT user_id, 'call_10000'
    FROM metrics WHERE total_requests >= 10000
    UNION ALL
    SELECT user_id, 'invite_1'
    FROM metrics WHERE aff_count >= 1
    UNION ALL
    SELECT user_id, 'invite_3'
    FROM metrics WHERE aff_count >= 3
    UNION ALL
    SELECT user_id, 'invite_10'
    FROM metrics WHERE aff_count >= 10
    UNION ALL
    SELECT user_id, 'invite_30'
    FROM metrics WHERE aff_count >= 30
)`

func resolveBadgeGroupRate(ctx context.Context, q sqlQueryer, userID, groupID int64) (*float64, error) {
	var rate sql.NullFloat64
	err := scanSingleRow(ctx, q, eligibleUserBadgesCTE+`
SELECT MIN(r.rate_multiplier)::double precision
FROM badge_benefit_rules r
JOIN unlocked u ON u.badge_id = r.badge_id
WHERE r.enabled = TRUE
  AND r.benefit_type = 'group_rate'
  AND r.group_id = $2`, []any{userID, groupID}, &rate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !rate.Valid {
		return nil, nil
	}
	v := rate.Float64
	return &v, nil
}

func resolveBadgeAffiliateRebateRate(ctx context.Context, q sqlQueryer, userID int64) (*float64, error) {
	var rate sql.NullFloat64
	err := scanSingleRow(ctx, q, eligibleUserBadgesCTE+`
SELECT MAX(r.affiliate_rebate_rate_percent)::double precision
FROM badge_benefit_rules r
JOIN unlocked u ON u.badge_id = r.badge_id
WHERE r.enabled = TRUE
  AND r.benefit_type = 'affiliate_rebate'`, []any{userID}, &rate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !rate.Valid {
		return nil, nil
	}
	v := rate.Float64
	return &v, nil
}
