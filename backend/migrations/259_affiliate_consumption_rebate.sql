-- 邀请返利由「被邀请人充值返利」改为「被邀请人消费返利」。
-- 消费返利由后台任务定时汇总 usage_logs（仅余额扣费）后按被邀请人入账，
-- 游标记录已结算到的 usage_logs.id；上线前的历史消费不补发返利。

CREATE TABLE IF NOT EXISTS affiliate_consumption_settlement_cursor (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    last_usage_log_id BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE affiliate_consumption_settlement_cursor IS '消费返利结算游标（单行）';
COMMENT ON COLUMN affiliate_consumption_settlement_cursor.last_usage_log_id IS '已结算到的 usage_logs.id（含）';

INSERT INTO affiliate_consumption_settlement_cursor (id, last_usage_log_id, updated_at)
SELECT 1, COALESCE(MAX(id), 0), NOW() FROM usage_logs
ON CONFLICT (id) DO NOTHING;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(20) NOT NULL DEFAULT 'order';

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_settlement_id BIGINT NULL;

COMMENT ON COLUMN user_affiliate_ledger.source_type IS '返利来源：order=充值订单/历史充值类来源，consumption=被邀请人消费结算';
COMMENT ON COLUMN user_affiliate_ledger.source_settlement_id IS '消费结算批次（本批次结算到的 usage_logs.id 上界）；非消费返利为 NULL';

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_affiliate_ledger_accrue_settlement_level_uniq
    ON user_affiliate_ledger (source_settlement_id, user_id, source_user_id, level)
    WHERE action = 'accrue' AND source_settlement_id IS NOT NULL;
