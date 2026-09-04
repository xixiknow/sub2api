package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SeedanceVideoModel0826     = "seedance-2.0-0826"
	SeedanceVideoModel0826Fast = "seedance-2.0-fast-0826"

	seedanceVideoTaskIDPrefix = "video_"
)

var (
	ErrSeedanceVideoForbidden      = infraerrors.New(http.StatusForbidden, "SEEDANCE_VIDEO_FORBIDDEN", "video request does not belong to this API key")
	ErrSeedanceVideoNoAccount      = infraerrors.New(http.StatusServiceUnavailable, "SEEDANCE_VIDEO_NO_ACCOUNT", "no available Drama account")
	ErrSeedanceVideoNotReady       = infraerrors.New(http.StatusConflict, "SEEDANCE_VIDEO_NOT_READY", "video task is not completed")
	ErrSeedanceVideoContentMissing = infraerrors.New(http.StatusGone, "SEEDANCE_VIDEO_CONTENT_MISSING", "video content is not available")
	ErrSeedanceVideoUpstream       = infraerrors.New(http.StatusBadGateway, "SEEDANCE_VIDEO_UPSTREAM_ERROR", "seedance upstream request failed")
)

type SeedanceVideoUploadSession struct {
	RequestID string            `json:"request_id"`
	UploadID  string            `json:"upload_id"`
	UploadURL string            `json:"upload_url"`
	Method    string            `json:"method"`
	FileField string            `json:"file_field"`
	FormData  map[string]string `json:"form_data"`
	Filename  string            `json:"filename"`
	MediaType string            `json:"media_type"`
	SizeBytes int64             `json:"size_bytes"`
	ExpiresAt int64             `json:"expires_at"`
}

type SeedanceVideoTaskMaterial struct {
	UploadID string `json:"upload_id"`
	Role     string `json:"role"`
}

type SeedanceVideoTaskRequest struct {
	Channel         string                      `json:"channel"`
	Model           string                      `json:"model"`
	Prompt          string                      `json:"prompt"`
	Seconds         *int                        `json:"seconds,omitempty"`
	Duration        *int                        `json:"duration,omitempty"`
	Resolution      string                      `json:"resolution"`
	AspectRatio     string                      `json:"aspect_ratio,omitempty"`
	Ratio           string                      `json:"ratio,omitempty"`
	TaskMode        string                      `json:"task_mode,omitempty"`
	GenerateAudio   *bool                       `json:"generate_audio,omitempty"`
	GenerateAudioV2 *bool                       `json:"generateAudio,omitempty"`
	ReturnLastFrame *bool                       `json:"return_last_frame,omitempty"`
	WebSearch       *bool                       `json:"web_search,omitempty"`
	Priority        *int                        `json:"priority,omitempty"`
	Materials       []SeedanceVideoTaskMaterial `json:"materials,omitempty"`
}

type SeedanceVideoTaskResult struct {
	VideoURL     string `json:"video_url"`
	LastFrameURL string `json:"last_frame_url,omitempty"`
}

type SeedanceVideoTaskMetadata struct {
	URL string `json:"url,omitempty"`
}

type SeedanceVideoTaskError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type SeedanceVideoTaskResponse struct {
	ID             string                     `json:"id,omitempty"`
	RequestID      string                     `json:"request_id,omitempty"`
	TaskID         string                     `json:"task_id,omitempty"`
	Object         string                     `json:"object,omitempty"`
	Model          string                     `json:"model,omitempty"`
	Channel        string                     `json:"channel,omitempty"`
	Status         string                     `json:"status,omitempty"`
	Progress       int                        `json:"progress,omitempty"`
	CreditsCharged int                        `json:"credits_charged,omitempty"`
	BillingStatus  string                     `json:"billing_status,omitempty"`
	CreatedAt      int64                      `json:"created_at,omitempty"`
	UpdatedAt      int64                      `json:"updated_at,omitempty"`
	CompletedAt    *int64                     `json:"completed_at,omitempty"`
	Metadata       *SeedanceVideoTaskMetadata `json:"metadata,omitempty"`
	Result         *SeedanceVideoTaskResult   `json:"result,omitempty"`
	Error          *SeedanceVideoTaskError    `json:"error,omitempty"`
}

func (t *SeedanceVideoTaskResponse) PublicID() string {
	if t == nil {
		return ""
	}
	if id := strings.TrimSpace(t.ID); id != "" {
		return id
	}
	if id := strings.TrimSpace(t.TaskID); id != "" {
		return id
	}
	return strings.TrimSpace(t.RequestID)
}

func (t *SeedanceVideoTaskResponse) ContentURL() string {
	if t == nil {
		return ""
	}
	if t.Metadata != nil {
		if url := strings.TrimSpace(t.Metadata.URL); url != "" {
			return url
		}
	}
	if t.Result != nil {
		return strings.TrimSpace(t.Result.VideoURL)
	}
	return ""
}

type SeedanceVideoContent struct {
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

type SeedanceVideoService struct {
	accounts      AccountRepository
	client        DramaVideoClient
	tasks         DramaVideoTaskRepository
	gateway       *GatewayService
	subscriptions *SubscriptionService
	usageBilling  UsageBillingRepository
	apiKeyQuota   APIKeyQuotaUpdater
	authCache     apiKeyAuthCacheInvalidator
	downloadHTTP  *http.Client
}

func NewSeedanceVideoService(
	accounts AccountRepository,
	client DramaVideoClient,
	tasks DramaVideoTaskRepository,
	gateway *GatewayService,
	subscriptions *SubscriptionService,
	usageBilling UsageBillingRepository,
	apiKeyQuota APIKeyQuotaUpdater,
	authCache apiKeyAuthCacheInvalidator,
) *SeedanceVideoService {
	return &SeedanceVideoService{
		accounts:      accounts,
		client:        client,
		tasks:         tasks,
		gateway:       gateway,
		subscriptions: subscriptions,
		usageBilling:  usageBilling,
		apiKeyQuota:   apiKeyQuota,
		authCache:     authCache,
		downloadHTTP: &http.Client{
			Timeout: 6 * time.Minute,
		},
	}
}

func (s *SeedanceVideoService) CreateUploadSession(ctx context.Context, apiKey *APIKey, rawBody []byte) (*SeedanceVideoUploadSession, error) {
	if s == nil || s.accounts == nil || s.client == nil {
		return nil, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_UNAVAILABLE", "Seedance video service is not available")
	}
	if apiKey == nil || apiKey.ID <= 0 || apiKey.UserID <= 0 || apiKey.GroupID == nil || apiKey.Group == nil {
		return nil, ErrSeedanceVideoForbidden
	}
	platform := apiKey.Group.Platform
	if resolvedPlatform, ok := ResolvedTargetPlatformFromContext(ctx); ok {
		platform = resolvedPlatform
	}
	if platform != PlatformDrama {
		return nil, infraerrors.NotFound("SEEDANCE_VIDEO_UNSUPPORTED_PLATFORM", "videos API is not supported for this platform")
	}
	payload, err := parseSeedanceVideoUploadSessionPayload(rawBody)
	if err != nil {
		return nil, err
	}
	account, err := s.selectAccount(ctx, *apiKey.GroupID)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "invalid upload session request")
	}
	return s.client.CreateVideoUploadSession(ctx, &account, body)
}

