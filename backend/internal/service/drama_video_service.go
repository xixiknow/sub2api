package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	dramaVideoPollInterval = 5 * time.Second
	dramaVideoPollTimeout  = 15 * time.Minute
	dramaVideoOutputDir    = "/tmp/sub2api-drama-videos"
)

var (
	dramaVideoAllowedAspectRatios = map[string]struct{}{
		"21:9": {},
		"16:9": {},
		"4:3":  {},
		"1:1":  {},
		"3:4":  {},
		"9:16": {},
	}
	dramaVideoAllowedDurations = map[int]struct{}{5: {}, 10: {}, 15: {}}
)

type DramaVideoService struct {
	tasks            DramaVideoTaskRepository
	accounts         AccountRepository
	client           DramaVideoClient
	billing          *BillingService
	usageBilling     UsageBillingRepository
	usageLogs        UsageLogRepository
	userGroupRates   UserGroupRateRepository
	apiKeyQuota      APIKeyQuotaUpdater
	authCache        apiKeyAuthCacheInvalidator
	resolver         *ModelPricingResolver
	outputDir        string
	pollInterval     time.Duration
	pollTimeout      time.Duration
	backgroundRunner func(func())
}

type DramaVideoCreateResult struct {
	Task    *DramaVideoPublicTask
	PollURL string
}

type DramaVideoContent struct {
	Task        *DramaVideoTask
	Path        string
	ContentType string
	Size        int64
}

type dramaVideoCreatePayload struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt"`
	Seconds        *int            `json:"seconds,omitempty"`
	Resolution     string          `json:"resolution,omitempty"`
	AspectRatio    string          `json:"aspect_ratio,omitempty"`
	NegativePrompt string          `json:"negative_prompt,omitempty"`
	GenerateAudio  *bool           `json:"generate_audio,omitempty"`
	Seed           *int64          `json:"seed,omitempty"`
	References     json.RawMessage `json:"references,omitempty"`
}

type dramaVideoCreatePlan struct {
	body            []byte
	requestHash     string
	model           string          // 客户端原始模型名（展示用）
	upstreamModel   string          // 映射后的上游模型名（实际发给上游）
	resolution      string
	aspectRatio     string
	durationSeconds int
	holdAmount      float64
	actualCost      float64
	account         Account
	videoRate       float64
}

func NewDramaVideoService(
	tasks DramaVideoTaskRepository,
	accounts AccountRepository,
	client DramaVideoClient,
	billing *BillingService,
	usageBilling UsageBillingRepository,
	usageLogs UsageLogRepository,
	userGroupRates UserGroupRateRepository,
	apiKeyQuota APIKeyQuotaUpdater,
	authCache apiKeyAuthCacheInvalidator,
	resolver *ModelPricingResolver,
) *DramaVideoService {
	return &DramaVideoService{
		tasks:          tasks,
		accounts:       accounts,
		client:         client,
		billing:        billing,
		usageBilling:   usageBilling,
		usageLogs:      usageLogs,
		userGroupRates: userGroupRates,
		apiKeyQuota:    apiKeyQuota,
		authCache:      authCache,
		resolver:       resolver,
		outputDir:      dramaVideoOutputDir,
		pollInterval:   dramaVideoPollInterval,
		pollTimeout:    dramaVideoPollTimeout,
		backgroundRunner: func(fn func()) {
			go fn()
		},
	}
}

