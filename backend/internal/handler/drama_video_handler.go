package handler

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DramaVideoHandler struct {
	service *service.DramaVideoService
}

func NewDramaVideoHandler(service *service.DramaVideoService) *DramaVideoHandler {
	return &DramaVideoHandler{service: service}
}

func (h *DramaVideoHandler) Create(c *gin.Context) {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		dramaVideoError(c, infraerrors.Unauthorized("API_KEY_REQUIRED", "API key is required"))
		return
	}
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		dramaVideoError(c, infraerrors.BadRequest("DRAMA_VIDEO_BODY_READ_FAILED", "failed to read request body").WithCause(err))
		return
	}
	got, err := h.service.Create(c.Request.Context(), apiKey, body, c.Request.URL.Path)
	if err != nil {
		dramaVideoError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	if got != nil && got.PollURL != "" {
		c.Header("Location", got.PollURL)
		c.Header("Retry-After", "5")
	}
	c.JSON(http.StatusAccepted, got.Task)
}

func (h *DramaVideoHandler) Get(c *gin.Context) {
	owner, ok := dramaVideoOwnerFromContext(c)
	if !ok {
		dramaVideoError(c, infraerrors.Unauthorized("API_KEY_REQUIRED", "API key is required"))
		return
	}
	got, err := h.service.Get(c.Request.Context(), owner, c.Param("request_id"))
	if err != nil {
		dramaVideoError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, got)
}

func (h *DramaVideoHandler) Content(c *gin.Context) {
	owner, ok := dramaVideoOwnerFromContext(c)
	if !ok {
		dramaVideoError(c, infraerrors.Unauthorized("API_KEY_REQUIRED", "API key is required"))
		return
	}
	content, err := h.service.Content(c.Request.Context(), owner, c.Param("request_id"))
	if err != nil {
		dramaVideoError(c, err)
		return
	}
	f, err := os.Open(content.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			dramaVideoError(c, service.ErrDramaVideoContentMissing)
			return
		}
		dramaVideoError(c, infraerrors.InternalServer("DRAMA_VIDEO_CONTENT_OPEN_FAILED", "failed to open video content").WithCause(err))
		return
	}
	defer f.Close()
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("Content-Type", content.ContentType)
	if content.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(content.Size, 10))
	}
	c.Status(http.StatusOK)
	if c.Request.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(c.Writer, f)
}

func dramaVideoOwnerFromContext(c *gin.Context) (service.DramaVideoOwner, bool) {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		return service.DramaVideoOwner{}, false
	}
	return service.DramaVideoOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID}, true
}

func dramaVideoError(c *gin.Context, err error) {
	setDramaVideoUpstreamContext(c, err)
	status := infraerrors.Code(err)
	code := infraerrors.Reason(err)
	message := infraerrors.Message(err)
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	if strings.TrimSpace(code) == "" {
		code = "DRAMA_VIDEO_ERROR"
	}
	if strings.TrimSpace(message) == "" {
		message = "Drama video request failed"
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "code": code, "message": message}})
}

type dramaVideoUpstreamErrorCarrier interface {
	error
	UpstreamStatusCode() int
	UpstreamErrorMessage() string
	UpstreamErrorDetail() string
}

func setDramaVideoUpstreamContext(c *gin.Context, err error) {
	if c == nil || err == nil {
		return
	}
	var carrier dramaVideoUpstreamErrorCarrier
	if !errors.As(err, &carrier) {
		return
	}
	service.SetOpsUpstreamError(c, carrier.UpstreamStatusCode(), carrier.UpstreamErrorMessage(), carrier.UpstreamErrorDetail())
}