func (s *SeedanceVideoService) CreateTask(ctx context.Context, apiKey *APIKey, rawBody []byte) (*SeedanceVideoTaskResponse, error) {
	if s == nil || s.accounts == nil || s.client == nil || s.tasks == nil {
		return nil, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_UNAVAILABLE", "Seedance video service is not available")
	}
	if apiKey == nil || apiKey.ID <= 0 || apiKey.UserID <= 0 || apiKey.GroupID == nil || apiKey.Group == nil {
		return nil, ErrSeedanceVideoForbidden
	}
	platform := apiKey.Group.Platform
	if resolvedPlatform, ok := ResolvedTargetPlatformFromContext(ctx); ok {
		platform = resolvedPlatform
	}
	if platform != PlatformDrama {
		return nil, infraerrors.NotFound("SEEDANCE_VIDEO_UNSUPPORTED_PLATFORM", "videos API is not supported for this platform")
	}
	payload, err := parseSeedanceVideoTaskPayload(rawBody)
	if err != nil {
		return nil, err
	}
	account, err := s.selectAccount(ctx, *apiKey.GroupID)
	if err != nil {
		return nil, err
	}
	upstreamModel := resolveSeedanceVideoModel(&account, payload.Model)
	if err := validateSeedanceVideoTaskModel(upstreamModel, payload.Resolution); err != nil {
		return nil, err
	}
	requestedModel := payload.Model
	payload.Model = upstreamModel
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "invalid video task request")
	}
	durationSeconds := 0
	if payload.Seconds != nil {
		durationSeconds = *payload.Seconds
	}
	cost, _, err := s.calculateSeedanceVideoCost(ctx, apiKey, requestedModel, upstreamModel, payload.Resolution, durationSeconds)
	if err != nil {
		return nil, err
	}
	holdAmount := 0.0
	if cost != nil {
		holdAmount = cost.ActualCost
	}
	subscription, err := s.seedanceActiveSubscription(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	taskID, err := NewSeedanceVideoTaskID()
	if err != nil {
		return nil, infraerrors.InternalServer("SEEDANCE_VIDEO_TASK_ID_FAILED", "failed to create Seedance video task id").WithCause(err)
	}
	requestHash := HashUsageRequestPayload(body)
	if subscription == nil {
		if err := s.reserveSeedanceHold(ctx, apiKey, taskID, requestHash, holdAmount); err != nil {
			return nil, err
		}
	} else {
		holdAmount = 0
	}
	localTask, err := s.persistSeedanceTask(ctx, apiKey, &account, taskID, requestedModel, upstreamModel, payload, body, holdAmount)
	if err != nil {
		if subscription == nil {
			_ = s.releaseSeedanceHold(context.Background(), apiKey, taskID, requestHash, holdAmount)
		}
		return nil, infraerrors.InternalServer("SEEDANCE_VIDEO_TASK_CREATE_FAILED", "failed to persist Seedance video task").WithCause(err)
	}
	task, err := s.client.CreateSeedanceVideoTask(ctx, &account, body)
	if err != nil {
		s.failSeedanceTask(context.Background(), apiKey, taskID, requestHash, holdAmount, "upstream_create_error", err.Error(), subscription == nil)
		return nil, err
	}
	s.syncSeedanceTaskStatus(ctx, apiKey, localTask, task)
	if seedanceVideoTaskIsCompleted(task) {
		if err := s.recordSeedanceCompletedUsage(ctx, apiKey, &account, localTask, task); err != nil {
			slog.Warn("seedance_video_usage_record_failed", "task_id", localTask.TaskID, "upstream_task_id", task.PublicID(), "api_key_id", apiKey.ID, "account_id", account.ID, "error", err)
		}
	}
	exposeSeedanceLocalTaskID(task, localTask.TaskID)
	return task, nil
}

func NewSeedanceVideoTaskID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return seedanceVideoTaskIDPrefix + hex.EncodeToString(buf), nil
}

func (s *SeedanceVideoService) GetTask(ctx context.Context, apiKey *APIKey, taskID string) (*SeedanceVideoTaskResponse, error) {
	if s == nil || s.accounts == nil || s.client == nil {
		return nil, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_UNAVAILABLE", "Seedance video service is not available")
	}
	if apiKey == nil || apiKey.ID <= 0 || apiKey.UserID <= 0 || apiKey.GroupID == nil || apiKey.Group == nil {
		return nil, ErrSeedanceVideoForbidden
	}
	platform := apiKey.Group.Platform
	if resolvedPlatform, ok := ResolvedTargetPlatformFromContext(ctx); ok {
		platform = resolvedPlatform
	}
	if platform != PlatformDrama {
		return nil, infraerrors.NotFound("SEEDANCE_VIDEO_UNSUPPORTED_PLATFORM", "videos API is not supported for this platform")
	}
	localTask, err := s.lookupSeedanceTask(ctx, apiKey, taskID)
	if err != nil {
		return nil, err
	}
	account, err := s.accountForSeedanceTask(ctx, apiKey, localTask)
	if err != nil {
		return nil, err
	}
	upstreamTaskID := strings.TrimSpace(taskID)
	if localTask != nil && strings.TrimSpace(localTask.UpstreamTaskID) != "" {
		upstreamTaskID = strings.TrimSpace(localTask.UpstreamTaskID)
	}
	task, err := s.client.GetSeedanceVideoTask(ctx, &account, upstreamTaskID)
	if err != nil {
		return nil, err
	}
	s.syncSeedanceTaskStatus(ctx, apiKey, localTask, task)
	if localTask != nil && seedanceVideoTaskIsCompleted(task) {
		if err := s.recordSeedanceCompletedUsage(ctx, apiKey, &account, localTask, task); err != nil {
			slog.Warn("seedance_video_usage_record_failed", "task_id", localTask.TaskID, "upstream_task_id", task.PublicID(), "api_key_id", apiKey.ID, "account_id", account.ID, "error", err)
		}
	}
	if localTask != nil {
		exposeSeedanceLocalTaskID(task, localTask.TaskID)
	}
	return task, nil
}

