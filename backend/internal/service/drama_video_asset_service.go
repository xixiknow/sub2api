package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const dramaVideoAssetIDPrefix = "vidasset_"

var dramaVideoAllowedMIME = map[string]string{
	"image/jpeg":      DramaVideoAssetKindImage,
	"image/png":       DramaVideoAssetKindImage,
	"image/webp":      DramaVideoAssetKindImage,
	"video/mp4":      DramaVideoAssetKindVideo,
	"video/quicktime": DramaVideoAssetKindVideo,
	"audio/mpeg":     DramaVideoAssetKindAudio,
	"audio/wav":      DramaVideoAssetKindAudio,
	"audio/x-wav":    DramaVideoAssetKindAudio,
}

// DramaVideoAssetModerator is an optional hook for future content review.
type DramaVideoAssetModerator interface {
	ModerateAsset(ctx context.Context, kind, mime string, data []byte) error
}

type DramaVideoAssetService struct {
	repo      DramaVideoAssetRepository
	settings  *SettingService
	cfg       *config.Config
	moderator DramaVideoAssetModerator
}

func NewDramaVideoAssetService(repo DramaVideoAssetRepository, settings *SettingService, cfg *config.Config) *DramaVideoAssetService {
	return &DramaVideoAssetService{repo: repo, settings: settings, cfg: cfg}
}

func (s *DramaVideoAssetService) Meta(ctx context.Context) DramaVideoWorkbenchMeta {
	meta := DramaVideoWorkbenchMeta{
		OutputRetentionDays: defaultDramaVideoOutputRetentionDays,
		AssetRetentionDays:  defaultDramaVideoAssetRetentionDays,
		AssetQuotaBytes:     defaultDramaVideoAssetQuotaBytesPerUser,
	}
	if s != nil && s.settings != nil {
		meta.OutputRetentionDays = s.settings.GetDramaVideoOutputRetentionDays(ctx)
		meta.AssetRetentionDays = s.settings.GetDramaVideoAssetRetentionDays(ctx)
		meta.AssetQuotaBytes = s.settings.GetDramaVideoAssetQuotaBytesPerUser(ctx)
	}
	return meta
}

func (s *DramaVideoAssetService) Upload(ctx context.Context, userID int64, filename string, data []byte) (*DramaVideoAssetPublic, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("DRAMA_VIDEO_UNAVAILABLE", "Drama video service is not available")
	}
	if userID <= 0 {
		return nil, ErrDramaVideoAssetForbidden
	}
	if len(data) == 0 {
		return nil, infraerrors.BadRequest("DRAMA_VIDEO_ASSET_EMPTY", "uploaded file is empty")
	}
	kind, mime, err := classifyDramaVideoAsset(filename, data)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > s.maxBytesForKind(kind) {
		return nil, infraerrors.BadRequest("DRAMA_VIDEO_ASSET_TOO_LARGE", "uploaded file exceeds the size limit")
	}
	if s.moderator != nil {
		if err := s.moderator.ModerateAsset(ctx, kind, mime, data); err != nil {
			return nil, err
		}
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	if existing, err := s.repo.GetActiveByUserSHA(ctx, userID, digest); err == nil && existing != nil {
		return DramaVideoAssetToPublic(existing), nil
	} else if err != nil && !errors.Is(err, ErrDramaVideoAssetNotFound) {
		return nil, err
	}
	quota := int64(0)
	if s.settings != nil {
		quota = s.settings.GetDramaVideoAssetQuotaBytesPerUser(ctx)
	}
	if quota > 0 {
		used, err := s.repo.SumBytesByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if used+int64(len(data)) > quota {
			return nil, infraerrors.BadRequest("DRAMA_VIDEO_ASSET_QUOTA", "asset storage quota exceeded")
		}
	}
	assetID, err := newDramaVideoAssetID()
	if err != nil {
		return nil, infraerrors.InternalServer("DRAMA_VIDEO_ASSET_ID_FAILED", "failed to create asset id").WithCause(err)
	}
	dir := filepath.Join(s.assetsDir(), strconv.FormatInt(userID, 10))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, infraerrors.InternalServer("DRAMA_VIDEO_ASSET_STORE_FAILED", "failed to store asset").WithCause(err)
	}
	path := filepath.Join(dir, assetID)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, infraerrors.InternalServer("DRAMA_VIDEO_ASSET_STORE_FAILED", "failed to store asset").WithCause(err)
	}
	asset, err := s.repo.Create(ctx, &DramaVideoAsset{
		AssetID:      assetID,
		UserID:       userID,
		Kind:         kind,
		MIME:         mime,
		Bytes:        int64(len(data)),
		SHA256:       digest,
		OriginalName: filepath.Base(filename),
		StoragePath:  path,
	})
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return DramaVideoAssetToPublic(asset), nil
}

