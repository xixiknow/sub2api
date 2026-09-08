-- User-uploaded video workbench assets and task retention columns.

CREATE TABLE IF NOT EXISTS drama_video_assets (
    id BIGSERIAL PRIMARY KEY,
    asset_id VARCHAR(80) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    kind VARCHAR(16) NOT NULL,
    mime VARCHAR(100) NOT NULL,
    bytes BIGINT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    original_name VARCHAR(255),
    storage_path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_drama_video_assets_owner
    ON drama_video_assets (user_id, deleted_at, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_drama_video_assets_user_sha
    ON drama_video_assets (user_id, sha256)
    WHERE deleted_at IS NULL;

ALTER TABLE drama_video_tasks
    ADD COLUMN IF NOT EXISTS asset_ids TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE drama_video_tasks
    ADD COLUMN IF NOT EXISTS output_expires_at TIMESTAMPTZ;

ALTER TABLE drama_video_tasks
    ADD COLUMN IF NOT EXISTS output_deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_drama_video_tasks_output_expires
    ON drama_video_tasks (output_expires_at)
    WHERE output_deleted_at IS NULL AND status = 'completed';

CREATE INDEX IF NOT EXISTS idx_drama_video_assets_last_used
    ON drama_video_assets (last_used_at)
    WHERE deleted_at IS NULL;

COMMENT ON TABLE drama_video_assets IS
    'User-uploaded reference media for the video workbench. Files live on local disk.';
