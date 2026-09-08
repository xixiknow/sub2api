package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubDramaTasks struct {
	mu   sync.Mutex
	byID map[string]*DramaVideoTask
}

func (s *stubDramaTasks) Create(_ context.Context, params CreateDramaVideoTaskParams) (*DramaVideoTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID == nil {
		s.byID = map[string]*DramaVideoTask{}
	}
	task := &DramaVideoTask{
		TaskID:          params.TaskID,
		UserID:          params.UserID,
		APIKeyID:        params.APIKeyID,
		GroupID:         params.GroupID,
		Model:           params.Model,
		UpstreamModel:   params.UpstreamModel,
		Status:          params.Status,
		HoldAmount:      params.HoldAmount,
		Resolution:      params.Resolution,
		DurationSeconds: params.DurationSeconds,
	}
	id := params.AccountID
	task.AccountID = &id
	s.byID[params.TaskID] = task
	return task, nil
}

func (s *stubDramaTasks) GetByTaskID(_ context.Context, taskID string) (*DramaVideoTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.byID[taskID]
	if !ok {
		return nil, ErrDramaVideoTaskNotFound
	}
	return task, nil
}

func (s *stubDramaTasks) GetForOwner(ctx context.Context, owner DramaVideoOwner, taskID string) (*DramaVideoTask, error) {
	task, err := s.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.UserID != owner.UserID || task.APIKeyID != owner.APIKeyID {
		return nil, ErrDramaVideoTaskNotFound
	}
	return task, nil
}

func (s *stubDramaTasks) ListByAPIKey(_ context.Context, userID, apiKeyID int64, limit, offset int) ([]*DramaVideoTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	out := make([]*DramaVideoTask, 0)
	for _, task := range s.byID {
		if task.UserID == userID && task.APIKeyID == apiKeyID {
			out = append(out, task)
		}
	}
	if offset >= len(out) {
		return []*DramaVideoTask{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

func (s *stubDramaTasks) UpdateStatus(_ context.Context, update DramaVideoTaskStatusUpdate) (*DramaVideoTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task := s.byID[update.TaskID]
	task.Status = update.Status
	if update.UpstreamTaskID != nil {
		task.UpstreamTaskID = *update.UpstreamTaskID
	}
	return task, nil
}

func (s *stubDramaTasks) MarkCompleted(_ context.Context, update DramaVideoTaskCompletionUpdate) (*DramaVideoTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task := s.byID[update.TaskID]
	task.Status = DramaVideoStatusCompleted
	task.OutputPath = update.OutputPath
	cost := update.ActualCost
	task.ActualCost = &cost
	task.OutputExpiresAt = update.OutputExpiresAt
	return task, nil
}

func (s *stubDramaTasks) ListOutputsDueForCleanup(_ context.Context, now time.Time, _ int) ([]*DramaVideoTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*DramaVideoTask, 0)
	for _, task := range s.byID {
		if task.Status == DramaVideoStatusCompleted && task.OutputDeletedAt == nil && task.OutputExpiresAt != nil && !task.OutputExpiresAt.After(now) {
			out = append(out, task)
		}
	}
	return out, nil
}

func (s *stubDramaTasks) MarkOutputDeleted(_ context.Context, taskID string, deletedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task := s.byID[taskID]
	if task == nil {
		return ErrDramaVideoTaskNotFound
	}
	task.OutputDeletedAt = &deletedAt
	task.OutputPath = ""
	return nil
}

type stubDramaAccounts struct {
	account Account
}

func (s stubDramaAccounts) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	return []Account{s.account}, nil
}

type stubDramaClient struct{}

func (stubDramaClient) CreateVideo(context.Context, *Account, string, []byte) (*DramaVideoUpstreamTask, error) {
	return &DramaVideoUpstreamTask{ID: "up_1", Status: DramaVideoStatusCompleted}, nil
}
func (stubDramaClient) GetVideo(context.Context, *Account, string, string) (*DramaVideoUpstreamTask, error) {
	return &DramaVideoUpstreamTask{ID: "up_1", Status: DramaVideoStatusCompleted}, nil
}
func (stubDramaClient) DownloadVideo(context.Context, *Account, string) (*DramaVideoDownload, error) {
	return &DramaVideoDownload{ContentType: "video/mp4", Data: []byte("\x00\x00\x00\x18ftypisom")}, nil
}

type stubDramaBillingRepo struct {
	mu       sync.Mutex
	holds    []string
	captures []string
	releases []string
}

func (s *stubDramaBillingRepo) ReserveBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.holds = append(s.holds, cmd.RequestID)
	return &BatchImageBalanceHoldResult{}, nil
}
func (s *stubDramaBillingRepo) CaptureBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.captures = append(s.captures, cmd.RequestID)
	return &BatchImageBalanceHoldResult{}, nil
}
func (s *stubDramaBillingRepo) ReleaseBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releases = append(s.releases, cmd.RequestID)
	return &BatchImageBalanceHoldResult{}, nil
}