func (s *DramaVideoService) Create(ctx context.Context, apiKey *APIKey, rawBody []byte, inboundPath string) (*DramaVideoCreateResult, error) {
	if s == nil || s.tasks == nil || s.accounts == nil || s.client == nil || s.billing == nil || s.usageBilling == nil {
		return nil, infraerrors.ServiceUnavailable("DRAMA_VIDEO_UNAVAILABLE", "Drama video service is not available")
	}
	if apiKey == nil || apiKey.ID <= 0 || apiKey.UserID <= 0 || apiKey.GroupID == nil || apiKey.Group == nil {
		return nil, ErrDramaVideoForbidden
	}
	platform := apiKey.Group.Platform
	if resolvedPlatform, ok := ResolvedTargetPlatformFromContext(ctx); ok {
		platform = resolvedPlatform
	}
	if platform != PlatformDrama {
		return nil, infraerrors.NotFound("DRAMA_VIDEO_UNSUPPORTED_PLATFORM", "videos API is not supported for this platform")
	}
	plan, err := s.prepareCreate(ctx, apiKey, rawBody)
	if err != nil {
		return nil, err
	}
	taskID, err := NewDramaVideoTaskID()
	if err != nil {
		return nil, infraerrors.InternalServer("DRAMA_VIDEO_TASK_ID_FAILED", "failed to create Drama video task id").WithCause(err)
	}
	if err := s.reserveHold(ctx, apiKey, taskID, plan); err != nil {
		return nil, err
	}
	task, err := s.tasks.Create(ctx, CreateDramaVideoTaskParams{
		TaskID:          taskID,
		UserID:          apiKey.UserID,
		APIKeyID:        apiKey.ID,
		GroupID:         *apiKey.GroupID,
		AccountID:       plan.account.ID,
		Model:           plan.model,
		UpstreamModel:   plan.upstreamModel,
		Status:          DramaVideoStatusCreated,
		Progress:        0,
		RequestHash:     plan.requestHash,
		Resolution:      plan.resolution,
		AspectRatio:     plan.aspectRatio,
		DurationSeconds: plan.durationSeconds,
		HoldAmount:      plan.holdAmount,
	})
	if err != nil {
		_ = s.releaseHold(context.Background(), apiKey, taskID, plan.requestHash, plan.holdAmount)
		return nil, infraerrors.InternalServer("DRAMA_VIDEO_TASK_CREATE_FAILED", "failed to persist Drama video task").WithCause(err)
	}

	runner := s.backgroundRunner
	if runner == nil {
		runner = func(fn func()) { go fn() }
	}
	runner(func() {
		s.processTask(context.Background(), taskID, apiKey, plan, inboundPath)
	})

	return &DramaVideoCreateResult{Task: DramaVideoTaskToPublic(task), PollURL: "/v1/videos/" + taskID}, nil
}

func (s *DramaVideoService) Get(ctx context.Context, owner DramaVideoOwner, taskID string) (*DramaVideoPublicTask, error) {
	if s == nil || s.tasks == nil {
		return nil, infraerrors.ServiceUnavailable("DRAMA_VIDEO_UNAVAILABLE", "Drama video service is not available")
	}
	task, err := s.tasks.GetForOwner(ctx, owner, taskID)
	if err != nil {
		return nil, err
	}
	return DramaVideoTaskToPublic(task), nil
}

func (s *DramaVideoService) Content(ctx context.Context, owner DramaVideoOwner, taskID string) (*DramaVideoContent, error) {
	if s == nil || s.tasks == nil {
		return nil, infraerrors.ServiceUnavailable("DRAMA_VIDEO_UNAVAILABLE", "Drama video service is not available")
	}
	task, err := s.tasks.GetForOwner(ctx, owner, taskID)
	if err != nil {
		return nil, err
	}
	if normalizeDramaVideoStatus(task.Status) != DramaVideoStatusCompleted || strings.TrimSpace(task.OutputPath) == "" {
		return nil, ErrDramaVideoNotReady
	}
	info, err := os.Stat(task.OutputPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrDramaVideoContentMissing
		}
		return nil, infraerrors.InternalServer("DRAMA_VIDEO_CONTENT_STAT_FAILED", "failed to inspect video content").WithCause(err)
	}
	contentType := strings.TrimSpace(task.OutputMIME)
	if contentType == "" {
		contentType = "video/mp4"
	}
	return &DramaVideoContent{Task: task, Path: task.OutputPath, ContentType: contentType, Size: info.Size()}, nil
}

