-- 邀请返利提现申请（支付宝，管理员人工打款）。
-- 提交时即从 user_affiliates.aff_quota 扣除并写 withdraw_request 流水；
-- 驳回/撤销时退回额度并写 withdraw_reverse 流水；标记已打款只改状态。
CREATE TABLE IF NOT EXISTS affiliate_withdraw_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    alipay_account VARCHAR(128) NOT NULL,
    qr_image TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    admin_note TEXT NULL,
    reviewed_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ NULL,
    hold_ledger_id BIGINT NULL,
    release_ledger_id BIGINT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_affiliate_withdraw_requests_status_created
    ON affiliate_withdraw_requests(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_withdraw_requests_user_created
    ON affiliate_withdraw_requests(user_id, created_at DESC);
-- 每个用户同时只能有一笔待审核申请
CREATE UNIQUE INDEX IF NOT EXISTS uniq_affiliate_withdraw_requests_user_pending
    ON affiliate_withdraw_requests(user_id) WHERE status = 'pending';

COMMENT ON TABLE affiliate_withdraw_requests IS '邀请返利支付宝提现申请';
COMMENT ON COLUMN affiliate_withdraw_requests.status IS 'pending|paid|rejected|cancelled';
COMMENT ON COLUMN affiliate_withdraw_requests.qr_image IS '支付宝收款码 data:image/...;base64';