func TestDramaVideoServiceCreateHoldsThenCaptures(t *testing.T) {
	gid := int64(7)
	tasks := &stubDramaTasks{}
	billingRepo := &stubDramaBillingRepo{}
	svc := &DramaVideoService{
		tasks:        tasks,
		accounts:     stubDramaAccounts{account: Account{ID: 3, Platform: PlatformDrama, Credentials: map[string]any{"api_key": "tok"}}},
		client:       stubDramaClient{},
		billing:      NewBillingService(nil, nil),
		usageBilling: billingRepo,
		outputDir:    t.TempDir(),
		backgroundRunner: func(fn func()) {
			fn()
		},
	}
	apiKey := &APIKey{
		ID:      11,
		UserID:  22,
		GroupID: &gid,
		Group:   &Group{ID: gid, Platform: PlatformDrama, RateMultiplier: 1},
	}
	got, err := svc.Create(context.Background(), apiKey, []byte(`{"model":"seedance2.0-F","prompt":"hi","resolution":"720p","seconds":8}`), "/v1/video/generations")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.True(t, IsDramaVideoTaskID(got.Task.ID))
	require.Equal(t, DramaFamilySeedance20F, got.Task.Model)
	require.Len(t, billingRepo.holds, 1)
	require.Equal(t, DramaVideoHoldRequestID(got.Task.ID), billingRepo.holds[0])
	require.Len(t, billingRepo.captures, 1)
	require.Empty(t, billingRepo.releases)
}

type failingDramaClient struct{}

func (failingDramaClient) CreateVideo(context.Context, *Account, string, []byte) (*DramaVideoUpstreamTask, error) {
	return nil, errors.New(`Drama upstream error: status=400 body={"code":"invalid_request","message":"model_not_supported","data":null}`)
}
func (failingDramaClient) GetVideo(context.Context, *Account, string, string) (*DramaVideoUpstreamTask, error) {
	return nil, errors.New("unused")
}
func (failingDramaClient) DownloadVideo(context.Context, *Account, string) (*DramaVideoDownload, error) {
	return nil, errors.New("unused")
}

func TestDramaVideoServiceReleasesHoldAfterUpstreamCreateError(t *testing.T) {
	gid := int64(7)
	billingRepo := &stubDramaBillingRepo{}
	svc := &DramaVideoService{
		tasks:        &stubDramaTasks{},
		accounts:     stubDramaAccounts{account: Account{ID: 3, Platform: PlatformDrama, Credentials: map[string]any{"api_key": "tok"}}},
		client:       failingDramaClient{},
		billing:      NewBillingService(nil, nil),
		usageBilling: billingRepo,
		outputDir:    t.TempDir(),
		backgroundRunner: func(fn func()) {
			fn()
		},
	}
	apiKey := &APIKey{
		ID:      11,
		UserID:  22,
		GroupID: &gid,
		Group:   &Group{ID: gid, Platform: PlatformDrama, RateMultiplier: 1},
	}
	got, err := svc.Create(context.Background(), apiKey, []byte(`{"model":"seedance2.0-A","prompt":"hi","resolution":"480p","seconds":4}`), "/v1/videos")
	require.NoError(t, err)
	require.Equal(t, []string{DramaVideoHoldRequestID(got.Task.ID)}, billingRepo.holds)
	require.Equal(t, []string{DramaVideoReleaseRequestID(got.Task.ID)}, billingRepo.releases)
	require.Equal(t, DramaVideoHoldRequestID(got.Task.ID), (&BatchImageBalanceHoldCommand{BatchID: got.Task.ID}).HoldClaimRequestID())
}

