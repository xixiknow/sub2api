package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

const (
	GrowthEventAffiliateTutorialDone = "affiliate_tutorial_done"

	GrowthBadgeAPIKeyMaster = "api_key_master"
	GrowthBadgeFirstRequest = "first_request"
	GrowthBadgeTaskExplorer = "task_explorer"
	GrowthBadgeCall100      = "call_100"
	GrowthBadgeCall1000     = "call_1000"
	GrowthBadgeCall10000    = "call_10000"
	GrowthBadgeInvite1      = "invite_1"
	GrowthBadgeInvite3      = "invite_3"
	GrowthBadgeInvite10     = "invite_10"
	GrowthBadgeInvite30     = "invite_30"
	GrowthBenefitTypeGroup  = "group_rate"
	GrowthBenefitTypeRebate = "affiliate_rebate"
)

var ErrBadgeBenefitRuleNotFound = infraerrors.NotFound("BADGE_BENEFIT_RULE_NOT_FOUND", "badge benefit rule not found")

type GrowthStatus struct {
	CommunityJoined         bool              `json:"community_joined"`
	CommunityJoinedAt       *time.Time        `json:"community_joined_at,omitempty"`
	AffiliateTutorialDone   bool              `json:"affiliate_tutorial_done"`
	AffiliateTutorialDoneAt *time.Time        `json:"affiliate_tutorial_done_at,omitempty"`
	Badges                  []UserBadgeStatus `json:"badges"`
}

type starterTaskGateStatus struct {
	HasAPIKey             bool
	HasFirstRequest       bool
	AffiliateTutorialDone bool
}

func (s starterTaskGateStatus) communityWelfareReady() bool {
	return s.HasAPIKey && s.HasFirstRequest && s.AffiliateTutorialDone
}

type GrowthBadgeDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Condition   string `json:"condition"`
	Reward      string `json:"reward"`
	Category    string `json:"category"`
	Tier        string `json:"tier"`
	Points      int    `json:"points"`
	UnlockCount int64  `json:"unlock_count"`
	RuleCount   int64  `json:"rule_count"`
}

type UserBadgeStatus struct {
	BadgeID    string    `json:"badge_id"`
	Name       string    `json:"name"`
	Title      string    `json:"title"`
	Tier       string    `json:"tier"`
	Points     int       `json:"points"`
	UnlockedAt time.Time `json:"unlocked_at"`
}

