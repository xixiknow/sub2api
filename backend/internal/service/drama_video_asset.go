package service

import (
	"context"
	"time"
)

const (
	DramaVideoAssetKindImage = "image"
	DramaVideoAssetKindVideo = "video"
	DramaVideoAssetKindAudio = "audio"
	DramaVideoAssetScheme    = "asset://"
)

type DramaVideoAsset struct {
	ID           int64
	AssetID      string
	UserID       int64
	Kind         string
	MIME         string
	Bytes        int64
	SHA256       string
	OriginalName string
	StoragePath  string
	CreatedAt    time.Time
	LastUsedAt   time.Time
	DeletedAt    *time.Time
}

type DramaVideoAssetPublic struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	MIME         string `json:"mime"`
	Bytes        int64  `json:"bytes"`
	OriginalName string `json:"original_name"`
	CreatedAt    int64  `json:"created_at"`
	LastUsedAt   int64  `json:"last_used_at"`
	PreviewPath  string `json:"preview_path"`
}

type DramaVideoAssetList struct {
	Items      []*DramaVideoAssetPublic `json:"items"`
	UsedBytes  int64                    `json:"used_bytes"`
	QuotaBytes int64                    `json:"quota_bytes"`
	RetentionDays int                   `json:"retention_days"`
}

type DramaVideoAssetAdminItem struct {
	DramaVideoAssetPublic
	UserID int64 `json:"user_id"`
}

type DramaVideoAssetAdminList struct {
	Items      []*DramaVideoAssetAdminItem `json:"items"`
	Total      int                         `json:"total"`
	UsedBytes  int64                       `json:"used_bytes"`
}

type DramaVideoWorkbenchMeta struct {
	OutputRetentionDays int   `json:"output_retention_days"`
	AssetRetentionDays  int   `json:"asset_retention_days"`
	AssetQuotaBytes     int64 `json:"asset_quota_bytes"`
}

type DramaVideoAssetRepository interface {
	Create(ctx context.Context, asset *DramaVideoAsset) (*DramaVideoAsset, error)
	GetByAssetID(ctx context.Context, assetID string) (*DramaVideoAsset, error)
	GetActiveByUserSHA(ctx context.Context, userID int64, sha256 string) (*DramaVideoAsset, error)
	ListByUser(ctx context.Context, userID int64, kind string, limit, offset int) ([]*DramaVideoAsset, error)
	SumBytesByUser(ctx context.Context, userID int64) (int64, error)
	SoftDelete(ctx context.Context, assetID string, deletedAt time.Time) error
	TouchLastUsed(ctx context.Context, assetIDs []string, usedAt time.Time) error
	ListExpired(ctx context.Context, cutoff time.Time, limit int) ([]*DramaVideoAsset, error)
	AdminList(ctx context.Context, userID int64, kind string, limit, offset int) ([]*DramaVideoAsset, int, error)
}

func DramaVideoAssetToPublic(asset *DramaVideoAsset) *DramaVideoAssetPublic {
	if asset == nil {
		return nil
	}
	return &DramaVideoAssetPublic{
		ID:           asset.AssetID,
		Kind:         asset.Kind,
		MIME:         asset.MIME,
		Bytes:        asset.Bytes,
		OriginalName: asset.OriginalName,
		CreatedAt:    asset.CreatedAt.Unix(),
		LastUsedAt:   asset.LastUsedAt.Unix(),
		PreviewPath:  "/api/v1/video-assets/" + asset.AssetID + "/content",
	}
}
