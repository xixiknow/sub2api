package service

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

type DramaVideoCleanupService struct {
	tasks  DramaVideoTaskRepository
	assets *DramaVideoAssetService
	cfg    *config.Config

	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

func NewDramaVideoCleanupService(tasks DramaVideoTaskRepository, assets *DramaVideoAssetService, cfg *config.Config) *DramaVideoCleanupService {
	return &DramaVideoCleanupService{tasks: tasks, assets: assets, cfg: cfg}
}

func ProvideDramaVideoCleanupService(tasks DramaVideoTaskRepository, assets *DramaVideoAssetService, cfg *config.Config) *DramaVideoCleanupService {
	svc := NewDramaVideoCleanupService(tasks, assets, cfg)
	svc.Start()
	return svc
}

func (s *DramaVideoCleanupService) Start() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	go s.loop(ctx)
}

func (s *DramaVideoCleanupService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *DramaVideoCleanupService) loop(ctx context.Context) {
	defer close(s.done)
	_ = s.RunOnce(ctx, time.Now())
	ticker := time.NewTicker(s.interval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := s.RunOnce(ctx, now); err != nil {
				logger.L().Warn("drama_video_cleanup_failed", zap.Error(err))
			}
		}
	}
}

func (s *DramaVideoCleanupService) RunOnce(ctx context.Context, now time.Time) error {
	if s == nil || s.tasks == nil {
		return nil
	}
	limit := s.batchSize()
	tasks, err := s.tasks.ListOutputsDueForCleanup(ctx, now, limit)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if path := strings.TrimSpace(task.OutputPath); path != "" {
			_ = os.Remove(path)
		}
		if err := s.tasks.MarkOutputDeleted(ctx, task.TaskID, now); err != nil {
			logger.L().Warn("drama_video_output_delete_failed", zap.String("task_id", task.TaskID), zap.Error(err))
		}
	}
	if s.assets != nil {
		_, _ = s.assets.CleanupExpired(ctx, now, limit)
	}
	return nil
}

func (s *DramaVideoCleanupService) interval() time.Duration {
	minutes := 30
	if s.cfg != nil && s.cfg.DramaVideo.CleanupIntervalMinutes > 0 {
		minutes = s.cfg.DramaVideo.CleanupIntervalMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func (s *DramaVideoCleanupService) batchSize() int {
	if s.cfg != nil && s.cfg.DramaVideo.CleanupBatchSize > 0 {
		return s.cfg.DramaVideo.CleanupBatchSize
	}
	return 100
}