type BadgeBenefitRule struct {
	ID                         int64     `json:"id"`
	BadgeID                    string    `json:"badge_id"`
	Name                       string    `json:"name"`
	BenefitType                string    `json:"benefit_type"`
	GroupID                    *int64    `json:"group_id,omitempty"`
	GroupName                  string    `json:"group_name,omitempty"`
	RateMultiplier             *float64  `json:"rate_multiplier,omitempty"`
	AffiliateRebateRatePercent *float64  `json:"affiliate_rebate_rate_percent,omitempty"`
	Enabled                    bool      `json:"enabled"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

type BadgeBenefitRuleInput struct {
	BadgeID                    string
	Name                       string
	BenefitType                string
	GroupID                    *int64
	RateMultiplier             *float64
	AffiliateRebateRatePercent *float64
	Enabled                    bool
}

type GrowthUserFilter struct {
	Search   string
	BadgeID  string
	Page     int
	PageSize int
}

type GrowthUserEntry struct {
	UserID                  int64             `json:"user_id"`
	Email                   string            `json:"email"`
	Username                string            `json:"username"`
	CreatedAt               *time.Time        `json:"created_at,omitempty"`
	TotalBadges             int64             `json:"total_badges"`
	BestAffiliateRebate     *float64          `json:"best_affiliate_rebate_rate_percent,omitempty"`
	BestGroupRateMultiplier *float64          `json:"best_group_rate_multiplier,omitempty"`
	Badges                  []UserBadgeStatus `json:"badges"`
}

type GrowthRecomputeResult struct {
	Refreshed int64 `json:"refreshed"`
	Removed   int64 `json:"removed"`
}

type growthQueryExecer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type GrowthService struct {
	db growthQueryExecer
}

func NewGrowthService(client *dbent.Client) *GrowthService {
	return &GrowthService{db: client}
}

func newGrowthServiceWithDB(db growthQueryExecer) *GrowthService {
	return &GrowthService{db: db}
}

var growthBadgeDefinitions = []GrowthBadgeDefinition{
	{
		ID:          GrowthBadgeAPIKeyMaster,
		Name:        "密钥掌控者",
		Title:       "创建第一个 API Key",
		Description: "完成调用前的关键准备动作。",
		Condition:   "API Key 数量达到 1",
		Reward:      "新人任务进度 +1",
		Category:    "starter",
		Tier:        "bronze",
		Points:      15,
	},
	{
		ID:          GrowthBadgeFirstRequest,
		Name:        "首调成功",
		Title:       "完成第一次 API 调用",
		Description: "从注册转为真实使用，进入可持续转化阶段。",
		Condition:   "累计调用达到 1 次",
		Reward:      "新人任务进度 +1",
		Category:    "starter",
		Tier:        "bronze",
		Points:      20,
	},
	{
		ID:          GrowthBadgeTaskExplorer,
		Name:        "新手毕业",
		Title:       "完成 4 个新人任务",
		Description: "完成创建 API Key、首次调用、邀请返利教学、兑换群福利这 4 个入门动作。",
		Condition:   "新人任务达到 4 个",
		Reward:      "成长进度加速",
		Category:    "starter",
		Tier:        "silver",
		Points:      30,
	},
	{
		ID:          GrowthBadgeCall100,
		Name:        "调用先锋",
		Title:       "累计 100 次调用",
		Description: "开始形成稳定使用习惯。",
		Condition:   "累计调用达到 100 次",
		Reward:      "进阶使用徽章",
		Category:    "usage",
		Tier:        "silver",
		Points:      40,
	},
	{
		ID:          GrowthBadgeCall1000,
		Name:        "千次调用",
		Title:       "累计 1,000 次调用",
		Description: "进入高频使用阶段。",
		Condition:   "累计调用达到 1,000 次",
		Reward:      "高频用户徽章",
		Category:    "usage",
		Tier:        "gold",
		Points:      80,
	},
	{
		ID:          GrowthBadgeCall10000,
		Name:        "万次引擎",
		Title:       "累计 10,000 次调用",
		Description: "已经具备核心用户特征。",
		Condition:   "累计调用达到 10,000 次",
		Reward:      "核心用户徽章",
		Category:    "usage",
		Tier:        "platinum",
		Points:      160,
	},
	{
		ID:          GrowthBadgeInvite1,
		Name:        "破冰邀请官",
		Title:       "完成 1 个有效邀请",
		Description: "用邀请链接带来第一位新用户。",
		Condition:   "邀请人数达到 1",
		Reward:      "邀请成长进度 +1",
		Category:    "affiliate",
		Tier:        "bronze",
		Points:      30,
	},
	{
		ID:          GrowthBadgeInvite3,
		Name:        "增长伙伴",
		Title:       "完成 3 个有效邀请",
		Description: "达到二级返现解锁门槛，下级邀请带来的充值也能产生返利。",
		Condition:   "邀请人数达到 3",
		Reward:      "二级返现解锁资格",
		Category:    "affiliate",
		Tier:        "silver",
		Points:      60,
	},
	{
		ID:          GrowthBadgeInvite10,
		Name:        "渠道先锋",
		Title:       "完成 10 个有效邀请",
		Description: "进入渠道型增长阶段，解锁三级返现资格。",
		Condition:   "邀请人数达到 10",
		Reward:      "三级返现解锁资格",
		Category:    "affiliate",
		Tier:        "gold",
		Points:      120,
	},
	{
		ID:          GrowthBadgeInvite30,
		Name:        "分销高手",
		Title:       "完成 30 个有效邀请",
		Description: "具备长期返利合伙人价值。",
		Condition:   "邀请人数达到 30",
		Reward:      "三级返现升级资格",
		Category:    "affiliate",
		Tier:        "platinum",
		Points:      240,
	},
}

var growthBadgeDefinitionByID = func() map[string]GrowthBadgeDefinition {
	out := make(map[string]GrowthBadgeDefinition, len(growthBadgeDefinitions))
	for _, def := range growthBadgeDefinitions {
		out[def.ID] = def
	}
	return out
}()

func (s *GrowthService) GetStatus(ctx context.Context, userID int64) (*GrowthStatus, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}

	communityJoinedAt, err := s.communityJoinedAt(ctx, userID)
	if err != nil {
		return nil, err
	}

	affiliateTutorialAt, err := s.growthEventAt(ctx, userID, GrowthEventAffiliateTutorialDone)
	if err != nil {
		return nil, err
	}

	affiliateCount, err := s.affiliateCount(ctx, userID)
	if err != nil {
		return nil, err
	}

	if _, err := s.RefreshUserBadges(ctx, userID); err != nil {
		return nil, err
	}
	badges, err := s.GetUserBadges(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &GrowthStatus{
		CommunityJoined:         communityJoinedAt != nil,
		CommunityJoinedAt:       communityJoinedAt,
		AffiliateTutorialDone:   affiliateTutorialAt != nil || affiliateCount > 0,
		AffiliateTutorialDoneAt: affiliateTutorialAt,
		Badges:                  badges,
	}, nil
}

func (s *GrowthService) MarkAffiliateTutorialDone(ctx context.Context, userID int64) (*GrowthStatus, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO user_growth_events (user_id, event_key, created_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (user_id, event_key) DO NOTHING`, userID, GrowthEventAffiliateTutorialDone); err != nil {
		return nil, err
	}
	return s.GetStatus(ctx, userID)
}