func (s *SeedanceVideoService) Content(ctx context.Context, apiKey *APIKey, taskID string, method string) (*SeedanceVideoContent, error) {
	task, err := s.GetTask(ctx, apiKey, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil || strings.ToLower(strings.TrimSpace(task.Status)) != "completed" || strings.TrimSpace(task.ContentURL()) == "" {
		return nil, ErrSeedanceVideoNotReady
	}
	return s.downloadURL(ctx, task.ContentURL(), method)
}

func (s *SeedanceVideoService) persistSeedanceTask(
	ctx context.Context,
	apiKey *APIKey,
	account *Account,
	taskID string,
	requestedModel string,
	upstreamModel string,
	payload seedanceVideoTaskCreatePayload,
	upstreamBody []byte,
	holdAmount float64,
) (*DramaVideoTask, error) {
	if s == nil || s.tasks == nil || apiKey == nil || account == nil {
		return nil, nil
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil
	}
	durationSeconds := 0
	if payload.Seconds != nil {
		durationSeconds = *payload.Seconds
	}
	localTask, err := s.tasks.Create(ctx, CreateDramaVideoTaskParams{
		TaskID:          taskID,
		UserID:          apiKey.UserID,
		APIKeyID:        apiKey.ID,
		GroupID:         *apiKey.GroupID,
		AccountID:       account.ID,
		Model:           requestedModel,
		UpstreamModel:   upstreamModel,
		Status:          DramaVideoStatusCreated,
		Progress:        0,
		RequestHash:     HashUsageRequestPayload(upstreamBody),
		Resolution:      NormalizeVideoBillingResolutionOrDefault(payload.Resolution),
		AspectRatio:     payload.AspectRatio,
		DurationSeconds: NormalizeVideoBillingDurationSecondsOrDefault(durationSeconds),
		HoldAmount:      holdAmount,
	})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	_, _ = s.tasks.UpdateStatus(ctx, DramaVideoTaskStatusUpdate{
		TaskID:      taskID,
		Status:      DramaVideoStatusSubmitting,
		SubmittedAt: &now,
	})
	return localTask, nil
}

func (s *SeedanceVideoService) lookupSeedanceTask(ctx context.Context, apiKey *APIKey, taskID string) (*DramaVideoTask, error) {
	if s == nil || s.tasks == nil || apiKey == nil {
		return nil, nil
	}
	task, err := s.tasks.GetByTaskID(ctx, strings.TrimSpace(taskID))
	if err != nil {
		if errors.Is(err, ErrDramaVideoTaskNotFound) {
			return nil, nil
		}
		return nil, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_TASK_LOOKUP_FAILED", "failed to load seedance video task").WithCause(err)
	}
	if task.UserID != apiKey.UserID || task.APIKeyID != apiKey.ID {
		return nil, ErrSeedanceVideoForbidden
	}
	return task, nil
}

func (s *SeedanceVideoService) accountForSeedanceTask(ctx context.Context, apiKey *APIKey, task *DramaVideoTask) (Account, error) {
	if task != nil && task.AccountID != nil && *task.AccountID > 0 {
		account, err := s.accounts.GetByID(ctx, *task.AccountID)
		if err != nil {
			return Account{}, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_ACCOUNT_LOOKUP_FAILED", "failed to load seedance video account").WithCause(err)
		}
		if account == nil {
			return Account{}, ErrSeedanceVideoNoAccount
		}
		return *account, nil
	}
	return s.selectAccount(ctx, *apiKey.GroupID)
}

func (s *SeedanceVideoService) syncSeedanceTaskStatus(ctx context.Context, apiKey *APIKey, localTask *DramaVideoTask, task *SeedanceVideoTaskResponse) {
	if s == nil || s.tasks == nil || localTask == nil || task == nil {
		return
	}
	status := normalizeSeedanceVideoStatus(task.Status)
	progress := seedanceVideoProgress(task)
	upstreamID := strings.TrimSpace(task.PublicID())
	var completedAt *time.Time
	if isSeedanceTerminalStatus(status) {
		completedAt = seedanceVideoCompletedAt(task)
	}
	if _, err := s.tasks.UpdateStatus(ctx, DramaVideoTaskStatusUpdate{
		TaskID:         localTask.TaskID,
		Status:         status,
		Progress:       &progress,
		UpstreamTaskID: &upstreamID,
		Error:          seedanceVideoErrorJSON(task),
		CompletedAt:    completedAt,
	}); err != nil {
		slog.Warn("seedance_video_task_status_update_failed", "task_id", localTask.TaskID, "error", err)
	}
	if isSeedanceUnsuccessfulTerminalStatus(status) && localTask.ActualCost == nil {
		if err := s.releaseSeedanceHold(context.Background(), apiKey, localTask.TaskID, localTask.RequestHash, localTask.HoldAmount); err != nil {
			slog.Warn("seedance_video_hold_release_failed", "task_id", localTask.TaskID, "error", err)
		}
	}
}

func (s *SeedanceVideoService) recordSeedanceCompletedUsage(
	ctx context.Context,
	apiKey *APIKey,
	account *Account,
	localTask *DramaVideoTask,
	task *SeedanceVideoTaskResponse,
) error {
	if s == nil || s.gateway == nil || s.gateway.billingService == nil || s.gateway.usageLogRepo == nil ||
		apiKey == nil || apiKey.User == nil || apiKey.Group == nil || account == nil || localTask == nil {
		return nil
	}
	if localTask.ActualCost != nil {
		return nil
	}
	durationSeconds := NormalizeVideoBillingDurationSecondsOrDefault(localTask.DurationSeconds)
	resolution := NormalizeVideoBillingResolutionOrDefault(localTask.Resolution)
	cost, videoRate, err := s.calculateSeedanceVideoCost(ctx, apiKey, localTask.Model, localTask.UpstreamModel, resolution, durationSeconds)
	if err != nil {
		return err
	}
	if cost == nil {
		cost = &CostBreakdown{BillingMode: string(BillingModeVideo)}
	}
	billingMode := string(BillingModeVideo)
	accountRateMultiplier := account.BillingRateMultiplier()
	billingType := BillingTypeBalance
	subscription, err := s.seedanceActiveSubscription(ctx, apiKey)
	if err != nil {
		return err
	}
	if subscription != nil {
		billingType = BillingTypeSubscription
	}
	if subscription == nil && shouldCaptureSeedanceHold(localTask) && localTask.HoldAmount > 0 && cost.ActualCost > localTask.HoldAmount {
		slog.Warn("seedance_video_actual_cost_exceeds_hold_capping_to_hold", "task_id", localTask.TaskID, "actual_cost", cost.ActualCost, "hold_amount", localTask.HoldAmount)
		cost.ActualCost = localTask.HoldAmount
		cost.TotalCost = localTask.HoldAmount / nonZeroMultiplier(videoRate)
	}
	durationMs := int(time.Since(localTask.CreatedAt).Milliseconds())
	if durationMs < 0 {
		durationMs = 0
	}
	requestID := SeedanceVideoCaptureRequestID(localTask.TaskID)
	usageLog := &UsageLog{
		UserID:                apiKey.UserID,
		APIKeyID:              apiKey.ID,
		AccountID:             account.ID,
		RequestID:             requestID,
		Model:                 localTask.Model,
		RequestedModel:        localTask.Model,
		UpstreamModel:         optionalTrimmedStringPtr(localTask.UpstreamModel),
		GroupID:               apiKey.GroupID,
		SubscriptionID:        optionalSubscriptionID(subscription),
		TotalCost:             cost.TotalCost,
		ActualCost:            cost.ActualCost,
		RateMultiplier:        videoRate,
		AccountRateMultiplier: &accountRateMultiplier,
		BillingType:           billingType,
		RequestType:           RequestTypeSync,
		Stream:                false,
		DurationMs:            &durationMs,
		VideoCount:            1,
		VideoResolution:       &resolution,
		VideoDurationSeconds:  &durationSeconds,
		BillingMode:           &billingMode,
		InboundEndpoint:       optionalTrimmedStringPtr("/v1/videos/tasks/:task_id"),
		UpstreamEndpoint:      optionalTrimmedStringPtr("/v1/videos/{task_id}"),
		CreatedAt:             time.Now(),
	}
	isSubscriptionBill := subscription != nil && apiKey.Group.IsSubscriptionType()
	if subscription == nil && shouldCaptureSeedanceHold(localTask) && localTask.HoldAmount > 0 {
		captureApplied, err := s.captureSeedanceHold(ctx, apiKey, localTask.TaskID, localTask.RequestHash, localTask.HoldAmount, cost.ActualCost)
		if err != nil {
			return err
		}
		if captureApplied {
			s.recordSeedanceQuotaUsage(ctx, apiKey, cost.ActualCost)
		}
		writeUsageLogBestEffort(ctx, s.gateway.usageLogRepo, usageLog, "service.seedance_video")
		if captureApplied && s.authCache != nil && apiKey.Key != "" {
			s.authCache.InvalidateAuthCacheByKey(ctx, apiKey.Key)
		}
		s.markSeedanceCompleted(ctx, localTask, task, cost.ActualCost)
		return nil
	}
	applied, billingErr := applyUsageBilling(ctx, requestID, usageLog, &postUsageBillingParams{
		Cost:                  cost,
		User:                  apiKey.User,
		APIKey:                apiKey,
		Account:               account,
		Subscription:          subscription,
		RequestPayloadHash:    localTask.RequestHash,
		IsSubscriptionBill:    isSubscriptionBill,
		AccountRateMultiplier: accountRateMultiplier,
		APIKeyService:         s.apiKeyQuota,
		Platform:              QuotaPlatform(ctx, apiKey),
	}, s.gateway.billingDeps(), s.gateway.usageBillingRepo)
	if billingErr != nil {
		failedLog := *usageLog
		failedLog.ActualCost = 0
		writeUsageLogBestEffort(ctx, s.gateway.usageLogRepo, &failedLog, "service.seedance_video")
		return billingErr
	}
	writeUsageLogBestEffort(ctx, s.gateway.usageLogRepo, usageLog, "service.seedance_video")
	if applied && s.authCache != nil && apiKey.Key != "" {
		s.authCache.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	}
	s.markSeedanceCompleted(ctx, localTask, task, cost.ActualCost)
	return nil
}

func (s *SeedanceVideoService) markSeedanceCompleted(ctx context.Context, localTask *DramaVideoTask, task *SeedanceVideoTaskResponse, actualCost float64) {
	completedAt := seedanceVideoCompletedAt(task)
	if completedAt == nil {
		now := time.Now()
		completedAt = &now
	}
	if s.tasks != nil {
		if _, err := s.tasks.MarkCompleted(ctx, DramaVideoTaskCompletionUpdate{
			TaskID:      localTask.TaskID,
			ActualCost:  actualCost,
			OutputPath:  task.ContentURL(),
			OutputMIME:  "video/mp4",
			CompletedAt: *completedAt,
		}); err != nil {
			slog.Warn("seedance_video_task_mark_completed_failed", "task_id", localTask.TaskID, "error", err)
		}
	}
}

func (s *SeedanceVideoService) reserveSeedanceHold(ctx context.Context, apiKey *APIKey, taskID, requestHash string, holdAmount float64) error {
	if holdAmount <= 0 {
		return nil
	}
	if s == nil || s.usageBilling == nil || apiKey == nil {
		return infraerrors.New(infraerrors.Code(ErrBatchImageBillingHoldFailed), "SEEDANCE_VIDEO_BILLING_HOLD_FAILED", "Seedance video balance hold failed")
	}
	_, err := s.usageBilling.ReserveBatchImageBalance(ctx, &BatchImageBalanceHoldCommand{
		RequestID:          SeedanceVideoHoldRequestID(taskID),
		APIKeyID:           apiKey.ID,
		UserID:             apiKey.UserID,
		BatchID:            taskID,
		HoldAmount:         holdAmount,
		RequestPayloadHash: requestHash,
	})
	if err != nil {
		if errors.Is(err, ErrBatchImageInsufficientBalance) {
			return infraerrors.New(infraerrors.Code(ErrBatchImageInsufficientBalance), "SEEDANCE_VIDEO_INSUFFICIENT_BALANCE", infraerrors.Message(ErrBatchImageInsufficientBalance)).WithCause(err)
		}
		return infraerrors.New(infraerrors.Code(ErrBatchImageBillingHoldFailed), "SEEDANCE_VIDEO_BILLING_HOLD_FAILED", "Seedance video balance hold failed").WithCause(err)
	}
	s.invalidateSeedanceBillingState(ctx, apiKey)
	return nil
}

func (s *SeedanceVideoService) captureSeedanceHold(ctx context.Context, apiKey *APIKey, taskID, requestHash string, holdAmount, actualAmount float64) (bool, error) {
	if holdAmount < 0 {
		holdAmount = 0
	}
	if actualAmount < 0 {
		actualAmount = 0
	}
	if holdAmount <= 0 && actualAmount <= 0 {
		return false, nil
	}
	if actualAmount-holdAmount > 0.00000001 {
		return false, infraerrors.New(infraerrors.Code(ErrBatchImageSettlementCostExceedsHold), "SEEDANCE_VIDEO_BILLING_CAPTURE_EXCEEDS_HOLD", "Seedance video settlement cost exceeds held balance")
	}
	if s == nil || s.usageBilling == nil || apiKey == nil {
		return false, infraerrors.New(infraerrors.Code(ErrBatchImageSettlementBillingFailed), "SEEDANCE_VIDEO_BILLING_CAPTURE_FAILED", "Seedance video billing capture failed")
	}
	result, err := s.usageBilling.CaptureBatchImageBalance(ctx, &BatchImageBalanceHoldCommand{
		RequestID:          SeedanceVideoCaptureRequestID(taskID),
		APIKeyID:           apiKey.ID,
		UserID:             apiKey.UserID,
		BatchID:            taskID,
		HoldAmount:         holdAmount,
		ActualAmount:       actualAmount,
		RequestPayloadHash: requestHash,
	})
	if err != nil {
		return false, infraerrors.New(infraerrors.Code(ErrBatchImageSettlementBillingFailed), "SEEDANCE_VIDEO_BILLING_CAPTURE_FAILED", "Seedance video billing capture failed").WithCause(err)
	}
	if result != nil && result.Applied {
		s.invalidateSeedanceBillingState(ctx, apiKey)
		return true, nil
	}
	return false, nil
}

func (s *SeedanceVideoService) releaseSeedanceHold(ctx context.Context, apiKey *APIKey, taskID, requestHash string, holdAmount float64) error {
	if holdAmount <= 0 || s == nil || s.usageBilling == nil || apiKey == nil {
		return nil
	}
	_, err := s.usageBilling.ReleaseBatchImageBalance(ctx, &BatchImageBalanceHoldCommand{
		RequestID:          SeedanceVideoReleaseRequestID(taskID),
		APIKeyID:           apiKey.ID,
		UserID:             apiKey.UserID,
		BatchID:            taskID,
		HoldAmount:         holdAmount,
		RequestPayloadHash: requestHash,
	})
	if err != nil {
		if errors.Is(err, ErrUsageBillingRequestConflict) {
			slog.Warn("seedance_video_release_fingerprint_conflict_treated_as_released", "task_id", strings.TrimSpace(taskID))
			return nil
		}
		return err
	}
	s.invalidateSeedanceBillingState(ctx, apiKey)
	return nil
}

func (s *SeedanceVideoService) failSeedanceTask(ctx context.Context, apiKey *APIKey, taskID, requestHash string, holdAmount float64, code, message string, release bool) {
	if s != nil && s.tasks != nil {
		completedAt := time.Now()
		_, _ = s.tasks.UpdateStatus(ctx, DramaVideoTaskStatusUpdate{
			TaskID:      taskID,
			Status:      DramaVideoStatusFailed,
			Error:       seedanceVideoErrorJSONFromFields(code, message),
			CompletedAt: &completedAt,
		})
	}
	if release {
		if err := s.releaseSeedanceHold(ctx, apiKey, taskID, requestHash, holdAmount); err != nil {
			slog.Warn("seedance_video_hold_release_failed", "task_id", taskID, "error", err)
		}
	}
}

func (s *SeedanceVideoService) recordSeedanceQuotaUsage(ctx context.Context, apiKey *APIKey, amount float64) {
	if s == nil || s.apiKeyQuota == nil || apiKey == nil || amount <= 0 {
		return
	}
	if err := s.apiKeyQuota.UpdateQuotaUsed(ctx, apiKey.ID, amount); err != nil {
		slog.Warn("seedance_video_api_key_quota_update_failed", "api_key_id", apiKey.ID, "error", err)
	}
	if err := s.apiKeyQuota.UpdateRateLimitUsage(ctx, apiKey.ID, amount); err != nil {
		slog.Warn("seedance_video_api_key_rate_limit_update_failed", "api_key_id", apiKey.ID, "error", err)
	}
}

func (s *SeedanceVideoService) invalidateSeedanceBillingState(ctx context.Context, apiKey *APIKey) {
	if apiKey == nil {
		return
	}
	if s != nil && s.gateway != nil && s.gateway.billingCacheService != nil {
		_ = s.gateway.billingCacheService.InvalidateUserBalance(ctx, apiKey.UserID)
	}
	if s != nil && s.authCache != nil && apiKey.Key != "" {
		s.authCache.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	}
}

func (s *SeedanceVideoService) seedanceActiveSubscription(ctx context.Context, apiKey *APIKey) (*UserSubscription, error) {
	if apiKey == nil || apiKey.Group == nil || apiKey.GroupID == nil || !apiKey.Group.IsSubscriptionType() {
		return nil, nil
	}
	if s == nil || s.subscriptions == nil {
		return nil, infraerrors.Forbidden("SEEDANCE_VIDEO_SUBSCRIPTION_NOT_FOUND", "No active subscription found for this group")
	}
	subscription, err := s.subscriptions.GetActiveSubscription(ctx, apiKey.UserID, *apiKey.GroupID)
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		return nil, infraerrors.Forbidden("SEEDANCE_VIDEO_SUBSCRIPTION_NOT_FOUND", "No active subscription found for this group")
	}
	return subscription, nil
}

