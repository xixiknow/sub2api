package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DramaVideoStatusQueued      = "queued"
	DramaVideoStatusInProgress  = "in_progress"
	DramaVideoStatusCompleted   = "completed"
	DramaVideoStatusFailed      = "failed"
	DramaVideoStatusCanceled    = "canceled"

	DramaVideoObjectTask       = "video"
	DramaVideoDefaultBaseURL   = "https://drama.dafeiyangapi.top"
	dramaVideoTaskIDPrefix     = "vidtask_"
	DramaVideoCreatePathVideos = "/v1/videos"
	DramaVideoCreatePathGens   = "/v1/video/generations"

	DramaVideoBillingPerSecond = "per_second"
	DramaVideoBillingPerClip   = "per_clip"
)

const (
	DramaFamilyMinimaxH3     = "minimax-h3"
	DramaFamilySeedance20A   = "seedance2.0-A"
	DramaFamilySeedance20FA  = "seedance2.0-fast-A"
	DramaFamilySeedance20MA  = "seedance2.0-Mini-A"
	DramaFamilySeedance20B   = "seedance2.0-B"
	DramaFamilySeedance20FB  = "seedance2.0-fast-B"
	DramaFamilySeedance20C   = "seedance-2.0-C"
	DramaFamilySeedance20E   = "seedance2.0-E"
	DramaFamilySeedance20F   = "seedance2.0-F"
	DramaFamilySeedance20FF  = "seedance2.0-fast-F"
	DramaFamilySeedance25A   = "seedance2.5-A"
	DramaFamilySeedance25B   = "seedance-2.5-B"
)

var (
	ErrDramaVideoTaskNotFound   = infraerrors.New(http.StatusNotFound, "DRAMA_VIDEO_TASK_NOT_FOUND", "video task not found")
	ErrDramaVideoForbidden      = infraerrors.New(http.StatusForbidden, "DRAMA_VIDEO_FORBIDDEN", "video task does not belong to this API key")
	ErrDramaVideoNoAccount      = infraerrors.New(http.StatusServiceUnavailable, "DRAMA_VIDEO_NO_ACCOUNT", "no available Drama account")
	ErrDramaVideoUpstream       = infraerrors.New(http.StatusBadGateway, "DRAMA_VIDEO_UPSTREAM_ERROR", "Drama upstream request failed")
	ErrDramaVideoNotReady       = infraerrors.New(http.StatusConflict, "DRAMA_VIDEO_NOT_READY", "video task is not completed")
	ErrDramaVideoContentMissing = infraerrors.New(http.StatusGone, "DRAMA_VIDEO_CONTENT_MISSING", "video content is not available")
)

// DramaVideoPublicFamilies is the operator-facing catalog. Channel and group
// pricing keys must use these names, never resolution-suffixed upstream IDs.
func DramaVideoPublicFamilies() []string {
	return []string{
		DramaFamilyMinimaxH3,
		DramaFamilySeedance20A,
		DramaFamilySeedance20FA,
		DramaFamilySeedance20MA,
		DramaFamilySeedance20B,
		DramaFamilySeedance20FB,
		DramaFamilySeedance20C,
		DramaFamilySeedance20E,
		DramaFamilySeedance20F,
		DramaFamilySeedance20FF,
		DramaFamilySeedance25A,
		DramaFamilySeedance25B,
	}
}

type DramaVideoResolvedModel struct {
	RequestedModel string
	Family         string
	Resolution     string
	UpstreamModel  string
	CreatePath     string
	BillingUnit    string
}

type DramaVideoTask struct {
	ID              int64
	TaskID          string
	UpstreamTaskID  string
	UserID          int64
	APIKeyID        int64
	GroupID         int64
	AccountID       *int64
	Model           string
	UpstreamModel   string
	Status          string
	Progress        int
	RequestHash     string
	Resolution      string
	AspectRatio     string
	DurationSeconds int
	HoldAmount      float64
	ActualCost      *float64
	OutputPath      string
	OutputMIME      string
	OutputBytes     int64
	OutputSHA256    string
	Error           json.RawMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
	SubmittedAt     *time.Time
	CompletedAt     *time.Time
}

type DramaVideoPublicTask struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"task_id"`
	Object      string          `json:"object"`
	Model       string          `json:"model"`
	Status      string          `json:"status"`
	Progress    int             `json:"progress"`
	Error       json.RawMessage `json:"error,omitempty"`
	CreatedAt   int64           `json:"created_at"`
	CompletedAt *int64          `json:"completed_at,omitempty"`
	Seconds     string          `json:"seconds,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
}

type DramaVideoCreateResult struct {
	Task    *DramaVideoPublicTask
	PollURL string
}

type DramaVideoOwner struct {
	UserID   int64
	APIKeyID int64
}

type CreateDramaVideoTaskParams struct {
	TaskID          string
	UserID          int64
	APIKeyID        int64
	GroupID         int64
	AccountID       int64
	Model           string
	UpstreamModel   string
	Status          string
	Progress        int
	RequestHash     string
	Resolution      string
	AspectRatio     string
	DurationSeconds int
	HoldAmount      float64
}

