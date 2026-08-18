-- Drama independent video API task persistence.
-- Local tasks keep upstream task ids, billing snapshots, and downloaded output
-- paths so status/content reads are bound to the original submitter/account.

CREATE TABLE IF NOT EXISTS drama_video_tasks (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(80) NOT NULL UNIQUE,
    upstream_task_id VARCHAR(160),
    user_id BIGINT NOT NULL REFERENCES users(id),
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    account_id BIGINT REFERENCES accounts(id),
    model VARCHAR(120) NOT NULL,
    upstream_model VARCHAR(120) NOT NULL,
    status VARCHAR(32) NOT NULL,
    progress INTEGER NOT NULL DEFAULT 0,
    request_hash VARCHAR(64),
    request_body_path TEXT,
    resolution VARCHAR(16),
    aspect_ratio VARCHAR(16),
    duration_seconds INTEGER,
    hold_amount DECIMAL(20,10) NOT NULL DEFAULT 0,
    actual_cost DECIMAL(20,10),
    output_path TEXT,
    output_mime VARCHAR(100),
    output_bytes BIGINT,
    output_sha256 VARCHAR(64),
    error JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_drama_video_tasks_owner
    ON drama_video_tasks (user_id, api_key_id, task_id);

CREATE INDEX IF NOT EXISTS idx_drama_video_tasks_status_updated
    ON drama_video_tasks (status, updated_at);

CREATE INDEX IF NOT EXISTS idx_drama_video_tasks_upstream
    ON drama_video_tasks (upstream_task_id)
    WHERE upstream_task_id IS NOT NULL;

COMMENT ON TABLE drama_video_tasks IS
    'Local task records for Drama /v1/videos integration. POST create is submitted upstream once; GET status/content reads this table.';

-- Migration 220 cleared video prices from every non-Grok/non-composite group.
-- Drama is now a first-class video platform, so restore operator-configured
-- Drama video prices from the safety backup when present.
DO $$
BEGIN
    IF to_regclass('public.groups_video_price_backup_220') IS NOT NULL THEN
        UPDATE groups g
        SET video_price_480p = b.video_price_480p,
            video_price_720p = b.video_price_720p,
            video_price_1080p = b.video_price_1080p,
            video_model_prices = b.video_model_prices
        FROM groups_video_price_backup_220 b
        WHERE g.id = b.group_id
          AND g.platform = 'drama'
          AND (
              g.video_price_480p IS NULL
              AND g.video_price_720p IS NULL
              AND g.video_price_1080p IS NULL
              AND (g.video_model_prices IS NULL OR g.video_model_prices = '{}'::jsonb)
          );
    END IF;
END $$;

-- Existing installs created these CHECK constraints before Drama existed.
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'drama'));

ALTER TABLE composite_model_routes
    DROP CONSTRAINT IF EXISTS composite_model_routes_target_platform_check;

ALTER TABLE composite_model_routes
    ADD CONSTRAINT composite_model_routes_target_platform_check
    CHECK (target_platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'drama'));
