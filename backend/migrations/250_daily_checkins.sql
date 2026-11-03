-- Rewards and their probability snapshots are immutable after the daily claim.
CREATE TABLE IF NOT EXISTS daily_checkins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    checkin_date DATE NOT NULL,
    reward_cents INTEGER NOT NULL CHECK (reward_cents IN (1, 3, 10, 50, 100)),
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, checkin_date)
);