type DramaVideoTaskStatusUpdate struct {
	TaskID         string
	Status         string
	Progress       *int
	UpstreamTaskID *string
	Error          json.RawMessage
	SubmittedAt    *time.Time
	CompletedAt    *time.Time
}

type DramaVideoTaskCompletionUpdate struct {
	TaskID       string
	ActualCost   float64
	OutputPath   string
	OutputMIME   string
	OutputBytes  int64
	OutputSHA256 string
	CompletedAt  time.Time
}

type DramaVideoTaskRepository interface {
	Create(ctx context.Context, params CreateDramaVideoTaskParams) (*DramaVideoTask, error)
	GetByTaskID(ctx context.Context, taskID string) (*DramaVideoTask, error)
	GetForOwner(ctx context.Context, owner DramaVideoOwner, taskID string) (*DramaVideoTask, error)
	UpdateStatus(ctx context.Context, update DramaVideoTaskStatusUpdate) (*DramaVideoTask, error)
	MarkCompleted(ctx context.Context, update DramaVideoTaskCompletionUpdate) (*DramaVideoTask, error)
}

type DramaVideoAccountSelector interface {
	ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error)
}

type DramaVideoBalanceHolder interface {
	ReserveBatchImageBalance(ctx context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error)
	CaptureBatchImageBalance(ctx context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error)
	ReleaseBatchImageBalance(ctx context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error)
}

type DramaVideoClient interface {
	CreateVideo(ctx context.Context, account *Account, path string, body []byte) (*DramaVideoUpstreamTask, error)
	GetVideo(ctx context.Context, account *Account, path, taskID string) (*DramaVideoUpstreamTask, error)
	DownloadVideo(ctx context.Context, account *Account, taskID string) (*DramaVideoDownload, error)
}

type DramaVideoUpstreamTask struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"task_id"`
	Object      string          `json:"object"`
	Model       string          `json:"model"`
	Status      string          `json:"status"`
	Progress    int             `json:"progress"`
	Error       json.RawMessage `json:"error,omitempty"`
	CreatedAt   int64           `json:"created_at,omitempty"`
	CompletedAt *int64          `json:"completed_at,omitempty"`
}

func (t *DramaVideoUpstreamTask) PublicID() string {
	if t == nil {
		return ""
	}
	if id := strings.TrimSpace(t.ID); id != "" {
		return id
	}
	return strings.TrimSpace(t.TaskID)
}

type DramaVideoDownload struct {
	ContentType string
	Data        []byte
}

func NewDramaVideoTaskID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return dramaVideoTaskIDPrefix + hex.EncodeToString(buf), nil
}

func IsDramaVideoTaskID(taskID string) bool {
	return strings.HasPrefix(strings.TrimSpace(taskID), dramaVideoTaskIDPrefix)
}

func DramaVideoHoldRequestID(taskID string) string {
	return "drama_video_hold:" + strings.TrimSpace(taskID)
}

func DramaVideoCaptureRequestID(taskID string) string {
	return "drama_video_capture:" + strings.TrimSpace(taskID)
}

func DramaVideoReleaseRequestID(taskID string) string {
	return "drama_video_release:" + strings.TrimSpace(taskID)
}

func NormalizeDramaUpstreamStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "created", "queued", "pending":
		return DramaVideoStatusQueued
	case "submitting", "submitted", "processing", "running", "in_progress", "downloading":
		return DramaVideoStatusInProgress
	case "completed", "succeeded", "success", "done":
		return DramaVideoStatusCompleted
	case "canceled", "cancelled":
		return DramaVideoStatusCanceled
	case "failed", "error", "expired":
		return DramaVideoStatusFailed
	default:
		return DramaVideoStatusInProgress
	}
}

type DramaVideoContent struct {
	Task        *DramaVideoTask
	Path        string
	ContentType string
	Size        int64
}

func IsTerminalDramaVideoStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case DramaVideoStatusCompleted, DramaVideoStatusFailed, DramaVideoStatusCanceled:
		return true
	default:
		return false
	}
}

func DramaVideoTaskToPublic(task *DramaVideoTask) *DramaVideoPublicTask {
	if task == nil {
		return nil
	}
	var completedAt *int64
	if task.CompletedAt != nil {
		v := task.CompletedAt.Unix()
		completedAt = &v
	}
	out := &DramaVideoPublicTask{
		ID:          task.TaskID,
		TaskID:      task.TaskID,
		Object:      DramaVideoObjectTask,
		Model:       task.Model,
		Status:      task.Status,
		Progress:    task.Progress,
		Error:       task.Error,
		CreatedAt:   task.CreatedAt.Unix(),
		CompletedAt: completedAt,
	}
	if task.DurationSeconds > 0 {
		out.Seconds = strconv.Itoa(task.DurationSeconds)
	}
	if task.AspectRatio != "" || task.Resolution != "" {
		out.Metadata = map[string]any{}
		if task.AspectRatio != "" {
			out.Metadata["aspect_ratio"] = task.AspectRatio
		}
		if task.Resolution != "" {
			out.Metadata["resolution"] = task.Resolution
		}
	}
	return out
}