func (s *GrowthService) starterTaskGateStatus(ctx context.Context, userID int64) (starterTaskGateStatus, error) {
	var status starterTaskGateStatus
	if userID <= 0 {
		return status, ErrUserNotFound
	}

	err := scanGrowthSingleRow(ctx, s.db, `
SELECT
    EXISTS (
        SELECT 1
        FROM api_keys ak
        WHERE ak.user_id = u.id AND ak.deleted_at IS NULL
    ) AS has_api_key,
    EXISTS (
        SELECT 1
        FROM usage_logs ul
        WHERE ul.user_id = u.id
    ) AS has_first_request,
    (
        EXISTS (
            SELECT 1
            FROM user_growth_events uge
            WHERE uge.user_id = u.id AND uge.event_key = $2
        )
        OR EXISTS (
            SELECT 1
            FROM user_affiliates aff
            WHERE aff.user_id = u.id AND COALESCE(aff.aff_count, 0) > 0
        )
    ) AS affiliate_tutorial_done
FROM users u
WHERE u.id = $1 AND u.deleted_at IS NULL
LIMIT 1`, []any{userID, GrowthEventAffiliateTutorialDone},
		&status.HasAPIKey,
		&status.HasFirstRequest,
		&status.AffiliateTutorialDone,
	)
	if err == sql.ErrNoRows {
		return status, ErrUserNotFound
	}
	return status, err
}

func (s *GrowthService) ListBadgeDefinitions(ctx context.Context) ([]GrowthBadgeDefinition, error) {
	defs := make([]GrowthBadgeDefinition, len(growthBadgeDefinitions))
	copy(defs, growthBadgeDefinitions)

	unlockCounts, err := s.countByBadge(ctx, `SELECT badge_id, COUNT(*)::bigint FROM user_badges GROUP BY badge_id`)
	if err != nil {
		return nil, err
	}
	ruleCounts, err := s.countByBadge(ctx, `SELECT badge_id, COUNT(*)::bigint FROM badge_benefit_rules GROUP BY badge_id`)
	if err != nil {
		return nil, err
	}
	for i := range defs {
		defs[i].UnlockCount = unlockCounts[defs[i].ID]
		defs[i].RuleCount = ruleCounts[defs[i].ID]
	}
	return defs, nil
}

func (s *GrowthService) GetUserBadges(ctx context.Context, userID int64) ([]UserBadgeStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT badge_id, unlocked_at
FROM user_badges
WHERE user_id = $1
ORDER BY unlocked_at ASC, badge_id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	badges := make([]UserBadgeStatus, 0)
	for rows.Next() {
		var badgeID string
		var unlockedAt time.Time
		if err := rows.Scan(&badgeID, &unlockedAt); err != nil {
			return nil, err
		}
		badges = append(badges, badgeStatusFromDefinition(badgeID, unlockedAt))
	}
	return badges, rows.Err()
}