func (s *DramaVideoService) prepareCreate(ctx context.Context, apiKey *APIKey, rawBody []byte) (*dramaVideoCreatePlan, error) {
	if len(rawBody) == 0 {
		return nil, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REQUEST", "request body is empty")
	}
	payload, err := parseDramaVideoCreatePayload(rawBody)
	if err != nil {
		return nil, err
	}
	account, err := s.selectAccount(ctx, *apiKey.GroupID)
	if err != nil {
		return nil, err
	}

	// 应用账号的 model_mapping：客户端发的模型名 → 上游模型名
	requestedModel := payload.Model
	upstreamModel := payload.Model
	if mapping := account.GetModelMapping(); len(mapping) > 0 {
		if mapped, ok := mapping[payload.Model]; ok && strings.TrimSpace(mapped) != "" {
			upstreamModel = strings.TrimSpace(mapped)
		}
	}

	// 校验映射后的上游模型是否合法
	switch upstreamModel {
	case DramaVideoModelV2Fast, DramaVideoModelV2:
	default:
		return nil, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_MODEL",
			fmt.Sprintf("model %q is not supported; must be %s or %s (or a mapped alias)", requestedModel, DramaVideoModelV2Fast, DramaVideoModelV2))
	}

	// 用上游模型名重建请求体
	payload.Model = upstreamModel
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REQUEST", "invalid Drama video request")
	}

	effectiveGroupMultiplier, err := s.resolveEffectiveGroupMultiplier(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	videoRate := resolveVideoRateMultiplier(apiKey, effectiveGroupMultiplier)

	// 优先尝试渠道定价覆盖（与 Grok 视频计费逻辑一致）
	var cost *CostBreakdown
	if s.resolver != nil && apiKey.Group != nil {
		gid := apiKey.Group.ID
		resolved := s.resolver.Resolve(ctx, PricingInput{
			Model:   requestedModel,
			GroupID: &gid,
			Group:   apiKey.Group,
		})
		if resolved != nil && resolved.Source == PricingSourceChannel &&
			(resolved.Mode == BillingModePerRequest || resolved.Mode == BillingModeImage || resolved.Mode == BillingModeVideo) {
			units := float64(1) // per_request
			if resolved.Mode == BillingModeVideo {
				units = float64(*payload.Seconds) // 按秒
			}
			channelCost, calcErr := s.billing.CalculateCostUnified(CostInput{
				Ctx:            ctx,
				Model:          requestedModel,
				GroupID:        &gid,
				Group:          apiKey.Group,
				RequestCount:   1,
				UsageUnits:     units,
				SizeTier:       payload.Resolution,
				RateMultiplier: videoRate,
				Resolver:       s.resolver,
				Resolved:       resolved,
			})
			if calcErr == nil && channelCost != nil {
				channelCost.BillingMode = string(BillingModeVideo)
				cost = channelCost
			}
		}
	}
	// 渠道定价未命中，回退到分组视频价格
	if cost == nil {
		cost = s.billing.CalculateVideoCost(upstreamModel, payload.Resolution, 1, *payload.Seconds, apiKey.Group.VideoPriceConfig(), videoRate)
	}
	if cost == nil || cost.ActualCost < 0 {
		return nil, infraerrors.BadRequest("DRAMA_VIDEO_PRICING_FAILED", "failed to calculate Drama video cost")
	}
	return &dramaVideoCreatePlan{
		body:            body,
		requestHash:     hashDramaVideoPayload(body),
		model:           requestedModel,  // 保留客户端原始模型名（展示用）
		upstreamModel:   upstreamModel,   // 映射后的上游模型名
		resolution:      payload.Resolution,
		aspectRatio:     payload.AspectRatio,
		durationSeconds: *payload.Seconds,
		holdAmount:      cost.ActualCost,
		actualCost:      cost.ActualCost,
		account:         account,
		videoRate:       videoRate,
	}, nil
}