func (s *DramaVideoAssetService) List(ctx context.Context, userID int64, kind string, limit, offset int) (*DramaVideoAssetList, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("DRAMA_VIDEO_UNAVAILABLE", "Drama video service is not available")
	}
	items, err := s.repo.ListByUser(ctx, userID, kind, limit, offset)
	if err != nil {
		return nil, err
	}
	used, err := s.repo.SumBytesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &DramaVideoAssetList{
		Items:     make([]*DramaVideoAssetPublic, 0, len(items)),
		UsedBytes: used,
	}
	meta := s.Meta(ctx)
	out.QuotaBytes = meta.AssetQuotaBytes
	out.RetentionDays = meta.AssetRetentionDays
	for _, item := range items {
		out.Items = append(out.Items, DramaVideoAssetToPublic(item))
	}
	return out, nil
}

func (s *DramaVideoAssetService) Delete(ctx context.Context, userID int64, assetID string, admin bool) error {
	asset, err := s.repo.GetByAssetID(ctx, assetID)
	if err != nil {
		return err
	}
	if asset.DeletedAt != nil {
		return ErrDramaVideoAssetNotFound
	}
	if !admin && asset.UserID != userID {
		return ErrDramaVideoAssetForbidden
	}
	if err := s.repo.SoftDelete(ctx, asset.AssetID, time.Now()); err != nil {
		return err
	}
	_ = os.Remove(asset.StoragePath)
	return nil
}

func (s *DramaVideoAssetService) Content(ctx context.Context, userID int64, assetID string, admin bool) (*DramaVideoAsset, error) {
	asset, err := s.repo.GetByAssetID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if asset.DeletedAt != nil {
		return nil, ErrDramaVideoAssetNotFound
	}
	if !admin && asset.UserID != userID {
		return nil, ErrDramaVideoAssetForbidden
	}
	return asset, nil
}

func (s *DramaVideoAssetService) AdminList(ctx context.Context, userID int64, kind string, limit, offset int) (*DramaVideoAssetAdminList, error) {
	items, total, err := s.repo.AdminList(ctx, userID, kind, limit, offset)
	if err != nil {
		return nil, err
	}
	out := &DramaVideoAssetAdminList{Items: make([]*DramaVideoAssetAdminItem, 0, len(items)), Total: total}
	for _, item := range items {
		pub := DramaVideoAssetToPublic(item)
		out.Items = append(out.Items, &DramaVideoAssetAdminItem{DramaVideoAssetPublic: *pub, UserID: item.UserID})
		out.UsedBytes += item.Bytes
	}
	return out, nil
}

func (s *DramaVideoAssetService) ResolveSources(ctx context.Context, userID int64, sources []string) (rewritten []string, assetIDs []string, err error) {
	rewritten = make([]string, len(sources))
	seen := make(map[string]struct{})
	for i, source := range sources {
		source = strings.TrimSpace(source)
		rewritten[i] = source
		if !strings.HasPrefix(strings.ToLower(source), DramaVideoAssetScheme) {
			continue
		}
		id := strings.TrimSpace(source[len(DramaVideoAssetScheme):])
		asset, getErr := s.repo.GetByAssetID(ctx, id)
		if getErr != nil {
			return nil, nil, getErr
		}
		if asset.DeletedAt != nil || asset.UserID != userID {
			return nil, nil, ErrDramaVideoAssetNotFound
		}
		signed, signErr := s.SignURL(asset.AssetID)
		if signErr != nil {
			return nil, nil, signErr
		}
		rewritten[i] = signed
		if _, ok := seen[asset.AssetID]; !ok {
			seen[asset.AssetID] = struct{}{}
			assetIDs = append(assetIDs, asset.AssetID)
		}
	}
	if len(assetIDs) > 0 {
		_ = s.repo.TouchLastUsed(ctx, assetIDs, time.Now())
	}
	return rewritten, assetIDs, nil
}

func (s *DramaVideoAssetService) SignURL(assetID string) (string, error) {
	base := s.publicBaseURL()
	if base == "" {
		return "", infraerrors.ServiceUnavailable("DRAMA_VIDEO_PUBLIC_BASE_URL", "drama_video.public_base_url or server.frontend_url must be set to serve asset links")
	}
	ttl := 120
	if s.cfg != nil && s.cfg.DramaVideo.AssetURLTTLMinutes > 0 {
		ttl = s.cfg.DramaVideo.AssetURLTTLMinutes
	}
	exp := time.Now().Add(time.Duration(ttl) * time.Minute).Unix()
	sig := s.sign(assetID, exp)
	u, err := url.Parse(strings.TrimRight(base, "/") + "/api/v1/public/video-assets/" + url.PathEscape(assetID))
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("exp", strconv.FormatInt(exp, 10))
	q.Set("sig", sig)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *DramaVideoAssetService) VerifySignedURL(assetID, expRaw, sig string) (*DramaVideoAsset, error) {
	exp, err := strconv.ParseInt(strings.TrimSpace(expRaw), 10, 64)
	if err != nil || exp < time.Now().Unix() {
		return nil, infraerrors.Unauthorized("DRAMA_VIDEO_ASSET_URL_EXPIRED", "asset URL is invalid or expired")
	}
	expected := s.sign(assetID, exp)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return nil, infraerrors.Unauthorized("DRAMA_VIDEO_ASSET_URL_INVALID", "asset URL signature is invalid")
	}
	asset, err := s.repo.GetByAssetID(context.Background(), assetID)
	if err != nil {
		return nil, err
	}
	if asset.DeletedAt != nil {
		return nil, ErrDramaVideoAssetNotFound
	}
	return asset, nil
}

