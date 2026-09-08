package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type memoryDramaAssets struct {
	mu     sync.Mutex
	byID   map[string]*DramaVideoAsset
	nextID int64
}

func (m *memoryDramaAssets) Create(_ context.Context, asset *DramaVideoAsset) (*DramaVideoAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byID == nil {
		m.byID = map[string]*DramaVideoAsset{}
	}
	m.nextID++
	cloned := *asset
	cloned.ID = m.nextID
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = time.Now()
	}
	if cloned.LastUsedAt.IsZero() {
		cloned.LastUsedAt = cloned.CreatedAt
	}
	m.byID[cloned.AssetID] = &cloned
	return &cloned, nil
}

func (m *memoryDramaAssets) GetByAssetID(_ context.Context, assetID string) (*DramaVideoAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	asset := m.byID[assetID]
	if asset == nil {
		return nil, ErrDramaVideoAssetNotFound
	}
	cloned := *asset
	return &cloned, nil
}

func (m *memoryDramaAssets) GetActiveByUserSHA(_ context.Context, userID int64, sha256 string) (*DramaVideoAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, asset := range m.byID {
		if asset.UserID == userID && asset.SHA256 == sha256 && asset.DeletedAt == nil {
			cloned := *asset
			return &cloned, nil
		}
	}
	return nil, ErrDramaVideoAssetNotFound
}

func (m *memoryDramaAssets) ListByUser(_ context.Context, userID int64, kind string, _, _ int) ([]*DramaVideoAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*DramaVideoAsset{}
	for _, asset := range m.byID {
		if asset.UserID != userID || asset.DeletedAt != nil {
			continue
		}
		if kind != "" && asset.Kind != kind {
			continue
		}
		cloned := *asset
		out = append(out, &cloned)
	}
	return out, nil
}

func (m *memoryDramaAssets) SumBytesByUser(_ context.Context, userID int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sum int64
	for _, asset := range m.byID {
		if asset.UserID == userID && asset.DeletedAt == nil {
			sum += asset.Bytes
		}
	}
	return sum, nil
}

func (m *memoryDramaAssets) SoftDelete(_ context.Context, assetID string, deletedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	asset := m.byID[assetID]
	if asset == nil {
		return ErrDramaVideoAssetNotFound
	}
	asset.DeletedAt = &deletedAt
	return nil
}

func (m *memoryDramaAssets) TouchLastUsed(_ context.Context, assetIDs []string, usedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range assetIDs {
		if asset := m.byID[id]; asset != nil {
			asset.LastUsedAt = usedAt
		}
	}
	return nil
}

func (m *memoryDramaAssets) ListExpired(_ context.Context, cutoff time.Time, _ int) ([]*DramaVideoAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*DramaVideoAsset{}
	for _, asset := range m.byID {
		if asset.DeletedAt == nil && !asset.LastUsedAt.After(cutoff) {
			cloned := *asset
			out = append(out, &cloned)
		}
	}
	return out, nil
}

func (m *memoryDramaAssets) AdminList(_ context.Context, userID int64, kind string, _, _ int) ([]*DramaVideoAsset, int, error) {
	items, err := m.ListByUser(context.Background(), userID, kind, 0, 0)
	if userID == 0 {
		m.mu.Lock()
		defer m.mu.Unlock()
		items = nil
		for _, asset := range m.byID {
			if asset.DeletedAt != nil {
				continue
			}
			if kind != "" && asset.Kind != kind {
				continue
			}
			cloned := *asset
			items = append(items, &cloned)
		}
	}
	return items, len(items), err
}

func TestClassifyDramaVideoAsset(t *testing.T) {
	kind, mime, err := classifyDramaVideoAsset("ref.png", []byte("\x89PNG\r\n\x1a\n"))
	require.NoError(t, err)
	require.Equal(t, DramaVideoAssetKindImage, kind)
	require.Equal(t, "image/png", mime)

	kind, mime, err = classifyDramaVideoAsset("clip.mp4", []byte("not-a-real-mp4"))
	require.NoError(t, err)
	require.Equal(t, DramaVideoAssetKindVideo, kind)
	require.Equal(t, "video/mp4", mime)

	_, _, err = classifyDramaVideoAsset("notes.txt", []byte("hello"))
	require.Error(t, err)
}

func TestDramaVideoAssetSignAndResolve(t *testing.T) {
	dir := t.TempDir()
	repo := &memoryDramaAssets{}
	svc := NewDramaVideoAssetService(repo, nil, &config.Config{
		DramaVideo: config.DramaVideoConfig{
			StorageDir:         dir,
			PublicBaseURL:     "https://example.com",
			AssetSigningSecret: "test-secret",
			AssetURLTTLMinutes: 30,
		},
	})

	png := []byte("\x89PNG\r\n\x1a\nxxxx")
	got, err := svc.Upload(context.Background(), 7, "ref.png", png)
	require.NoError(t, err)
	require.Equal(t, "image", got.Kind)
	require.True(t, strings.HasPrefix(got.ID, "vidasset_"))

	again, err := svc.Upload(context.Background(), 7, "ref.png", png)
	require.NoError(t, err)
	require.Equal(t, got.ID, again.ID)

	signed, err := svc.SignURL(got.ID)
	require.NoError(t, err)
	require.Contains(t, signed, "/api/v1/public/video-assets/")
	require.Contains(t, signed, "sig=")

	rewritten, ids, err := svc.ResolveSources(context.Background(), 7, []string{"asset://" + got.ID, "https://cdn.example.com/a.png"})
	require.NoError(t, err)
	require.Equal(t, []string{got.ID}, ids)
	require.True(t, strings.HasPrefix(rewritten[0], "https://example.com/api/v1/public/video-assets/"))
	require.Equal(t, "https://cdn.example.com/a.png", rewritten[1])

	_, _, err = svc.ResolveSources(context.Background(), 8, []string{"asset://" + got.ID})
	require.Error(t, err)
}

func TestDramaVideoCleanupRemovesExpiredOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.mp4")
	require.NoError(t, os.WriteFile(path, []byte("mp4"), 0o600))
	expired := time.Now().Add(-time.Minute)
	tasks := &stubDramaTasks{byID: map[string]*DramaVideoTask{
		"vidtask_1": {
			TaskID:          "vidtask_1",
			Status:          DramaVideoStatusCompleted,
			OutputPath:      path,
			OutputExpiresAt: &expired,
		},
	}}
	svc := NewDramaVideoCleanupService(tasks, nil, nil)
	require.NoError(t, svc.RunOnce(context.Background(), time.Now()))
	_, err := os.Stat(path)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	require.NotNil(t, tasks.byID["vidtask_1"].OutputDeletedAt)
}
