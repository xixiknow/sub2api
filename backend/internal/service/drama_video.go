package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DramaVideoStatusCreated     = "created"
	DramaVideoStatusSubmitting  = "submitting"
	DramaVideoStatusQueued      = "queued"
	DramaVideoStatusInProgress  = "in_progress"
	DramaVideoStatusUnknown     = "unknown"
	DramaVideoStatusDownloading = "downloading"
	DramaVideoStatusCompleted   = "completed"
	DramaVideoStatusFailed      = "failed"
	DramaVideoStatusCanceled    = "canceled"
	DramaVideoStatusExpired     = "expired"

	DramaVideoObjectTask = "video"

	DramaVideoDefaultBaseURL = "https://drama.dafeiyangapi.top"

	DramaVideoModelV2Fast       = "drama-video-v2-fast"
	DramaVideoModelV2           = "drama-video-v2"
	DramaVideoModelSeedance20   = "seedance2.0"
	DramaVideoModelSeedance25   = "seedance2.5"
	DramaVideoModelSD25Vref720p = "sd2-5-vref-720p"

	dramaVideoTaskIDPrefix = "vidtask_"
)

var (
	ErrDramaVideoTaskNotFound   = infraerrors.New(http.StatusNotFound, "DRAMA_VIDEO_TASK_NOT_FOUND", "video task not found")
	ErrDramaVideoForbidden      = infraerrors.New(http.StatusForbidden, "DRAMA_VIDEO_FORBIDDEN", "video task does not belong to this API key")
	ErrDramaVideoInvalidRequest = infraerrors.New(http.StatusBadRequest, "DRAMA_VIDEO_INVALID_REQUEST", "invalid Drama video request")
	ErrDramaVideoNoAccount      = infraerrors.New(http.StatusServiceUnavailable, "DRAMA_VIDEO_NO_ACCOUNT", "no available Drama account")
	ErrDramaVideoUpstream       = infraerrors.New(http.StatusBadGateway, "DRAMA_VIDEO_UPSTREAM_ERROR", "Drama upstream request failed")
	ErrDramaVideoNotReady       = infraerrors.New(http.StatusConflict, "DRAMA_VIDEO_NOT_READY", "video task is not completed")
	ErrDramaVideoContentMissing = infraerrors.New(http.StatusGone, "DRAMA_VIDEO_CONTENT_MISSING", "video content is not available")
)

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
	RequestBodyPath string
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
	RequestBodyPath string
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

type DramaVideoClient interface {
	CreateVideo(ctx context.Context, account *Account, body []byte) (*DramaVideoUpstreamTask, error)
	GetVideo(ctx context.Context, account *Account, taskID string) (*DramaVideoUpstreamTask, error)
	DownloadVideo(ctx context.Context, account *Account, taskID string) (*DramaVideoDownload, error)
	CreateVideoUploadSession(ctx context.Context, account *Account, body []byte) (*SeedanceVideoUploadSession, error)
	CreateSeedanceVideoTask(ctx context.Context, account *Account, body []byte) (*SeedanceVideoTaskResponse, error)
	GetSeedanceVideoTask(ctx context.Context, account *Account, taskID string) (*SeedanceVideoTaskResponse, error)
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

func IsTerminalDramaVideoStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case DramaVideoStatusCompleted, DramaVideoStatusFailed, DramaVideoStatusCanceled, DramaVideoStatusExpired:
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
	return &DramaVideoPublicTask{
		ID:          task.TaskID,
		TaskID:      task.TaskID,
		Object:      DramaVideoObjectTask,
		Model:       task.Model,
		Status:      normalizeDramaVideoStatus(task.Status),
		Progress:    task.Progress,
		Error:       task.Error,
		CreatedAt:   task.CreatedAt.Unix(),
		CompletedAt: completedAt,
	}
}

func normalizeDramaVideoStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case DramaVideoStatusCreated, DramaVideoStatusSubmitting, DramaVideoStatusQueued:
		return DramaVideoStatusQueued
	case DramaVideoStatusInProgress, DramaVideoStatusDownloading:
		return DramaVideoStatusInProgress
	case DramaVideoStatusUnknown:
		return DramaVideoStatusUnknown
	case DramaVideoStatusCompleted:
		return DramaVideoStatusCompleted
	case DramaVideoStatusCanceled:
		return DramaVideoStatusCanceled
	case DramaVideoStatusExpired:
		return DramaVideoStatusExpired
	case DramaVideoStatusFailed:
		return DramaVideoStatusFailed
	default:
		return DramaVideoStatusUnknown
	}
}

func NormalizeDramaUpstreamStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case DramaVideoStatusQueued, DramaVideoStatusInProgress, DramaVideoStatusUnknown, DramaVideoStatusCompleted, DramaVideoStatusFailed, DramaVideoStatusCanceled, DramaVideoStatusExpired:
		return strings.ToLower(strings.TrimSpace(status))
	case "cancelled":
		return DramaVideoStatusCanceled
	case "running", "processing":
		return DramaVideoStatusInProgress
	default:
		return DramaVideoStatusUnknown
	}
}

func DramaVideoErrorJSON(errorType, message string) json.RawMessage {
	errorType = strings.TrimSpace(errorType)
	if errorType == "" {
		errorType = "api_error"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "Drama video task failed"
	}
	data, _ := json.Marshal(map[string]string{"type": errorType, "message": message})
	return data
}

func DramaVideoHoldRequestID(taskID string) string {
	return BatchImageHoldRequestID(taskID)
}

func DramaVideoCaptureRequestID(taskID string) string {
	return "drama_video_capture:" + strings.TrimSpace(taskID)
}