func (s *DramaVideoAssetService) CleanupExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	if s == nil || s.repo == nil || s.settings == nil {
		return 0, nil
	}
	days := s.settings.GetDramaVideoAssetRetentionDays(ctx)
	if days <= 0 {
		return 0, nil
	}
	cutoff := now.AddDate(0, 0, -days)
	items, err := s.repo.ListExpired(ctx, cutoff, limit)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, item := range items {
		if err := s.repo.SoftDelete(ctx, item.AssetID, now); err != nil {
			continue
		}
		_ = os.Remove(item.StoragePath)
		n++
	}
	return n, nil
}

func (s *DramaVideoAssetService) sign(assetID string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(s.signingSecret()))
	_, _ = io.WriteString(mac, assetID)
	_, _ = io.WriteString(mac, "|")
	_, _ = io.WriteString(mac, strconv.FormatInt(exp, 10))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *DramaVideoAssetService) signingSecret() string {
	if s.cfg != nil {
		if secret := strings.TrimSpace(s.cfg.DramaVideo.AssetSigningSecret); secret != "" {
			return secret
		}
		if secret := strings.TrimSpace(s.cfg.JWT.Secret); secret != "" {
			return secret
		}
	}
	return "drama-video-asset-dev-secret"
}

func (s *DramaVideoAssetService) publicBaseURL() string {
	if s.cfg == nil {
		return ""
	}
	if u := strings.TrimSpace(s.cfg.DramaVideo.PublicBaseURL); u != "" {
		return strings.TrimRight(u, "/")
	}
	return strings.TrimRight(strings.TrimSpace(s.cfg.Server.FrontendURL), "/")
}

func (s *DramaVideoAssetService) assetsDir() string {
	if s.cfg != nil {
		return s.cfg.DramaVideo.AssetsDir()
	}
	return filepath.Join("data", "drama-video", "assets")
}

func (s *DramaVideoAssetService) maxBytesForKind(kind string) int64 {
	image, video, audio := int64(10*1024*1024), int64(40*1024*1024), int64(20*1024*1024)
	if s.cfg != nil {
		if s.cfg.DramaVideo.MaxAssetBytesImage > 0 {
			image = s.cfg.DramaVideo.MaxAssetBytesImage
		}
		if s.cfg.DramaVideo.MaxAssetBytesVideo > 0 {
			video = s.cfg.DramaVideo.MaxAssetBytesVideo
		}
		if s.cfg.DramaVideo.MaxAssetBytesAudio > 0 {
			audio = s.cfg.DramaVideo.MaxAssetBytesAudio
		}
	}
	switch kind {
	case DramaVideoAssetKindVideo:
		return video
	case DramaVideoAssetKindAudio:
		return audio
	default:
		return image
	}
}

func classifyDramaVideoAsset(filename string, data []byte) (kind, mime string, err error) {
	detected := http.DetectContentType(data)
	mime = strings.ToLower(strings.TrimSpace(detected))
	if mapped, ok := dramaVideoAllowedMIME[mime]; ok {
		return mapped, mime, nil
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return DramaVideoAssetKindImage, "image/jpeg", nil
	case ".png":
		return DramaVideoAssetKindImage, "image/png", nil
	case ".webp":
		return DramaVideoAssetKindImage, "image/webp", nil
	case ".mp4":
		return DramaVideoAssetKindVideo, "video/mp4", nil
	case ".mov":
		return DramaVideoAssetKindVideo, "video/quicktime", nil
	case ".mp3":
		return DramaVideoAssetKindAudio, "audio/mpeg", nil
	case ".wav":
		return DramaVideoAssetKindAudio, "audio/wav", nil
	default:
		return "", "", infraerrors.BadRequest("DRAMA_VIDEO_ASSET_TYPE", "unsupported asset type")
	}
}

func newDramaVideoAssetID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return dramaVideoAssetIDPrefix + hex.EncodeToString(buf), nil
}

func extractDramaVideoReferenceSources(payload dramaVideoCreatePayload) []string {
	out := make([]string, 0, len(payload.References))
	for _, ref := range payload.References {
		out = append(out, ref.Source)
	}
	return out
}
