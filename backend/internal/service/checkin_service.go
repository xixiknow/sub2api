package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

// Beijing has no DST. Calendar dates are independent of the server timezone.
var checkinZone = time.FixedZone("Asia/Shanghai", 8*60*60)

type CheckinPrize struct {
	Cents       int     `json:"cents"`
	Probability float64 `json:"probability"` // percentage, all prizes sum to 100
}

type CheckinSnapshot struct {
	Version       int            `json:"version"`
	ActiveDays    int            `json:"active_days"`
	StreakDays    int            `json:"streak_days"` // includes today's claim; capped at seven
	Spend         float64        `json:"spend"`
	HasRecharge   bool           `json:"has_recharge"`
	ActivityScore float64        `json:"activity_score"`
	StreakScore   float64        `json:"streak_score"`
	SpendScore    float64        `json:"spend_score"`
	RechargeScore float64        `json:"recharge_score"`
	Score         float64        `json:"score"`
	Prizes        []CheckinPrize `json:"prizes"`
}

type CheckinRecord struct {
	Date        string    `json:"date"`
	RewardCents int       `json:"reward_cents"`
	CreatedAt   time.Time `json:"created_at"`
}

type CheckinStatus struct {
	Date        string          `json:"date"`
	NextResetAt time.Time       `json:"next_reset_at"`
	CheckedIn   bool            `json:"checked_in"`
	RewardCents int             `json:"reward_cents"`
	Snapshot    CheckinSnapshot `json:"snapshot"`
	History     []CheckinRecord `json:"history"` // last 30 claims
}

type checkinDB interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type checkinSwitch interface {
	IsCheckinEnabled(ctx context.Context) bool
}

type CheckinService struct {
	db       *sql.DB
	cache    BillingCache
	auth     APIKeyAuthCacheInvalidator
	cfg      *config.Config
	settings checkinSwitch
	now      func() time.Time
	draw     func(int64) (int64, error)
}

func NewCheckinService(db *sql.DB, cache BillingCache, auth APIKeyAuthCacheInvalidator, cfg *config.Config, settings *SettingService) *CheckinService {
	s := &CheckinService{db: db, cache: cache, auth: auth, cfg: cfg, now: time.Now, draw: func(n int64) (int64, error) {
		v, err := rand.Int(rand.Reader, big.NewInt(n))
		if err != nil {
			return 0, err
		}
		return v.Int64(), nil
	}}
	// Avoid storing a typed nil pointer in the interface.
	if settings != nil {
		s.settings = settings
	}
	return s
}

func checkinDay(now time.Time) time.Time {
	t := now.In(checkinZone)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, checkinZone)
}

func checkinSpendScore(amount float64) float64 {
	thresholds := []float64{0, 1, 5, 10, 20}
	points := []float64{0, 3, 8, 12, 15}
	if amount <= 0 {
		return 0
	}
	for i := 1; i < len(thresholds); i++ {
		if amount < thresholds[i] {
			return points[i-1] + (amount-thresholds[i-1])/(thresholds[i]-thresholds[i-1])*(points[i]-points[i-1])
		}
	}
	return 15
}

func calculateCheckinSnapshot(active, streak int, spend float64, recharge bool) CheckinSnapshot {
	active = max(0, min(7, active))
	streak = max(1, min(7, streak))
	s := CheckinSnapshot{Version: 1, ActiveDays: active, StreakDays: streak, Spend: math.Max(0, spend), HasRecharge: recharge}
	s.ActivityScore = 40 * float64(active) / 7
	s.StreakScore = 40 * float64(streak) / 7
	s.SpendScore = checkinSpendScore(spend)
	if recharge {
		s.RechargeScore = 5
	}
	s.Score = s.ActivityScore + s.StreakScore + s.SpendScore + s.RechargeScore
	s.Prizes = checkinPrizes(s.Score)
	return s
}

func checkinPrizes(score float64) []CheckinPrize {
	score = math.Max(0, math.Min(100, score))
	prizes := make([]CheckinPrize, 0, 5)
	// Integer millionths preserve an exact total and avoid boundary ambiguity.
	base := []int64{650000, 250000, 80000, 15000, 5000}
	full := []int64{250000, 300000, 300000, 120000, 30000}
	cents := []int{1, 3, 10, 50, 100}
	remaining := int64(1000000)
	for i := range cents {
		weight := int64(math.Round(float64(base[i]) + float64(full[i]-base[i])*score/100))
		if i == len(cents)-1 {
			weight = remaining
		}
		remaining -= weight
		prizes = append(prizes, CheckinPrize{cents[i], float64(weight) / 10000})
	}
	return prizes
}

func checkinReward(prizes []CheckinPrize, ticket int64) int {
	for _, p := range prizes {
		ticket -= int64(math.Round(p.Probability * 10000))
		if ticket < 0 {
			return p.Cents
		}
	}
	return prizes[len(prizes)-1].Cents
}

var ErrCheckinDisabled = infraerrors.Forbidden("CHECKIN_DISABLED", "Check-in is disabled")

func (s *CheckinService) enabled(ctx context.Context) error {
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return ErrCheckinDisabled
	}
	if s.settings == nil || !s.settings.IsCheckinEnabled(ctx) {
		return ErrCheckinDisabled
	}
	return nil
}

