-- Admin-authored marketing email campaigns.
-- Recipients are snapshotted when a campaign is launched so the worker can
-- resume after restarts and each user receives a campaign at most once.
CREATE TABLE IF NOT EXISTS email_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    subject VARCHAR(200) NOT NULL,
    body_html TEXT NOT NULL,
    layout_mode VARCHAR(20) NOT NULL DEFAULT 'card',
    title VARCHAR(200) NOT NULL DEFAULT '',
    accent_color VARCHAR(20) NOT NULL DEFAULT '#4f46e5',
    locale VARCHAR(10) NOT NULL DEFAULT 'zh',
    audience JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    throttle_per_minute INTEGER NOT NULL DEFAULT 30,
    total_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    created_by BIGINT,
    worker_id VARCHAR(64) NOT NULL DEFAULT '',
    heartbeat_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_campaigns_status_created
    ON email_campaigns (status, created_at);

CREATE TABLE IF NOT EXISTS email_campaign_recipients (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES email_campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    email VARCHAR(255) NOT NULL,
    recipient_name VARCHAR(255) NOT NULL DEFAULT '',
    balance DECIMAL(20,8) NOT NULL DEFAULT 0,
    last_activity_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error TEXT NOT NULL DEFAULT '',
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (campaign_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_email_campaign_recipients_campaign_status
    ON email_campaign_recipients (campaign_id, status, id);