func (s *GrowthService) RefreshUserBadges(ctx context.Context, userID int64) (*GrowthRecomputeResult, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	return s.runBadgeRefreshQuery(ctx, refreshUserBadgesSQL, userID)
}

func (s *GrowthService) RecomputeAllBadges(ctx context.Context) (*GrowthRecomputeResult, error) {
	return s.runBadgeRefreshQuery(ctx, refreshAllBadgesSQL)
}

func (s *GrowthService) ListBenefitRules(ctx context.Context, badgeID, benefitType string) ([]BadgeBenefitRule, error) {
	where := []string{"1=1"}
	args := make([]any, 0, 2)
	if badgeID = strings.TrimSpace(badgeID); badgeID != "" {
		args = append(args, badgeID)
		where = append(where, fmt.Sprintf("r.badge_id = $%d", len(args)))
	}
	if benefitType = strings.TrimSpace(benefitType); benefitType != "" {
		args = append(args, benefitType)
		where = append(where, fmt.Sprintf("r.benefit_type = $%d", len(args)))
	}

	query := fmt.Sprintf(`
SELECT r.id, r.badge_id, r.name, r.benefit_type, r.group_id, COALESCE(g.name, ''),
       r.rate_multiplier, r.affiliate_rebate_rate_percent, r.enabled, r.created_at, r.updated_at
FROM badge_benefit_rules r
LEFT JOIN groups g ON g.id = r.group_id
WHERE %s
ORDER BY r.enabled DESC, r.badge_id ASC, r.benefit_type ASC, r.id DESC`, strings.Join(where, " AND "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]BadgeBenefitRule, 0)
	for rows.Next() {
		rule, err := scanBadgeBenefitRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *GrowthService) CreateBenefitRule(ctx context.Context, input BadgeBenefitRuleInput) (*BadgeBenefitRule, error) {
	normalized, err := normalizeBadgeBenefitRuleInput(input)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
INSERT INTO badge_benefit_rules (
    badge_id, name, benefit_type, group_id, rate_multiplier,
    affiliate_rebate_rate_percent, enabled, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
RETURNING id, badge_id, name, benefit_type, group_id,
          COALESCE((SELECT name FROM groups WHERE id = badge_benefit_rules.group_id), ''),
          rate_multiplier, affiliate_rebate_rate_percent, enabled, created_at, updated_at`,
		normalized.BadgeID,
		normalized.Name,
		normalized.BenefitType,
		nullableInt64(normalized.GroupID),
		nullableFloat64(normalized.RateMultiplier),
		nullableFloat64(normalized.AffiliateRebateRatePercent),
		normalized.Enabled,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSingleBadgeBenefitRuleRows(rows)
}

func (s *GrowthService) UpdateBenefitRule(ctx context.Context, id int64, input BadgeBenefitRuleInput) (*BadgeBenefitRule, error) {
	if id <= 0 {
		return nil, ErrBadgeBenefitRuleNotFound
	}
	normalized, err := normalizeBadgeBenefitRuleInput(input)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
UPDATE badge_benefit_rules
SET badge_id = $2,
    name = $3,
    benefit_type = $4,
    group_id = $5,
    rate_multiplier = $6,
    affiliate_rebate_rate_percent = $7,
    enabled = $8,
    updated_at = NOW()
WHERE id = $1
RETURNING id, badge_id, name, benefit_type, group_id,
          COALESCE((SELECT name FROM groups WHERE id = badge_benefit_rules.group_id), ''),
          rate_multiplier, affiliate_rebate_rate_percent, enabled, created_at, updated_at`,
		id,
		normalized.BadgeID,
		normalized.Name,
		normalized.BenefitType,
		nullableInt64(normalized.GroupID),
		nullableFloat64(normalized.RateMultiplier),
		nullableFloat64(normalized.AffiliateRebateRatePercent),
		normalized.Enabled,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSingleBadgeBenefitRuleRows(rows)
}

func (s *GrowthService) DeleteBenefitRule(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrBadgeBenefitRuleNotFound
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM badge_benefit_rules WHERE id = $1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return ErrBadgeBenefitRuleNotFound
	}
	return nil
}

func (s *GrowthService) ListGrowthUsers(ctx context.Context, filter GrowthUserFilter) ([]GrowthUserEntry, int64, error) {
	page, pageSize := normalizeGrowthPagination(filter.Page, filter.PageSize)
	where, args := buildGrowthUsersWhere(filter.Search, filter.BadgeID)

	countQuery := fmt.Sprintf(`SELECT COUNT(*)::bigint FROM users u WHERE %s`, where)
	var total int64
	if err := scanGrowthSingleRow(ctx, s.db, countQuery, args, &total); err != nil {
		return nil, 0, err
	}

	itemArgs := append([]any{}, args...)
	itemArgs = append(itemArgs, pageSize, (page-1)*pageSize)
	query := fmt.Sprintf(`
SELECT u.id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       u.created_at,
       COUNT(DISTINCT ub.badge_id)::bigint AS total_badges,
       MAX(CASE WHEN r.benefit_type = 'affiliate_rebate' AND r.enabled THEN r.affiliate_rebate_rate_percent END)::double precision,
       MIN(CASE WHEN r.benefit_type = 'group_rate' AND r.enabled THEN r.rate_multiplier END)::double precision
FROM users u
LEFT JOIN user_badges ub ON ub.user_id = u.id
LEFT JOIN badge_benefit_rules r ON r.badge_id = ub.badge_id
WHERE %s
GROUP BY u.id, u.email, u.username, u.created_at
ORDER BY total_badges DESC, u.id DESC
LIMIT $%d OFFSET $%d`, where, len(itemArgs)-1, len(itemArgs))

	rows, err := s.db.QueryContext(ctx, query, itemArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]GrowthUserEntry, 0, pageSize)
	userIDs := make([]int64, 0, pageSize)
	for rows.Next() {
		var item GrowthUserEntry
		var createdAt sql.NullTime
		var rebate sql.NullFloat64
		var groupRate sql.NullFloat64
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &createdAt, &item.TotalBadges, &rebate, &groupRate); err != nil {
			return nil, 0, err
		}
		if createdAt.Valid {
			item.CreatedAt = &createdAt.Time
		}
		if rebate.Valid {
			v := rebate.Float64
			item.BestAffiliateRebate = &v
		}
		if groupRate.Valid {
			v := groupRate.Float64
			item.BestGroupRateMultiplier = &v
		}
		items = append(items, item)
		userIDs = append(userIDs, item.UserID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	badgesByUser, err := s.badgesForUsers(ctx, userIDs)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		if userBadges := badgesByUser[items[i].UserID]; userBadges != nil {
			items[i].Badges = userBadges
		} else {
			items[i].Badges = []UserBadgeStatus{}
		}
	}
	return items, total, nil
}