func parseDramaVideoCreatePayload(rawBody []byte) (dramaVideoCreatePayload, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &keys); err != nil {
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_JSON", "request body must be valid JSON")
	}
	forbidden := []string{"duration", "size", "images", "messages", "shots"}
	for _, key := range forbidden {
		if _, ok := keys[key]; ok {
			return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_UNSUPPORTED_FIELD", fmt.Sprintf("%s is not supported by Drama Video V2", key))
		}
	}
	if rawMeta, ok := keys["metadata"]; ok && len(rawMeta) > 0 {
		var metadata map[string]json.RawMessage
		if json.Unmarshal(rawMeta, &metadata) == nil {
			if _, ok := metadata["resolution"]; ok {
				return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_UNSUPPORTED_FIELD", "metadata.resolution is not supported by Drama Video V2")
			}
		}
	}
	var payload dramaVideoCreatePayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_JSON", "request body must be valid JSON")
	}
	payload.Model = strings.ToLower(strings.TrimSpace(payload.Model))
	if payload.Model == "" {
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_MODEL", "model is required")
	}
	payload.Prompt = strings.TrimSpace(payload.Prompt)
	promptLen := utf8.RuneCountInString(payload.Prompt)
	if promptLen < 1 || promptLen > 2500 {
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_PROMPT", "prompt must be 1-2500 characters")
	}
	if strings.TrimSpace(payload.NegativePrompt) != "" && utf8.RuneCountInString(strings.TrimSpace(payload.NegativePrompt)) > 2500 {
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_NEGATIVE_PROMPT", "negative_prompt must be at most 2500 characters")
	}
	payload.NegativePrompt = strings.TrimSpace(payload.NegativePrompt)
	if payload.Seconds == nil {
		v := 5
		payload.Seconds = &v
	}
	if _, ok := dramaVideoAllowedDurations[*payload.Seconds]; !ok {
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_SECONDS", "seconds must be 5, 10, or 15")
	}
	payload.Resolution = strings.ToLower(strings.TrimSpace(payload.Resolution))
	if payload.Resolution == "" {
		payload.Resolution = VideoBillingResolution720P
	}
	switch payload.Resolution {
	case VideoBillingResolution480P, VideoBillingResolution720P:
	case VideoBillingResolution1080P:
		if payload.Model == DramaVideoModelV2Fast {
			return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_RESOLUTION", "drama-video-v2-fast supports only 480p and 720p")
		}
	default:
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_RESOLUTION", "resolution must be 480p, 720p, or 1080p")
	}
	payload.AspectRatio = strings.TrimSpace(payload.AspectRatio)
	if payload.AspectRatio == "" {
		payload.AspectRatio = "16:9"
	}
	if _, ok := dramaVideoAllowedAspectRatios[payload.AspectRatio]; !ok {
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_ASPECT_RATIO", "aspect_ratio is not supported")
	}
	if payload.Seed != nil && (*payload.Seed < 0 || *payload.Seed > 2147483647) {
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_SEED", "seed must be between 0 and 2147483647")
	}
	if len(payload.References) > 0 && !json.Valid(payload.References) {
		return dramaVideoCreatePayload{}, infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REFERENCES", "references must be valid JSON")
	}
	return payload, nil
}

func (s *DramaVideoService) resolveEffectiveGroupMultiplier(ctx context.Context, apiKey *APIKey) (float64, error) {
	if apiKey == nil || apiKey.Group == nil {
		return 1, nil
	}
	multiplier := apiKey.Group.RateMultiplier
	if multiplier < 0 {
		multiplier = 0
	}
	if s != nil && s.userGroupRates != nil && apiKey.GroupID != nil {
		userRate, err := s.userGroupRates.GetByUserAndGroup(ctx, apiKey.UserID, *apiKey.GroupID)
		if err != nil {
			return 0, infraerrors.ServiceUnavailable("DRAMA_VIDEO_RATE_LOOKUP_FAILED", "failed to resolve Drama video rate multiplier").WithCause(err)
		}
		if userRate != nil {
			multiplier = *userRate
		}
	}
	if multiplier < 0 {
		return 0, nil
	}
	return multiplier, nil
}