func TestDramaVideoServiceIsolatesOwner(t *testing.T) {
	gid := int64(7)
	tasks := &stubDramaTasks{}
	svc := &DramaVideoService{
		tasks:        tasks,
		accounts:     stubDramaAccounts{account: Account{ID: 3, Credentials: map[string]any{"api_key": "tok"}}},
		client:       stubDramaClient{},
		billing:      NewBillingService(nil, nil),
		usageBilling: &stubDramaBillingRepo{},
		outputDir:    t.TempDir(),
		backgroundRunner: func(fn func()) {
			fn()
		},
	}
	apiKey := &APIKey{ID: 11, UserID: 22, GroupID: &gid, Group: &Group{ID: gid, Platform: PlatformDrama, RateMultiplier: 1}}
	got, err := svc.Create(context.Background(), apiKey, []byte(`{"model":"seedance2.0-F","prompt":"hi","resolution":"720p","seconds":8}`), "/v1/video/generations")
	require.NoError(t, err)
	_, err = svc.Get(context.Background(), DramaVideoOwner{UserID: 99, APIKeyID: 11}, got.Task.ID)
	require.Error(t, err)
	_, err = svc.Get(context.Background(), DramaVideoOwner{UserID: 22, APIKeyID: 11}, got.Task.ID)
	require.NoError(t, err)
}

func TestDramaVideoServiceRejectsWrongCreatePath(t *testing.T) {
	gid := int64(7)
	svc := &DramaVideoService{
		tasks:        &stubDramaTasks{},
		accounts:     stubDramaAccounts{account: Account{ID: 3, Credentials: map[string]any{"api_key": "tok"}}},
		client:       stubDramaClient{},
		billing:      NewBillingService(nil, nil),
		usageBilling: &stubDramaBillingRepo{},
	}
	apiKey := &APIKey{ID: 1, UserID: 2, GroupID: &gid, Group: &Group{ID: gid, Platform: PlatformDrama}}
	_, err := svc.Create(context.Background(), apiKey, []byte(`{"model":"seedance2.0-B","prompt":"hi"}`), "/v1/videos")
	require.Error(t, err)
}

func TestDramaVideoServiceList(t *testing.T) {
	tasks := &stubDramaTasks{byID: map[string]*DramaVideoTask{
		"vidtask_a": {TaskID: "vidtask_a", UserID: 22, APIKeyID: 11, Model: "seedance2.0-A", Status: DramaVideoStatusQueued},
		"vidtask_b": {TaskID: "vidtask_b", UserID: 22, APIKeyID: 99, Model: "seedance2.0-B", Status: DramaVideoStatusCompleted},
		"vidtask_c": {TaskID: "vidtask_c", UserID: 22, APIKeyID: 11, Model: "minimax-h3", Status: DramaVideoStatusInProgress},
	}}
	svc := &DramaVideoService{tasks: tasks}
	got, err := svc.List(context.Background(), DramaVideoOwner{UserID: 22, APIKeyID: 11}, 20, 0)
	require.NoError(t, err)
	require.Equal(t, "list", got.Object)
	require.Len(t, got.Data, 2)
	ids := map[string]bool{}
	for _, item := range got.Data {
		ids[item.ID] = true
	}
	require.True(t, ids["vidtask_a"])
	require.True(t, ids["vidtask_c"])
	require.False(t, ids["vidtask_b"])
}