func (s *GrowthService) communityJoinedAt(ctx context.Context, userID int64) (*time.Time, error) {
	return s.queryOptionalTime(ctx, `
SELECT pcu.used_at
FROM promo_code_usages pcu
JOIN promo_codes pc ON pc.id = pcu.promo_code_id
WHERE pcu.user_id = $1
  AND pc.purpose = $2
ORDER BY pcu.used_at DESC
LIMIT 1`, userID, PromoCodePurposeCommunityJoin)
}

func (s *GrowthService) growthEventAt(ctx context.Context, userID int64, eventKey string) (*time.Time, error) {
	return s.queryOptionalTime(ctx, `
SELECT created_at
FROM user_growth_events
WHERE user_id = $1
  AND event_key = $2
ORDER BY created_at DESC
LIMIT 1`, userID, eventKey)
}

func (s *GrowthService) affiliateCount(ctx context.Context, userID int64) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT COALESCE(aff_count, 0)
FROM user_affiliates
WHERE user_id = $1
LIMIT 1`, userID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	if !rows.Next() {
		return 0, rows.Err()
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, rows.Err()
}

func (s *GrowthService) queryOptionalTime(ctx context.Context, query string, args ...any) (*time.Time, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, rows.Err()
	}
	var value time.Time
	if err := rows.Scan(&value); err != nil {
		return nil, err
	}
	return &value, rows.Err()
}

func (s *GrowthService) countByBadge(ctx context.Context, query string) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var badgeID string
		var count int64
		if err := rows.Scan(&badgeID, &count); err != nil {
			return nil, err
		}
		counts[badgeID] = count
	}
	return counts, rows.Err()
}

func (s *GrowthService) runBadgeRefreshQuery(ctx context.Context, query string, args ...any) (*GrowthRecomputeResult, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result GrowthRecomputeResult
	if !rows.Next() {
		return &result, rows.Err()
	}
	if err := rows.Scan(&result.Refreshed, &result.Removed); err != nil {
		return nil, err
	}
	return &result, rows.Err()
}

func (s *GrowthService) badgesForUsers(ctx context.Context, userIDs []int64) (map[int64][]UserBadgeStatus, error) {
	out := make(map[int64][]UserBadgeStatus, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT user_id, badge_id, unlocked_at
FROM user_badges
WHERE user_id = ANY($1)
ORDER BY user_id ASC, unlocked_at ASC, badge_id ASC`, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var userID int64
		var badgeID string
		var unlockedAt time.Time
		if err := rows.Scan(&userID, &badgeID, &unlockedAt); err != nil {
			return nil, err
		}
		out[userID] = append(out[userID], badgeStatusFromDefinition(badgeID, unlockedAt))
	}
	return out, rows.Err()
}