func (s *DramaVideoService) selectAccount(ctx context.Context, groupID int64) (Account, error) {
	accounts, err := s.accounts.ListSchedulableByGroupIDAndPlatform(ctx, groupID, PlatformDrama)
	if err != nil {
		return Account{}, infraerrors.ServiceUnavailable("DRAMA_VIDEO_ACCOUNT_LOOKUP_FAILED", "failed to load Drama accounts").WithCause(err)
	}
	for _, account := range accounts {
		if strings.TrimSpace(account.GetCredential("api_key")) != "" || strings.TrimSpace(account.GetCredential("token")) != "" {
			return account, nil
		}
	}
	return Account{}, ErrDramaVideoNoAccount
}

func (s *DramaVideoService) reserveHold(ctx context.Context, apiKey *APIKey, taskID string, plan *dramaVideoCreatePlan) error {
	if plan == nil {
		return infraerrors.BadRequest("DRAMA_VIDEO_INVALID_REQUEST", "invalid Drama video request")
	}
	_, err := s.usageBilling.ReserveBatchImageBalance(ctx, &BatchImageBalanceHoldCommand{
		RequestID:          DramaVideoHoldRequestID(taskID),
		APIKeyID:           apiKey.ID,
		UserID:             apiKey.UserID,
		BatchID:            taskID,
		HoldAmount:         plan.holdAmount,
		RequestPayloadHash: plan.requestHash,
	})
	if err != nil {
		if errors.Is(err, ErrBatchImageInsufficientBalance) {
			return infraerrors.New(infraerrors.Code(ErrBatchImageInsufficientBalance), "DRAMA_VIDEO_INSUFFICIENT_BALANCE", infraerrors.Message(ErrBatchImageInsufficientBalance)).WithCause(err)
		}
		return infraerrors.New(infraerrors.Code(ErrBatchImageBillingHoldFailed), "DRAMA_VIDEO_BILLING_HOLD_FAILED", "Drama video balance hold failed").WithCause(err)
	}
	return nil
}

func (s *DramaVideoService) releaseHold(ctx context.Context, apiKey *APIKey, taskID, requestHash string, holdAmount float64) error {
	if s == nil || s.usageBilling == nil || apiKey == nil {
		return nil
	}
	_, err := s.usageBilling.ReleaseBatchImageBalance(ctx, &BatchImageBalanceHoldCommand{
		RequestID:          DramaVideoReleaseRequestID(taskID),
		APIKeyID:           apiKey.ID,
		UserID:             apiKey.UserID,
		BatchID:            taskID,
		HoldAmount:         holdAmount,
		RequestPayloadHash: requestHash,
	})
	return err
}

func (s *DramaVideoService) captureHold(ctx context.Context, apiKey *APIKey, taskID, requestHash string, holdAmount, actualAmount float64) error {
	if s == nil || s.usageBilling == nil || apiKey == nil {
		return nil
	}
	_, err := s.usageBilling.CaptureBatchImageBalance(ctx, &BatchImageBalanceHoldCommand{
		RequestID:          DramaVideoCaptureRequestID(taskID),
		APIKeyID:           apiKey.ID,
		UserID:             apiKey.UserID,
		BatchID:            taskID,
		HoldAmount:         holdAmount,
		ActualAmount:       actualAmount,
		RequestPayloadHash: requestHash,
	})
	return err
}

