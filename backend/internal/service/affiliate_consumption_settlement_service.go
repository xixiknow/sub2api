package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
)

const (
	affiliateConsumptionSettlementInterval      = time.Hour
	affiliateConsumptionSettlementLag           = 5 * time.Minute
	affiliateConsumptionSettlementTimeout       = 10 * time.Minute
	affiliateConsumptionSettlementLeaderLockKey = "affiliate:consumption_settlement:leader"
	affiliateConsumptionSettlementLeaderLockTTL = 15 * time.Minute
)

// AffiliateConsumptionSettlementRepository 是消费返利结算所需的仓储能力，
// 由 affiliateRepository 实现。LockConsumptionSettlementCursor 必须在事务中调用。
type AffiliateConsumptionSettlementRepository interface {
	LockConsumptionSettlementCursor(ctx context.Context) (int64, error)
	GetConsumptionSettlementUpperBound(ctx context.Context, afterID int64, before time.Time) (int64, error)
	SumInviteeBalanceConsumption(ctx context.Context, afterID, upperID int64, durationDays int) ([]AffiliateInviteeConsumption, error)
	AdvanceConsumptionSettlementCursor(ctx context.Context, upperID int64) error
}

// AffiliateConsumptionSettlementResult 是一次结算的结果摘要。
type AffiliateConsumptionSettlementResult struct {
	FromUsageLogID int64
	ToUsageLogID   int64
	InviteeCount   int
	TotalRebate    float64
}

// AffiliateConsumptionSettlementService 定时把被邀请人的余额消费汇总为邀请返利。
//
// 每次结算在一个数据库事务中完成：锁定游标 → 取本批次 usage_logs.id 上界（留出
// 写入延迟）→ 按被邀请人汇总余额消费 → 沿邀请链路入账 → 推进游标。任何一步失败
// 都整体回滚，下次从同一游标重试；流水按 (结算批次, 邀请人, 被邀请人, 层级) 唯一，
// 不会重复入账。
type AffiliateConsumptionSettlementService struct {
	entClient        *dbent.Client
	repo             AffiliateConsumptionSettlementRepository
	affiliateService *AffiliateService
	settingService   *SettingService
	cfg              *config.Config

	interval time.Duration
	lag      time.Duration
	now      func() time.Time

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewAffiliateConsumptionSettlementService(
	entClient *dbent.Client,
	repo AffiliateConsumptionSettlementRepository,
	affiliateService *AffiliateService,
	settingService *SettingService,
	cfg *config.Config,
) *AffiliateConsumptionSettlementService {
	return &AffiliateConsumptionSettlementService{
		entClient:        entClient,
		repo:             repo,
		affiliateService: affiliateService,
		settingService:   settingService,
		cfg:              cfg,
		interval:         affiliateConsumptionSettlementInterval,
		lag:              affiliateConsumptionSettlementLag,
		now:              time.Now,
		instanceID:       uuid.NewString(),
		stopCh:           make(chan struct{}),
	}
}

// SetLeaderLock 配置多实例互斥；不配置时单实例直接运行（游标行锁仍保证正确性）。
func (s *AffiliateConsumptionSettlementService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *AffiliateConsumptionSettlementService) Start() {
	if s == nil || s.entClient == nil || s.repo == nil || s.affiliateService == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *AffiliateConsumptionSettlementService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *AffiliateConsumptionSettlementService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), affiliateConsumptionSettlementTimeout)
	defer cancel()

	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, affiliateConsumptionSettlementLeaderLockKey, s.instanceID, affiliateConsumptionSettlementLeaderLockTTL)
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	result, err := s.SettleOnce(ctx)
	if err != nil {
		slog.Error("[AffiliateSettlement] consumption rebate settlement failed", "error", err)
		return
	}
	if result != nil && result.InviteeCount > 0 {
		slog.Info("[AffiliateSettlement] consumption rebate settled",
			"from_usage_log_id", result.FromUsageLogID,
			"to_usage_log_id", result.ToUsageLogID,
			"invitees", result.InviteeCount,
			"total_rebate", result.TotalRebate)
	}
}

// SettleOnce 执行一次消费返利结算。返利功能关闭期间游标保持不动，重新开启后
// 关闭期间的消费不会补发：游标会直接跳到当前上界。
func (s *AffiliateConsumptionSettlementService) SettleOnce(ctx context.Context) (*AffiliateConsumptionSettlementResult, error) {
	if s == nil || s.entClient == nil || s.repo == nil || s.affiliateService == nil {
		return nil, nil
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return nil, nil
	}
	affiliateEnabled := s.affiliateService.IsEnabled(ctx)

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin consumption settlement tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	cursor, err := s.repo.LockConsumptionSettlementCursor(txCtx)
	if err != nil {
		return nil, err
	}
	upper, err := s.repo.GetConsumptionSettlementUpperBound(txCtx, cursor, s.now().Add(-s.lag))
	if err != nil {
		return nil, err
	}
	result := &AffiliateConsumptionSettlementResult{FromUsageLogID: cursor, ToUsageLogID: upper}
	if upper <= cursor {
		return result, nil
	}

	if affiliateEnabled {
		durationDays := 0
		if s.settingService != nil {
			durationDays = s.settingService.GetAffiliateRebateDurationDays(ctx)
		}
		consumptions, err := s.repo.SumInviteeBalanceConsumption(txCtx, cursor, upper, durationDays)
		if err != nil {
			return nil, err
		}
		for _, item := range consumptions {
			rebate, err := s.affiliateService.AccrueInviteRebateForSettlement(txCtx, item.UserID, item.Amount, upper)
			if err != nil {
				return nil, fmt.Errorf("accrue consumption rebate for user %d: %w", item.UserID, err)
			}
			if rebate > 0 {
				result.InviteeCount++
				result.TotalRebate = roundTo(result.TotalRebate+rebate, 8)
			}
		}
	}

	if err := s.repo.AdvanceConsumptionSettlementCursor(txCtx, upper); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit consumption settlement tx: %w", err)
	}
	return result, nil
}