func (s *SeedanceVideoService) calculateSeedanceVideoCost(
	ctx context.Context,
	apiKey *APIKey,
	requestedModel string,
	upstreamModel string,
	resolution string,
	durationSeconds int,
) (*CostBreakdown, float64, error) {
	videoRate := 1.0
	if apiKey != nil && apiKey.GroupID != nil && apiKey.Group != nil {
		baseMultiplier := apiKey.Group.RateMultiplier
		if s != nil && s.gateway != nil {
			baseMultiplier = s.gateway.ResolveUserGroupRateMultiplier(ctx, apiKey.UserID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
		}
		if baseMultiplier < 0 {
			baseMultiplier = 0
		}
		videoRate = resolveVideoRateMultiplier(apiKey, baseMultiplier)
	}
	if videoRate < 0 {
		videoRate = 0
	}
	if s == nil || s.gateway == nil || s.gateway.billingService == nil {
		return &CostBreakdown{BillingMode: string(BillingModeVideo)}, videoRate, nil
	}
	resolution = NormalizeVideoBillingResolutionOrDefault(resolution)
	durationSeconds = NormalizeVideoBillingDurationSecondsOrDefault(durationSeconds)
	if s.gateway.resolver != nil && apiKey != nil && apiKey.Group != nil {
		gid := apiKey.Group.ID
		resolved := s.gateway.resolver.Resolve(ctx, PricingInput{
			Model:   requestedModel,
			GroupID: &gid,
			Group:   apiKey.Group,
		})
		if resolved != nil && resolved.Source == PricingSourceChannel &&
			(resolved.Mode == BillingModePerRequest || resolved.Mode == BillingModeImage || resolved.Mode == BillingModeVideo) {
			units := float64(1)
			if resolved.Mode == BillingModeVideo {
				units = float64(durationSeconds)
			}
			cost, err := s.gateway.billingService.CalculateCostUnified(CostInput{
				Ctx:            ctx,
				Model:          requestedModel,
				GroupID:        &gid,
				Group:          apiKey.Group,
				RequestCount:   1,
				UsageUnits:     units,
				SizeTier:       resolution,
				RateMultiplier: videoRate,
				Resolver:       s.gateway.resolver,
				Resolved:       resolved,
			})
			if err == nil && cost != nil {
				cost.BillingMode = string(BillingModeVideo)
				return cost, videoRate, nil
			}
		}
	}
	var groupConfig *VideoPriceConfig
	if apiKey != nil && apiKey.Group != nil {
		groupConfig = apiKey.Group.VideoPriceConfig()
	}
	cost := s.gateway.billingService.CalculateVideoCost(upstreamModel, resolution, 1, durationSeconds, groupConfig, videoRate)
	if cost == nil {
		return nil, videoRate, infraerrors.BadRequest("SEEDANCE_VIDEO_PRICING_FAILED", "failed to calculate seedance video cost")
	}
	return cost, videoRate, nil
}

func normalizeSeedanceVideoStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "pending":
		return DramaVideoStatusQueued
	case "in_progress", "running", "processing":
		return DramaVideoStatusInProgress
	case "completed", "complete", "succeeded", "success", "done":
		return DramaVideoStatusCompleted
	case "failed", "error":
		return DramaVideoStatusFailed
	case "canceled", "cancelled":
		return DramaVideoStatusCanceled
	case "expired":
		return DramaVideoStatusExpired
	default:
		return DramaVideoStatusUnknown
	}
}