func badgeStatusFromDefinition(badgeID string, unlockedAt time.Time) UserBadgeStatus {
	def, ok := growthBadgeDefinitionByID[badgeID]
	if !ok {
		return UserBadgeStatus{BadgeID: badgeID, Name: badgeID, Title: badgeID, UnlockedAt: unlockedAt}
	}
	return UserBadgeStatus{
		BadgeID:    badgeID,
		Name:       def.Name,
		Title:      def.Title,
		Tier:       def.Tier,
		Points:     def.Points,
		UnlockedAt: unlockedAt,
	}
}

func normalizeBadgeBenefitRuleInput(input BadgeBenefitRuleInput) (BadgeBenefitRuleInput, error) {
	input.BadgeID = strings.TrimSpace(input.BadgeID)
	input.Name = strings.TrimSpace(input.Name)
	input.BenefitType = strings.TrimSpace(input.BenefitType)
	if _, ok := growthBadgeDefinitionByID[input.BadgeID]; !ok {
		return input, invalidBadgeBenefitRule("unknown badge_id")
	}
	if input.Name == "" {
		input.Name = growthBadgeDefinitionByID[input.BadgeID].Name + "权益"
	}

	switch input.BenefitType {
	case GrowthBenefitTypeGroup:
		if input.GroupID == nil || *input.GroupID <= 0 {
			return input, invalidBadgeBenefitRule("group_id is required for group rate benefits")
		}
		if input.RateMultiplier == nil || !validPositiveFinite(*input.RateMultiplier) {
			return input, invalidBadgeBenefitRule("rate_multiplier must be greater than 0")
		}
		input.AffiliateRebateRatePercent = nil
	case GrowthBenefitTypeRebate:
		if input.AffiliateRebateRatePercent == nil || !validPercent(*input.AffiliateRebateRatePercent) {
			return input, invalidBadgeBenefitRule("affiliate_rebate_rate_percent must be between 0 and 100")
		}
		input.GroupID = nil
		input.RateMultiplier = nil
	default:
		return input, invalidBadgeBenefitRule("benefit_type must be group_rate or affiliate_rebate")
	}
	return input, nil
}

func invalidBadgeBenefitRule(message string) error {
	return infraerrors.BadRequest("INVALID_BADGE_BENEFIT_RULE", message)
}

