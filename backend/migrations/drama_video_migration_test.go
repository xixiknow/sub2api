package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDramaVideoMigrationExtendsPlatformChecksAndCreatesTasks(t *testing.T) {
	content, err := FS.ReadFile("234_drama_video.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check")
	require.Contains(t, sql, "'kimi', 'zhipu', 'deepseek', 'drama'")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS drama_video_tasks")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS composite_model_routes_target_platform_check")
}

func TestDramaVideoAssetsRetentionMigration(t *testing.T) {
	content, err := FS.ReadFile("235_drama_video_assets_retention.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS drama_video_assets")
	require.Contains(t, sql, "output_expires_at")
	require.Contains(t, sql, "output_deleted_at")
	require.Contains(t, sql, "asset_ids")
}