func seedanceVideoTaskIsCompleted(task *SeedanceVideoTaskResponse) bool {
	return task != nil && normalizeSeedanceVideoStatus(task.Status) == DramaVideoStatusCompleted && strings.TrimSpace(task.ContentURL()) != ""
}

func seedanceVideoProgress(task *SeedanceVideoTaskResponse) int {
	if seedanceVideoTaskIsCompleted(task) {
		return 100
	}
	if task == nil {
		return 0
	}
	return clampDramaProgress(task.Progress)
}

func isSeedanceTerminalStatus(status string) bool {
	switch status {
	case DramaVideoStatusCompleted, DramaVideoStatusFailed, DramaVideoStatusCanceled, DramaVideoStatusExpired:
		return true
	default:
		return false
	}
}

func isSeedanceUnsuccessfulTerminalStatus(status string) bool {
	switch status {
	case DramaVideoStatusFailed, DramaVideoStatusCanceled, DramaVideoStatusExpired:
		return true
	default:
		return false
	}
}

func seedanceVideoCompletedAt(task *SeedanceVideoTaskResponse) *time.Time {
	if task == nil || task.CompletedAt == nil || *task.CompletedAt <= 0 {
		return nil
	}
	t := time.Unix(*task.CompletedAt, 0)
	return &t
}