func (s *DramaVideoService) processTask(ctx context.Context, taskID string, apiKey *APIKey, plan *dramaVideoCreatePlan, inboundPath string) {
	if s == nil || plan == nil || apiKey == nil {
		return
	}
	now := time.Now()
	_, _ = s.tasks.UpdateStatus(ctx, DramaVideoTaskStatusUpdate{TaskID: taskID, Status: DramaVideoStatusSubmitting, SubmittedAt: &now})
	upstreamTask, err := s.client.CreateVideo(ctx, &plan.account, plan.body)
	if err != nil {
		s.failTask(ctx, apiKey, taskID, plan, "upstream_create_error", err.Error(), true)
		return
	}
	upstreamID := upstreamTask.PublicID()
	progress := clampDramaProgress(upstreamTask.Progress)
	status := NormalizeDramaUpstreamStatus(upstreamTask.Status)
	if status == "" || status == DramaVideoStatusUnknown {
		status = DramaVideoStatusQueued
	}
	_, _ = s.tasks.UpdateStatus(ctx, DramaVideoTaskStatusUpdate{TaskID: taskID, Status: status, Progress: &progress, UpstreamTaskID: &upstreamID})

	deadline := time.Now().Add(s.pollTimeout)
	if s.pollTimeout <= 0 {
		deadline = time.Now().Add(dramaVideoPollTimeout)
	}
	pollInterval := s.pollInterval
	if pollInterval <= 0 {
		pollInterval = dramaVideoPollInterval
	}
	current := upstreamTask
	for {
		if current != nil && NormalizeDramaUpstreamStatus(current.Status) == DramaVideoStatusCompleted {
			break
		}
		if current != nil {
			switch NormalizeDramaUpstreamStatus(current.Status) {
			case DramaVideoStatusFailed, DramaVideoStatusCanceled, DramaVideoStatusExpired:
				status := NormalizeDramaUpstreamStatus(current.Status)
				completedAt := time.Now()
				_, _ = s.tasks.UpdateStatus(ctx, DramaVideoTaskStatusUpdate{TaskID: taskID, Status: status, Progress: dramaPtrInt(clampDramaProgress(current.Progress)), Error: current.Error, CompletedAt: &completedAt})
				_ = s.releaseHold(ctx, apiKey, taskID, plan.requestHash, plan.holdAmount)
				return
			}
		}
		if time.Now().After(deadline) {
			s.failTask(ctx, apiKey, taskID, plan, "poll_timeout", "Drama video polling timed out", true)
			return
		}
		select {
		case <-ctx.Done():
			s.failTask(context.Background(), apiKey, taskID, plan, "context_cancelled", ctx.Err().Error(), true)
			return
		case <-time.After(pollInterval):
		}
		polled, err := s.client.GetVideo(ctx, &plan.account, upstreamID)
		if err != nil {
			slog.Warn("drama_video_poll_failed", "task_id", taskID, "upstream_task_id", upstreamID, "error", err)
			continue
		}
		current = polled
		status := NormalizeDramaUpstreamStatus(polled.Status)
		progress := clampDramaProgress(polled.Progress)
		_, _ = s.tasks.UpdateStatus(ctx, DramaVideoTaskStatusUpdate{TaskID: taskID, Status: status, Progress: &progress, Error: polled.Error})
	}

	_, _ = s.tasks.UpdateStatus(ctx, DramaVideoTaskStatusUpdate{TaskID: taskID, Status: DramaVideoStatusDownloading})
	download, err := s.client.DownloadVideo(ctx, &plan.account, upstreamID)
	if err != nil {
		s.failTask(ctx, apiKey, taskID, plan, "download_error", err.Error(), true)
		return
	}
	path, sha, err := s.writeOutput(taskID, download.Data)
	if err != nil {
		s.failTask(ctx, apiKey, taskID, plan, "store_error", err.Error(), true)
		return
	}
	if err := s.captureHold(ctx, apiKey, taskID, plan.requestHash, plan.holdAmount, plan.actualCost); err != nil {
		s.failTask(ctx, apiKey, taskID, plan, "billing_capture_error", err.Error(), false)
		return
	}
	completedAt := time.Now()
	_, err = s.tasks.MarkCompleted(ctx, DramaVideoTaskCompletionUpdate{
		TaskID:       taskID,
		ActualCost:   plan.actualCost,
		OutputPath:   path,
		OutputMIME:   download.ContentType,
		OutputBytes:  int64(len(download.Data)),
		OutputSHA256: sha,
		CompletedAt:  completedAt,
	})
	if err != nil {
		slog.Warn("drama_video_mark_completed_failed", "task_id", taskID, "error", err)
	}
	if err := s.recordUsage(ctx, apiKey, plan, taskID, inboundPath); err != nil {
		slog.Warn("drama_video_usage_log_failed", "task_id", taskID, "error", err)
	}
}

