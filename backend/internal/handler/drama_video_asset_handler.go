package handler

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *DramaVideoHandler) WorkbenchMeta(c *gin.Context) {
	if h == nil || h.assets == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("DRAMA_VIDEO_UNAVAILABLE", "Drama video service is not available"))
		return
	}
	response.Success(c, h.assets.Meta(c.Request.Context()))
}

func (h *DramaVideoHandler) UploadAsset(c *gin.Context) {
	userID, ok := dramaVideoUserID(c)
	if !ok || h.assets == nil {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("DRAMA_VIDEO_ASSET_FILE", "file is required"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 64*1024*1024+1))
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("DRAMA_VIDEO_ASSET_FILE", "failed to read uploaded file"))
		return
	}
	name := ""
	if header != nil {
		name = header.Filename
	}
	got, err := h.assets.Upload(c.Request.Context(), userID, name, data)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, got)
}

func (h *DramaVideoHandler) ListAssets(c *gin.Context) {
	userID, ok := dramaVideoUserID(c)
	if !ok || h.assets == nil {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	got, err := h.assets.List(c.Request.Context(), userID, c.Query("kind"), limit, offset)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, got)
}

func (h *DramaVideoHandler) DeleteAsset(c *gin.Context) {
	userID, ok := dramaVideoUserID(c)
	if !ok || h.assets == nil {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}
	if err := h.assets.Delete(c.Request.Context(), userID, c.Param("id"), false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *DramaVideoHandler) AssetContent(c *gin.Context) {
	userID, ok := dramaVideoUserID(c)
	if !ok || h.assets == nil {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}
	asset, err := h.assets.Content(c.Request.Context(), userID, c.Param("id"), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	serveDramaVideoAssetFile(c, asset)
}

func (h *DramaVideoHandler) PublicAssetContent(c *gin.Context) {
	if h == nil || h.assets == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("DRAMA_VIDEO_UNAVAILABLE", "Drama video service is not available"))
		return
	}
	asset, err := h.assets.VerifySignedURL(c.Param("id"), c.Query("exp"), c.Query("sig"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	serveDramaVideoAssetFile(c, asset)
}

func (h *DramaVideoHandler) AdminListAssets(c *gin.Context) {
	if h == nil || h.assets == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("DRAMA_VIDEO_UNAVAILABLE", "Drama video service is not available"))
		return
	}
	userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	got, err := h.assets.AdminList(c.Request.Context(), userID, c.Query("kind"), limit, offset)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, got)
}

func (h *DramaVideoHandler) AdminDeleteAsset(c *gin.Context) {
	if h == nil || h.assets == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("DRAMA_VIDEO_UNAVAILABLE", "Drama video service is not available"))
		return
	}
	if err := h.assets.Delete(c.Request.Context(), 0, c.Param("id"), true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func dramaVideoUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		return 0, false
	}
	return subject.UserID, true
}

func serveDramaVideoAssetFile(c *gin.Context, asset *service.DramaVideoAsset) {
	f, err := os.Open(asset.StoragePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			response.ErrorFrom(c, service.ErrDramaVideoAssetNotFound)
			return
		}
		response.ErrorFrom(c, infraerrors.InternalServer("DRAMA_VIDEO_ASSET_OPEN_FAILED", "failed to open asset").WithCause(err))
		return
	}
	defer f.Close()
	if asset.MIME != "" {
		c.Header("Content-Type", asset.MIME)
	}
	c.Header("Cache-Control", "private, max-age=300")
	http.ServeContent(c.Writer, c.Request, asset.OriginalName, time.Time{}, f)
}
