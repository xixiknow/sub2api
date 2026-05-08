-- 三级邀请返佣：为返佣流水记录层级、比例，并用订单+层级保证幂等。

-- 兼容历史实例：部分设置表由早期 schema 创建，只有 updated_at 而没有 created_at。
-- 后续 Go/SQL 写入路径可能会带上 created_at，先补齐默认值避免安装阶段中断。
ALTER TABLE settings
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS level INTEGER NOT NULL DEFAULT 1;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS rate_percent DECIMAL(10,4) NOT NULL DEFAULT 0;

COMMENT ON COLUMN user_affiliate_ledger.level IS '返佣层级：1=直接邀请，2/3=上级邀请链路';
COMMENT ON COLUMN user_affiliate_ledger.rate_percent IS '该笔返佣发放时使用的比例快照（百分比）';
COMMENT ON COLUMN user_affiliate_ledger.action IS 'accrue|transfer|refund_clawback';

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_level
    ON user_affiliate_ledger(level)
    WHERE action = 'accrue';

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_affiliate_ledger_accrue_order_level_uniq
    ON user_affiliate_ledger(source_order_id, user_id, source_user_id, level)
    WHERE action = 'accrue' AND source_order_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_affiliate_ledger_clawback_order_level_uniq
    ON user_affiliate_ledger(source_order_id, user_id, source_user_id, level)
    WHERE action = 'refund_clawback' AND source_order_id IS NOT NULL;

INSERT INTO settings (key, value)
VALUES ('affiliate_level_rates', '[20,5,2]')
ON CONFLICT (key) DO NOTHING;

UPDATE settings
SET value = '168',
    updated_at = NOW()
WHERE key = 'affiliate_rebate_freeze_hours'
  AND (value IS NULL OR value = '' OR value = '0');