func (s *DramaVideoService) failTask(ctx context.Context, apiKey *APIKey, taskID string, plan *dramaVideoCreatePlan, errorType, message string, release bool) {
	completedAt := time.Now()
	_, _ = s.tasks.UpdateStatus(ctx, DramaVideoTaskStatusUpdate{
		TaskID:      taskID,
		Status:      DramaVideoStatusFailed,
		Error:       DramaVideoErrorJSON(errorType, message),
		CompletedAt: &completedAt,
	})
	if release {
		if err := s.releaseHold(context.Background(), apiKey, taskID, plan.requestHash, plan.holdAmount); err != nil {
			slog.Warn("drama_video_hold_release_failed", "task_id", taskID, "error", err)
		}
	}
}

func (s *DramaVideoService) writeOutput(taskID string, data []byte) (string, string, error) {
	outputDir := strings.TrimSpace(s.outputDir)
	if outputDir == "" {
		outputDir = dramaVideoOutputDir
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(data)
	sha := strings.ToLower(hex.EncodeToString(sum[:]))
	path := filepath.Join(outputDir, taskID+".mp4")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", "", err
	}
	return path, sha, nil
}

func (s *DramaVideoService) recordUsage(ctx context.Context, apiKey *APIKey, plan *dramaVideoCreatePlan, taskID, inboundPath string) error {
	if s == nil || s.usageLogs == nil || apiKey == nil || plan == nil {
		return nil
	}
	billingMode := string(BillingModeVideo)
	resolution := plan.resolution
	durationSeconds := plan.durationSeconds
	groupID := apiKey.GroupID
	log := &UsageLog{
		UserID:                apiKey.UserID,
		APIKeyID:              apiKey.ID,
		AccountID:             plan.account.ID,
		RequestID:             DramaVideoCaptureRequestID(taskID),
		Model:                 plan.model,
		RequestedModel:        plan.model,
		UpstreamModel:         optionalTrimmedStringPtr(plan.model),
		GroupID:               groupID,
		TotalCost:             plan.actualCost / nonZeroMultiplier(plan.videoRate),
		ActualCost:            plan.actualCost,
		RateMultiplier:        plan.videoRate,
		AccountRateMultiplier: plan.account.RateMultiplier,
		BillingType:           BillingTypeBalance,
		RequestType:           RequestTypeSync,
		Stream:                false,
		VideoCount:            1,
		VideoResolution:       &resolution,
		VideoDurationSeconds:  &durationSeconds,
		BillingMode:           &billingMode,
		InboundEndpoint:       optionalTrimmedStringPtr(firstNonEmpty(inboundPath, "/v1/videos")),
		UpstreamEndpoint:      optionalTrimmedStringPtr("/v1/videos"),
		CreatedAt:             time.Now(),
	}
	inserted, err := s.usageLogs.Create(ctx, log)
	if err != nil {
		return err
	}
	if inserted && s.apiKeyQuota != nil && plan.actualCost > 0 {
		if err := s.apiKeyQuota.UpdateQuotaUsed(ctx, apiKey.ID, plan.actualCost); err != nil {
			slog.Warn("drama_video_api_key_quota_update_failed", "task_id", taskID, "api_key_id", apiKey.ID, "error", err)
		}
		if err := s.apiKeyQuota.UpdateRateLimitUsage(ctx, apiKey.ID, plan.actualCost); err != nil {
			slog.Warn("drama_video_api_key_rate_limit_update_failed", "task_id", taskID, "api_key_id", apiKey.ID, "error", err)
		}
	}
	if inserted && s.authCache != nil && apiKey.Key != "" {
		s.authCache.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	}
	return nil
}

func hashDramaVideoPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func clampDramaProgress(progress int) int {
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}

func dramaPtrInt(v int) *int {
	return &v
}

func nonZeroMultiplier(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}

func DramaVideoReleaseRequestID(taskID string) string {
	return "drama_video_release:" + strings.TrimSpace(taskID)
}