func validPositiveFinite(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func validPercent(v float64) bool {
	return v >= 0 && v <= 100 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableFloat64(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func scanBadgeBenefitRule(rows interface {
	Scan(dest ...any) error
}) (BadgeBenefitRule, error) {
	var rule BadgeBenefitRule
	var groupID sql.NullInt64
	var rate sql.NullFloat64
	var rebate sql.NullFloat64
	if err := rows.Scan(
		&rule.ID,
		&rule.BadgeID,
		&rule.Name,
		&rule.BenefitType,
		&groupID,
		&rule.GroupName,
		&rate,
		&rebate,
		&rule.Enabled,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	); err != nil {
		return rule, err
	}
	if groupID.Valid {
		v := groupID.Int64
		rule.GroupID = &v
	}
	if rate.Valid {
		v := rate.Float64
		rule.RateMultiplier = &v
	}
	if rebate.Valid {
		v := rebate.Float64
		rule.AffiliateRebateRatePercent = &v
	}
	return rule, nil
}

func scanSingleBadgeBenefitRuleRows(rows *sql.Rows) (*BadgeBenefitRule, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrBadgeBenefitRuleNotFound
	}
	rule, err := scanBadgeBenefitRule(rows)
	if err != nil {
		return nil, err
	}
	return &rule, rows.Err()
}

func normalizeGrowthPagination(page, pageSize int) (int, int) {
	if page < 1 {
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

func buildGrowthUsersWhere(search, badgeID string) (string, []any) {
	where := []string{"u.deleted_at IS NULL"}
	args := make([]any, 0, 2)
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+search+"%")
		where = append(where, fmt.Sprintf("(u.email ILIKE $%d OR u.username ILIKE $%d)", len(args), len(args)))
	}
	if badgeID = strings.TrimSpace(badgeID); badgeID != "" {
		args = append(args, badgeID)
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM user_badges fub WHERE fub.user_id = u.id AND fub.badge_id = $%d)", len(args)))
	}
	return strings.Join(where, " AND "), args
}

func scanGrowthSingleRow(ctx context.Context, q growthQueryExecer, query string, args []any, dest ...any) error {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	return rows.Err()
}

const refreshUserBadgesSQL = `
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
    SELECT user_id, 'api_key_master'::varchar AS badge_id, jsonb_build_object('current', total_api_keys, 'target', 1) AS metadata
    FROM metrics WHERE total_api_keys >= 1
    UNION ALL
    SELECT user_id, 'first_request', jsonb_build_object('current', total_requests, 'target', 1)
    FROM metrics WHERE total_requests >= 1
    UNION ALL
    SELECT user_id, 'task_explorer', jsonb_build_object('current',
        ((CASE WHEN total_api_keys >= 1 THEN 1 ELSE 0 END) +
         (CASE WHEN total_requests >= 1 THEN 1 ELSE 0 END) +
         (CASE WHEN community_joined THEN 1 ELSE 0 END) +
         (CASE WHEN affiliate_tutorial_done THEN 1 ELSE 0 END)), 'target', 4)
    FROM metrics
    WHERE ((CASE WHEN total_api_keys >= 1 THEN 1 ELSE 0 END) +
           (CASE WHEN total_requests >= 1 THEN 1 ELSE 0 END) +
           (CASE WHEN community_joined THEN 1 ELSE 0 END) +
           (CASE WHEN affiliate_tutorial_done THEN 1 ELSE 0 END)) >= 4
    UNION ALL
    SELECT user_id, 'call_100', jsonb_build_object('current', total_requests, 'target', 100)
    FROM metrics WHERE total_requests >= 100
    UNION ALL
    SELECT user_id, 'call_1000', jsonb_build_object('current', total_requests, 'target', 1000)
    FROM metrics WHERE total_requests >= 1000
    UNION ALL
    SELECT user_id, 'call_10000', jsonb_build_object('current', total_requests, 'target', 10000)
    FROM metrics WHERE total_requests >= 10000
    UNION ALL
    SELECT user_id, 'invite_1', jsonb_build_object('current', aff_count, 'target', 1)
    FROM metrics WHERE aff_count >= 1
    UNION ALL
    SELECT user_id, 'invite_3', jsonb_build_object('current', aff_count, 'target', 3)
    FROM metrics WHERE aff_count >= 3
    UNION ALL
    SELECT user_id, 'invite_10', jsonb_build_object('current', aff_count, 'target', 10)
    FROM metrics WHERE aff_count >= 10
    UNION ALL
    SELECT user_id, 'invite_30', jsonb_build_object('current', aff_count, 'target', 30)
    FROM metrics WHERE aff_count >= 30
), upserted AS (
    INSERT INTO user_badges (user_id, badge_id, unlocked_at, metadata, created_at, updated_at)
    SELECT user_id, badge_id, NOW(), metadata, NOW(), NOW()
    FROM unlocked
    ON CONFLICT (user_id, badge_id) DO UPDATE
        SET metadata = EXCLUDED.metadata,
            updated_at = EXCLUDED.updated_at
    RETURNING 1
), deleted AS (
    DELETE FROM user_badges ub
    WHERE ub.user_id = $1
      AND ub.badge_id IN ('api_key_master', 'first_request', 'task_explorer', 'call_100', 'call_1000', 'call_10000', 'invite_1', 'invite_3', 'invite_10', 'invite_30')
      AND NOT EXISTS (
          SELECT 1 FROM unlocked u WHERE u.user_id = ub.user_id AND u.badge_id = ub.badge_id
      )
    RETURNING 1
)
SELECT (SELECT COUNT(*)::bigint FROM upserted), (SELECT COUNT(*)::bigint FROM deleted)`

const refreshAllBadgesSQL = `
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
    WHERE u.deleted_at IS NULL
), unlocked AS (
    SELECT user_id, 'api_key_master'::varchar AS badge_id, jsonb_build_object('current', total_api_keys, 'target', 1) AS metadata
    FROM metrics WHERE total_api_keys >= 1
    UNION ALL
    SELECT user_id, 'first_request', jsonb_build_object('current', total_requests, 'target', 1)
    FROM metrics WHERE total_requests >= 1
    UNION ALL
    SELECT user_id, 'task_explorer', jsonb_build_object('current',
        ((CASE WHEN total_api_keys >= 1 THEN 1 ELSE 0 END) +
         (CASE WHEN total_requests >= 1 THEN 1 ELSE 0 END) +
         (CASE WHEN community_joined THEN 1 ELSE 0 END) +
         (CASE WHEN affiliate_tutorial_done THEN 1 ELSE 0 END)), 'target', 4)
    FROM metrics
    WHERE ((CASE WHEN total_api_keys >= 1 THEN 1 ELSE 0 END) +
           (CASE WHEN total_requests >= 1 THEN 1 ELSE 0 END) +
           (CASE WHEN community_joined THEN 1 ELSE 0 END) +
           (CASE WHEN affiliate_tutorial_done THEN 1 ELSE 0 END)) >= 4
    UNION ALL
    SELECT user_id, 'call_100', jsonb_build_object('current', total_requests, 'target', 100)
    FROM metrics WHERE total_requests >= 100
    UNION ALL
    SELECT user_id, 'call_1000', jsonb_build_object('current', total_requests, 'target', 1000)
    FROM metrics WHERE total_requests >= 1000
    UNION ALL
    SELECT user_id, 'call_10000', jsonb_build_object('current', total_requests, 'target', 10000)
    FROM metrics WHERE total_requests >= 10000
    UNION ALL
    SELECT user_id, 'invite_1', jsonb_build_object('current', aff_count, 'target', 1)
    FROM metrics WHERE aff_count >= 1
    UNION ALL
    SELECT user_id, 'invite_3', jsonb_build_object('current', aff_count, 'target', 3)
    FROM metrics WHERE aff_count >= 3
    UNION ALL
    SELECT user_id, 'invite_10', jsonb_build_object('current', aff_count, 'target', 10)
    FROM metrics WHERE aff_count >= 10
    UNION ALL
    SELECT user_id, 'invite_30', jsonb_build_object('current', aff_count, 'target', 30)
    FROM metrics WHERE aff_count >= 30
), upserted AS (
    INSERT INTO user_badges (user_id, badge_id, unlocked_at, metadata, created_at, updated_at)
    SELECT user_id, badge_id, NOW(), metadata, NOW(), NOW()
    FROM unlocked
    ON CONFLICT (user_id, badge_id) DO UPDATE
        SET metadata = EXCLUDED.metadata,
            updated_at = EXCLUDED.updated_at
    RETURNING 1
), deleted AS (
    DELETE FROM user_badges ub
    WHERE ub.badge_id IN ('api_key_master', 'first_request', 'task_explorer', 'call_100', 'call_1000', 'call_10000', 'invite_1', 'invite_3', 'invite_10', 'invite_30')
      AND NOT EXISTS (
          SELECT 1 FROM unlocked u WHERE u.user_id = ub.user_id AND u.badge_id = ub.badge_id
      )
    RETURNING 1
)
SELECT (SELECT COUNT(*)::bigint FROM upserted), (SELECT COUNT(*)::bigint FROM deleted)`
