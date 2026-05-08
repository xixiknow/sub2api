-- Registration seats backed by affiliate invite codes.
ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS registration_seat_total INTEGER NOT NULL DEFAULT 0;

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS registration_seat_used INTEGER NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_user_affiliates_registration_seats_nonnegative'
    ) THEN
        ALTER TABLE user_affiliates
            ADD CONSTRAINT chk_user_affiliates_registration_seats_nonnegative
                CHECK (registration_seat_total >= 0 AND registration_seat_used >= 0);
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_user_affiliates_registration_seats
    ON user_affiliates(aff_code, registration_seat_total, registration_seat_used);

COMMENT ON COLUMN user_affiliates.registration_seat_total IS 'Total registration seats purchased for this affiliate code';
COMMENT ON COLUMN user_affiliates.registration_seat_used IS 'Registration seats consumed by successful signups';

CREATE TABLE IF NOT EXISTS user_affiliate_registration_seat_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(32) NOT NULL,
    quantity INTEGER NOT NULL,
    amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    source_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    balance_after DECIMAL(20,8) NULL,
    seat_total_after INTEGER NULL,
    seat_used_after INTEGER NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_affiliate_registration_seat_ledger_user_id
    ON user_affiliate_registration_seat_ledger(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_affiliate_registration_seat_ledger_action
    ON user_affiliate_registration_seat_ledger(action, created_at DESC);

COMMENT ON TABLE user_affiliate_registration_seat_ledger IS 'Affiliate registration seat purchase/use/restore ledger';
COMMENT ON COLUMN user_affiliate_registration_seat_ledger.action IS 'purchase|use|restore';
COMMENT ON COLUMN user_affiliate_registration_seat_ledger.quantity IS 'Seat quantity delta for this ledger entry';
COMMENT ON COLUMN user_affiliate_registration_seat_ledger.amount IS 'Balance amount charged for purchase entries';
COMMENT ON COLUMN user_affiliate_registration_seat_ledger.source_user_id IS 'Invitee user id for use/restore entries';

INSERT INTO settings (key, value)
VALUES ('affiliate_registration_seat_cost', '1')
ON CONFLICT (key) DO NOTHING;