func seedanceVideoErrorJSON(task *SeedanceVideoTaskResponse) json.RawMessage {
	if task == nil || task.Error == nil || (strings.TrimSpace(task.Error.Code) == "" && strings.TrimSpace(task.Error.Message) == "") {
		return nil
	}
	data, _ := json.Marshal(task.Error)
	return data
}

func seedanceVideoErrorJSONFromFields(code, message string) json.RawMessage {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "api_error"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "Seedance video task failed"
	}
	data, _ := json.Marshal(SeedanceVideoTaskError{Code: code, Message: message})
	return data
}

func shouldCaptureSeedanceHold(task *DramaVideoTask) bool {
	return task != nil && strings.HasPrefix(strings.TrimSpace(task.TaskID), seedanceVideoTaskIDPrefix)
}

func exposeSeedanceLocalTaskID(task *SeedanceVideoTaskResponse, localTaskID string) {
	if task == nil {
		return
	}
	localTaskID = strings.TrimSpace(localTaskID)
	if localTaskID == "" {
		return
	}
	task.ID = localTaskID
	task.TaskID = localTaskID
}

func SeedanceVideoHoldRequestID(taskID string) string {
	return BatchImageHoldRequestID(taskID)
}

func SeedanceVideoCaptureRequestID(taskID string) string {
	return "seedance_video_capture:" + strings.TrimSpace(taskID)
}

