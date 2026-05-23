-- Badge benefit rules:
-- - user_badges stores computed badge unlock state for admin inspection.
-- - badge_benefit_rules lets admins attach automatic benefits to badges.
-- Existing manual per-user group rates / affiliate rates still take precedence.

CREATE TABLE IF NOT EXISTS user_badges (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    badge_id VARCHAR(80) NOT NULL,
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, badge_id)
);

CREATE INDEX IF NOT EXISTS idx_user_badges_badge_id
    ON user_badges(badge_id);

CREATE INDEX IF NOT EXISTS idx_user_badges_unlocked_at
    ON user_badges(unlocked_at DESC);

CREATE TABLE IF NOT EXISTS badge_benefit_rules (
    id BIGSERIAL PRIMARY KEY,
    badge_id VARCHAR(80) NOT NULL,
    name VARCHAR(120) NOT NULL,
    benefit_type VARCHAR(40) NOT NULL,
    group_id BIGINT REFERENCES groups(id) ON DELETE CASCADE,
    rate_multiplier DECIMAL(10,4),
    affiliate_rebate_rate_percent DECIMAL(8,4),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'badge_benefit_rules_type_check'
    ) THEN
        ALTER TABLE badge_benefit_rules
            ADD CONSTRAINT badge_benefit_rules_type_check
            CHECK (benefit_type IN ('group_rate', 'affiliate_rebate'));
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'badge_benefit_rules_payload_check'
    ) THEN
        ALTER TABLE badge_benefit_rules
            ADD CONSTRAINT badge_benefit_rules_payload_check
            CHECK (
                (
                    benefit_type = 'group_rate'
                    AND group_id IS NOT NULL
                    AND rate_multiplier IS NOT NULL
                    AND rate_multiplier > 0
                    AND affiliate_rebate_rate_percent IS NULL
                )
                OR
                (
                    benefit_type = 'affiliate_rebate'
                    AND group_id IS NULL
                    AND rate_multiplier IS NULL
                    AND affiliate_rebate_rate_percent IS NOT NULL
                    AND affiliate_rebate_rate_percent >= 0
                    AND affiliate_rebate_rate_percent <= 100
                )
            );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_badge_benefit_rules_badge_id
    ON badge_benefit_rules(badge_id);

CREATE INDEX IF NOT EXISTS idx_badge_benefit_rules_type_enabled
    ON badge_benefit_rules(benefit_type, enabled);

CREATE INDEX IF NOT EXISTS idx_badge_benefit_rules_group_id
    ON badge_benefit_rules(group_id)
    WHERE group_id IS NOT NULL;