func (s *CheckinService) Status(ctx context.Context, userID int64) (*CheckinStatus, error) {
	if err := s.enabled(ctx); err != nil {
		return nil, err
	}
	return readCheckinStatus(ctx, s.db, userID, s.now())
}

func readCheckinStatus(ctx context.Context, db checkinDB, userID int64, now time.Time) (*CheckinStatus, error) {
	day := checkinDay(now)
	status := &CheckinStatus{Date: day.Format(time.DateOnly), NextResetAt: day.AddDate(0, 0, 1), History: []CheckinRecord{}}
	rows, err := db.QueryContext(ctx, `SELECT checkin_date::text, reward_cents, created_at, snapshot FROM daily_checkins WHERE user_id=$1 AND checkin_date<=$2 ORDER BY checkin_date DESC LIMIT 30`, userID, status.Date)
	if err != nil {
		return nil, err
	}
	dates := make(map[string]bool)
	for rows.Next() {
		var r CheckinRecord
		var raw []byte
		if err = rows.Scan(&r.Date, &r.RewardCents, &r.CreatedAt, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		status.History = append(status.History, r)
		dates[r.Date] = true
		if r.Date == status.Date {
			status.CheckedIn = true
			status.RewardCents = r.RewardCents
			if err = json.Unmarshal(raw, &status.Snapshot); err != nil {
				rows.Close()
				return nil, err
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// A committed claim always returns its original odds and factors, including retries.
	if status.CheckedIn {
		return status, nil
	}
	streak := 1
	for i := 1; i < 7; i++ {
		if !dates[day.AddDate(0, 0, -i).Format(time.DateOnly)] {
			break
		}
		streak++
	}
	// Seven Beijing calendar days including today, through the claim instant.
	start := day.AddDate(0, 0, -6)
	var active int
	var spend float64
	var recharge bool
	err = db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT (created_at AT TIME ZONE 'Asia/Shanghai')::date), COALESCE(SUM(CASE WHEN billing_type=0 THEN actual_cost ELSE 0 END),0) FROM usage_logs WHERE user_id=$1 AND created_at >= $2 AND created_at <= $3 AND actual_cost>0`, userID, start, now).Scan(&active, &spend)
	if err != nil {
		return nil, err
	}
	err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND completed_at >= $2 AND completed_at <= $3 AND amount>0 AND status IN ('COMPLETED','PARTIALLY_REFUNDED','REFUND_REQUESTED','REFUNDING','REFUND_PENDING','REFUND_FAILED') AND (status<>'PARTIALLY_REFUNDED' OR refund_amount<amount))`, userID, start, now).Scan(&recharge)
	if err != nil {
		return nil, err
	}
	status.Snapshot = calculateCheckinSnapshot(active, streak, spend, recharge)
	return status, nil
}

func (s *CheckinService) Claim(ctx context.Context, userID int64, captchaHash string) (*CheckinStatus, error) {
	if decoded, err := hex.DecodeString(captchaHash); err != nil || len(decoded) != 32 {
		return nil, ErrCheckinCaptchaRequired
	}
	if err := s.enabled(ctx); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Serializes every claim for this user, also across application replicas.
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL AND status='active' FOR UPDATE`, userID).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	now := s.now()
	status, err := readCheckinStatus(ctx, tx, userID, now)
	if err != nil {
		return nil, err
	}
	if !status.CheckedIn {
		ticket, err := s.draw(1000000)
		if err != nil {
			return nil, err
		}
		cents := checkinReward(status.Snapshot.Prizes, ticket)
		snapshot, err := json.Marshal(status.Snapshot)
		if err != nil {
			return nil, err
		}
		var claimID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO daily_checkins (user_id,checkin_date,reward_cents,snapshot,created_at,captcha_hash) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, userID, status.Date, cents, string(snapshot), now, captchaHash).Scan(&claimID)
		if err != nil {
			if e, ok := err.(*pq.Error); ok && e.Code == "23505" && e.Constraint == "daily_checkins_captcha_hash_unique" {
				return nil, ErrCheckinCaptchaRequired
			}
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$2::numeric/100, updated_at=$3 WHERE id=$1`, userID, cents, now); err != nil {
			return nil, err
		}
		// Used history entry only: no redeemable token, recharge total, or referral reward.
		if _, err = tx.ExecContext(ctx, `INSERT INTO redeem_codes (code,type,value,status,used_by,used_at,created_at,notes) VALUES ($1,'checkin_balance',$2::numeric/100,'used',$3,$4,$4,'Daily check-in reward')`, fmt.Sprintf("CHECKIN-%d", claimID), cents, userID, now); err != nil {
			return nil, err
		}
		status.CheckedIn = true
		status.RewardCents = cents
		status.History = append([]CheckinRecord{{Date: status.Date, RewardCents: cents, CreatedAt: now}}, status.History...)
		if len(status.History) > 30 {
			status.History = status.History[:30]
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	// Retry also invalidates caches, healing a previously interrupted response.
	cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s.auth != nil {
		s.auth.InvalidateAuthCacheByUserID(cacheCtx, userID)
	}
	if s.cache != nil {
		if err = s.cache.InvalidateUserBalance(cacheCtx, userID); err != nil {
			slog.Error("checkin balance cache invalidation", "user_id", userID, "error", err)
		}
	}
	return status, nil
}