func SeedanceVideoReleaseRequestID(taskID string) string {
	return "seedance_video_release:" + strings.TrimSpace(taskID)
}

func (s *SeedanceVideoService) selectAccount(ctx context.Context, groupID int64) (Account, error) {
	accounts, err := s.accounts.ListSchedulableByGroupIDAndPlatform(ctx, groupID, PlatformDrama)
	if err != nil {
		return Account{}, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_ACCOUNT_LOOKUP_FAILED", "failed to load Drama accounts").WithCause(err)
	}
	for _, account := range accounts {
		if strings.TrimSpace(account.GetCredential("api_key")) != "" || strings.TrimSpace(account.GetCredential("token")) != "" {
			return account, nil
		}
	}
	return Account{}, ErrSeedanceVideoNoAccount
}

func (s *SeedanceVideoService) downloadURL(ctx context.Context, targetURL, method string) (*SeedanceVideoContent, error) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return nil, ErrSeedanceVideoContentMissing
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(strings.TrimSpace(firstNonEmpty(method, http.MethodGet))), targetURL, nil)
	if err != nil {
		return nil, infraerrors.InternalServer("SEEDANCE_VIDEO_CONTENT_REQUEST_FAILED", "failed to build download request").WithCause(err)
	}
	resp, err := s.downloadHTTP.Do(req)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SEEDANCE_VIDEO_CONTENT_DOWNLOAD_FAILED", "failed to download video content").WithCause(err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, infraerrors.New(http.StatusBadGateway, "SEEDANCE_VIDEO_CONTENT_DOWNLOAD_FAILED", fmt.Sprintf("download returned status %d", resp.StatusCode))
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "video/mp4"
	}
	size := resp.ContentLength
	if size < 0 {
		size = 0
	}
	return &SeedanceVideoContent{
		ContentType: contentType,
		Size:        size,
		Body:        resp.Body,
	}, nil
}

func parseSeedanceVideoUploadSessionPayload(rawBody []byte) (seedanceVideoUploadSessionPayload, error) {
	var payload seedanceVideoUploadSessionPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_JSON", "request body must be valid JSON")
	}
	payload.Filename = strings.TrimSpace(payload.Filename)
	payload.MediaType = strings.ToLower(strings.TrimSpace(payload.MediaType))
	payload.ContentType = strings.ToLower(strings.TrimSpace(payload.ContentType))
	if payload.Filename == "" {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "filename is required")
	}
	if payload.SizeBytes <= 0 {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "size_bytes is required")
	}
	switch payload.MediaType {
	case "image":
		if payload.SizeBytes > 20*1024*1024 {
			return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "image files must be 20MB or smaller")
		}
		if !strings.HasPrefix(payload.ContentType, "image/") || !hasSeedanceUploadExtension(payload.Filename, ".png", ".jpg", ".jpeg", ".webp", ".gif") {
			return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "image uploads require an image MIME type and image extension")
		}
	case "video", "audio":
		if payload.DurationSeconds == nil {
			return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "duration_seconds is required for video and audio uploads")
		}
		if *payload.DurationSeconds < 2 || *payload.DurationSeconds > 15 {
			return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "duration_seconds must be between 2 and 15")
		}
		if payload.SizeBytes > 300*1024*1024 {
			return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "video and audio files must be 300MB or smaller")
		}
		if payload.MediaType == "video" {
			if !strings.HasPrefix(payload.ContentType, "video/") || !hasSeedanceUploadExtension(payload.Filename, ".mp4", ".webm", ".mov") {
				return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "video uploads require a video MIME type and video extension")
			}
		} else if !strings.HasPrefix(payload.ContentType, "audio/") || !hasSeedanceUploadExtension(payload.Filename, ".mp3", ".wav", ".m4a", ".aac", ".ogg") {
			return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "audio uploads require an audio MIME type and audio extension")
		}
	default:
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "media_type must be image, video, or audio")
	}
	if payload.ContentType == "" {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "content_type is required")
	}
	return payload, nil
}

func hasSeedanceUploadExtension(filename string, allowed ...string) bool {
	filename = strings.ToLower(strings.TrimSpace(filename))
	for _, ext := range allowed {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}

func parseSeedanceVideoTaskPayload(rawBody []byte) (seedanceVideoTaskCreatePayload, error) {
	var payload seedanceVideoTaskCreatePayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_JSON", "request body must be valid JSON")
	}
	payload.Channel = strings.ToLower(strings.TrimSpace(payload.Channel))
	if payload.Channel == "" {
		payload.Channel = "s"
	}
	if payload.Channel != "s" {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "channel must be s")
	}
	payload.Model = strings.ToLower(strings.TrimSpace(payload.Model))
	if payload.Model == "" {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_MODEL", "model is required")
	}
	payload.Prompt = strings.TrimSpace(payload.Prompt)
	if n := utf8.RuneCountInString(payload.Prompt); n < 1 || n > 4000 {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "prompt must be 1-4000 characters")
	}
	if payload.Seconds == nil {
		payload.Seconds = payload.Duration
	}
	if payload.Seconds == nil {
		defaultSeconds := 5
		payload.Seconds = &defaultSeconds
	}
	if payload.Duration != nil && *payload.Duration != *payload.Seconds {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "seconds and duration must match when both are provided")
	}
	if *payload.Seconds < 4 || *payload.Seconds > 15 {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "seconds must be between 4 and 15")
	}
	payload.Resolution = strings.ToLower(strings.TrimSpace(payload.Resolution))
	if payload.Resolution == "" {
		payload.Resolution = "720p"
	}
	payload.AspectRatio = strings.TrimSpace(payload.AspectRatio)
	payload.Ratio = strings.TrimSpace(payload.Ratio)
	if payload.AspectRatio != "" && payload.Ratio != "" && payload.AspectRatio != payload.Ratio {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "aspect_ratio and ratio must match when both are provided")
	}
	payload.AspectRatio = strings.TrimSpace(firstNonEmpty(payload.AspectRatio, payload.Ratio))
	if payload.AspectRatio == "" {
		payload.AspectRatio = "16:9"
	}
	switch payload.AspectRatio {
	case "adaptive", "16:9", "4:3", "1:1", "3:4", "9:16", "21:9":
	default:
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "aspect_ratio is not supported")
	}
	payload.TaskMode = strings.ToLower(strings.TrimSpace(payload.TaskMode))
	if payload.TaskMode == "" {
		payload.TaskMode = "text"
	}
	switch payload.TaskMode {
	case "text", "first_frame", "first_last_frame", "references":
	default:
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "task_mode is not supported")
	}
	if payload.GenerateAudio == nil && payload.GenerateAudioV2 != nil {
		payload.GenerateAudio = payload.GenerateAudioV2
	}
	if payload.GenerateAudio == nil {
		defaultGenerate := true
		payload.GenerateAudio = &defaultGenerate
	}
	if payload.ReturnLastFrame == nil {
		defaultReturnLastFrame := false
		payload.ReturnLastFrame = &defaultReturnLastFrame
	}
	if payload.WebSearch == nil {
		defaultWebSearch := false
		payload.WebSearch = &defaultWebSearch
	}
	if payload.Priority == nil {
		defaultPriority := 0
		payload.Priority = &defaultPriority
	}
	if *payload.Priority < 0 || *payload.Priority > 9 {
		return payload, infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "priority must be between 0 and 9")
	}
	if err := validateSeedanceVideoMaterials(payload.TaskMode, payload.Materials); err != nil {
		return payload, err
	}
	payload.Duration = nil
	payload.Ratio = ""
	payload.GenerateAudioV2 = nil
	return payload, nil
}

