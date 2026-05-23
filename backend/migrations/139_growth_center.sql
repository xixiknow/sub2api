-- Growth center state:
-- 1) classify permanent promo codes that prove a user joined the QQ community;
-- 2) persist tutorial-only user growth events across devices.
ALTER TABLE promo_codes
    ADD COLUMN IF NOT EXISTS purpose VARCHAR(40) NOT NULL DEFAULT 'general';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'promo_codes_purpose_check'
    ) THEN
        ALTER TABLE promo_codes
            ADD CONSTRAINT promo_codes_purpose_check
            CHECK (purpose IN ('general', 'community_join'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_promo_codes_purpose
    ON promo_codes(purpose);

CREATE TABLE IF NOT EXISTS user_growth_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_key VARCHAR(80) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_growth_events_event_key_check'
    ) THEN
        ALTER TABLE user_growth_events
            ADD CONSTRAINT user_growth_events_event_key_check
            CHECK (event_key IN ('affiliate_tutorial_done'));
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_growth_events_user_event_unique
    ON user_growth_events(user_id, event_key);

CREATE INDEX IF NOT EXISTS idx_user_growth_events_user_id
    ON user_growth_events(user_id);
