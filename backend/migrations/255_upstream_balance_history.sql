-- 中转上游余额历史快照：每次成功查询写一行，用于计算消耗速度与可用天数，保留 30 天。
CREATE TABLE IF NOT EXISTS upstream_balance_snapshots (
    id           BIGSERIAL PRIMARY KEY,
    account_id   BIGINT        NOT NULL,
    balance      NUMERIC(20, 6),
    used         NUMERIC(20, 6),
    limit_amount NUMERIC(20, 6),
    unlimited    BOOLEAN       NOT NULL DEFAULT FALSE,
    source       VARCHAR(32)   NOT NULL DEFAULT '',
    fetched_at   TIMESTAMPTZ   NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_upstream_balance_snapshots_account_fetched
    ON upstream_balance_snapshots (account_id, fetched_at DESC);
CREATE INDEX IF NOT EXISTS idx_upstream_balance_snapshots_fetched
    ON upstream_balance_snapshots (fetched_at);

-- 上游余额短信提醒记录：用于后台查看历史、去重与每日限额。
-- account_levels: {"<钱包主账号ID>": "warn|critical|exhausted"}
CREATE TABLE IF NOT EXISTS upstream_balance_alerts (
    id             BIGSERIAL PRIMARY KEY,
    level          VARCHAR(16)  NOT NULL,
    account_levels JSONB        NOT NULL DEFAULT '{}'::jsonb,
    content        TEXT         NOT NULL DEFAULT '',
    provider       VARCHAR(16)  NOT NULL DEFAULT '',
    phones         TEXT         NOT NULL DEFAULT '',
    status         VARCHAR(16)  NOT NULL,
    error          TEXT         NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upstream_balance_alerts_created
    ON upstream_balance_alerts (created_at DESC);