type seedanceVideoUploadSessionPayload struct {
	Filename        string   `json:"filename"`
	MediaType       string   `json:"media_type"`
	ContentType     string   `json:"content_type"`
	SizeBytes       int64    `json:"size_bytes"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
}

type seedanceVideoTaskCreatePayload struct {
	Channel         string                      `json:"channel"`
	Model           string                      `json:"model"`
	Prompt          string                      `json:"prompt"`
	Seconds         *int                        `json:"seconds,omitempty"`
	Duration        *int                        `json:"duration,omitempty"`
	Resolution      string                      `json:"resolution"`
	AspectRatio     string                      `json:"aspect_ratio,omitempty"`
	Ratio           string                      `json:"ratio,omitempty"`
	TaskMode        string                      `json:"task_mode,omitempty"`
	GenerateAudio   *bool                       `json:"generate_audio,omitempty"`
	GenerateAudioV2 *bool                       `json:"generateAudio,omitempty"`
	ReturnLastFrame *bool                       `json:"return_last_frame,omitempty"`
	WebSearch       *bool                       `json:"web_search,omitempty"`
	Priority        *int                        `json:"priority,omitempty"`
	Materials       []SeedanceVideoTaskMaterial `json:"materials,omitempty"`
}

func validateSeedanceVideoMaterials(taskMode string, materials []SeedanceVideoTaskMaterial) error {
	if len(materials) == 0 {
		if taskMode == "text" {
			return nil
		}
		return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "materials are required for this task_mode")
	}
	if len(materials) > 15 {
		return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "materials may contain at most 15 items")
	}

	imageCount := 0
	videoCount := 0
	audioCount := 0
	for index, material := range materials {
		material.UploadID = strings.TrimSpace(material.UploadID)
		material.Role = strings.ToLower(strings.TrimSpace(material.Role))
		if material.UploadID == "" {
			return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", fmt.Sprintf("materials[%d].upload_id is required", index))
		}
		switch taskMode {
		case "text":
			return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "materials are not allowed for text mode")
		case "first_frame":
			if material.Role != "first_frame" {
				return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "first_frame mode requires role=first_frame")
			}
			if index > 0 {
				return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "first_frame mode requires exactly one material")
			}
		case "first_last_frame":
			if material.Role != "first_frame" && material.Role != "last_frame" {
				return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "first_last_frame mode requires first_frame and last_frame roles")
			}
		case "references":
			switch material.Role {
			case "reference_image":
				imageCount++
			case "reference_video":
				videoCount++
			case "reference_audio":
				audioCount++
			default:
				return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "references mode requires reference_image/reference_video/reference_audio roles")
			}
		}
	}
	if taskMode == "first_frame" && len(materials) != 1 {
		return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "first_frame mode requires exactly one material")
	}
	if taskMode == "first_last_frame" {
		if len(materials) != 2 {
			return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "first_last_frame mode requires exactly two materials")
		}
		if strings.ToLower(strings.TrimSpace(materials[0].Role)) != "first_frame" || strings.ToLower(strings.TrimSpace(materials[1].Role)) != "last_frame" {
			return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "first_last_frame materials must be ordered first_frame then last_frame")
		}
	}
	if taskMode == "references" {
		if imageCount > 9 || videoCount > 3 || audioCount > 3 {
			return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "reference counts exceed the supported limits")
		}
	}
	return nil
}

func resolveSeedanceVideoModel(account *Account, requestedModel string) string {
	model := strings.ToLower(strings.TrimSpace(requestedModel))
	if account == nil {
		return model
	}
	if mapping := account.GetModelMapping(); len(mapping) > 0 {
		if mapped, ok := mapping[model]; ok && strings.TrimSpace(mapped) != "" {
			return strings.ToLower(strings.TrimSpace(mapped))
		}
	}
	return model
}

func validateSeedanceVideoTaskModel(model, resolution string) error {
	switch model {
	case SeedanceVideoModel0826Fast:
		switch resolution {
		case "480p", "720p":
			return nil
		default:
			return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "seedance-2.0-fast-0826 only supports 480p and 720p")
		}
	case SeedanceVideoModel0826:
		switch resolution {
		case "480p", "720p", "1080p", "4k":
			return nil
		default:
			return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_REQUEST", "resolution must be 480p, 720p, 1080p, or 4K")
		}
	default:
		return infraerrors.BadRequest("SEEDANCE_VIDEO_INVALID_MODEL", "model is not supported")
	}
}
