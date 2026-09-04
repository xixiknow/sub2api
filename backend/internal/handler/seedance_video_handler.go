package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type SeedanceVideoHandler struct {
	service *service.SeedanceVideoService
}

func NewSeedanceVideoHandler(service *service.SeedanceVideoService) *SeedanceVideoHandler {
	return &SeedanceVideoHandler{service: service}
}

func (h *SeedanceVideoHandler) UploadSession(c *gin.Context) {
	if h == nil || h.service == nil {
		dramaVideoError(c, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_UNAVAILABLE", "Seedance video service is not available"))
		return
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		dramaVideoError(c, infraerrors.Unauthorized("API_KEY_REQUIRED", "API key is required"))
		return
	}
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		dramaVideoError(c, infraerrors.BadRequest("SEEDANCE_VIDEO_BODY_READ_FAILED", "failed to read request body").WithCause(err))
		return
	}
	got, err := h.service.CreateUploadSession(c.Request.Context(), apiKey, body)
	if err != nil {
		dramaVideoError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, got)
}

func (h *SeedanceVideoHandler) CreateTask(c *gin.Context) {
	if h == nil || h.service == nil {
		dramaVideoError(c, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_UNAVAILABLE", "Seedance video service is not available"))
		return
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		dramaVideoError(c, infraerrors.Unauthorized("API_KEY_REQUIRED", "API key is required"))
		return
	}
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		dramaVideoError(c, infraerrors.BadRequest("SEEDANCE_VIDEO_BODY_READ_FAILED", "failed to read request body").WithCause(err))
		return
	}
	scope := "seedance.video.task.create"
	payload := body
	execute := func(ctx context.Context) (map[string]any, error) {
		got, err := h.service.CreateTask(ctx, apiKey, payload)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status_code": http.StatusAccepted, "data": got}, nil
	}
	c.Header("Cache-Control", "no-store")
	executeSeedanceIdempotentJSON(c, scope, payload, service.DefaultWriteIdempotencyTTL(), execute)
}

func (h *SeedanceVideoHandler) GetTask(c *gin.Context) {
	if h == nil || h.service == nil {
		dramaVideoError(c, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_UNAVAILABLE", "Seedance video service is not available"))
		return
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		dramaVideoError(c, infraerrors.Unauthorized("API_KEY_REQUIRED", "API key is required"))
		return
	}
	got, err := h.service.GetTask(c.Request.Context(), apiKey, c.Param("task_id"))
	if err != nil {
		dramaVideoError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, got)
}

func (h *SeedanceVideoHandler) Content(c *gin.Context) {
	if h == nil || h.service == nil {
		dramaVideoError(c, infraerrors.ServiceUnavailable("SEEDANCE_VIDEO_UNAVAILABLE", "Seedance video service is not available"))
		return
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		dramaVideoError(c, infraerrors.Unauthorized("API_KEY_REQUIRED", "API key is required"))
		return
	}
	got, err := h.service.Content(c.Request.Context(), apiKey, c.Param("task_id"), c.Request.Method)
	if err != nil {
		dramaVideoError(c, err)
		return
	}
	defer got.Body.Close()
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("Content-Type", got.ContentType)
	if got.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(got.Size, 10))
	}
	c.Status(http.StatusOK)
	if c.Request.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(c.Writer, got.Body)
}

func executeSeedanceIdempotentJSON(
	c *gin.Context,
	scope string,
	payload any,
	ttl time.Duration,
	execute func(context.Context) (map[string]any, error),
) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		dramaVideoError(c, service.ErrIdempotencyKeyRequired)
		return
	}
	coordinator := service.DefaultIdempotencyCoordinator()
	if coordinator == nil {
		result, err := execute(c.Request.Context())
		if err != nil {
			dramaVideoError(c, err)
			return
		}
		statusCode, data, err := seedanceJSONResponseFromMap(result)
		if err != nil {
			dramaVideoError(c, err)
			return
		}
		c.JSON(statusCode, data)
		return
	}

	actorScope := "user:0"
	if apiKey, ok := middleware.GetAPIKeyFromContext(c); ok && apiKey != nil && apiKey.UserID > 0 {
		actorScope = fmt.Sprintf("user:%d", apiKey.UserID)
	}

	result, err := coordinator.Execute(c.Request.Context(), service.IdempotencyExecuteOptions{
		Scope:          scope,
		ActorScope:     actorScope,
		Method:         c.Request.Method,
		Route:          c.FullPath(),
		IdempotencyKey: key,
		Payload:        payload,
		RequireKey:     true,
		TTL:            ttl,
	}, func(ctx context.Context) (any, error) {
		return execute(ctx)
	})
	if err != nil {
		if retryAfter := service.RetryAfterSecondsFromError(err); retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		dramaVideoError(c, err)
		return
	}
	if result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	statusCode, data, err := seedanceJSONResponseFromAny(result.Data)
	if err != nil {
		dramaVideoError(c, err)
		return
	}
	c.JSON(statusCode, data)
}

func seedanceJSONResponseFromAny(value any) (int, any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return seedanceJSONResponseFromMap(typed)
	default:
		return 0, nil, infraerrors.InternalServer("SEEDANCE_VIDEO_IDEMPOTENCY_INVALID_RESPONSE", "idempotent response has unexpected shape")
	}
}

func seedanceJSONResponseFromMap(value map[string]any) (int, any, error) {
	if value == nil {
		return 0, nil, infraerrors.InternalServer("SEEDANCE_VIDEO_IDEMPOTENCY_INVALID_RESPONSE", "idempotent response has unexpected shape")
	}
	statusCode := http.StatusAccepted
	if raw, ok := value["status_code"]; ok {
		switch typed := raw.(type) {
		case float64:
			statusCode = int(typed)
		case int:
			statusCode = typed
		case int32:
			statusCode = int(typed)
		case int64:
			statusCode = int(typed)
		}
	}
	data, ok := value["data"]
	if !ok {
		return 0, nil, infraerrors.InternalServer("SEEDANCE_VIDEO_IDEMPOTENCY_INVALID_RESPONSE", "idempotent response is missing data")
	}
	if statusCode <= 0 {
		statusCode = http.StatusAccepted
	}
	return statusCode, data, nil
}
