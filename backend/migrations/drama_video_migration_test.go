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
