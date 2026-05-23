-- Align invitation growth defaults with the activation plan:
-- initial direct rebate starts at 5%, level 2/3 are locked until invite-count tasks unlock them.
--
-- Only rewrite the historical defaults. Explicit admin-customized values are preserved.
UPDATE settings
SET value = '5',
    updated_at = NOW()
WHERE key = 'affiliate_rebate_rate'
  AND value IN ('20', '20.0', '20.00', '20.00000000');

UPDATE settings
SET value = '[5,1,0.5]',
    updated_at = NOW()
WHERE key = 'affiliate_level_rates'
  AND REPLACE(value, ' ', '') IN ('[20,5,2]', '[5,0,0]');

INSERT INTO settings (key, value)
VALUES ('affiliate_rebate_rate', '5')
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value)
VALUES ('affiliate_level_rates', '[5,1,0.5]')
ON CONFLICT (key) DO NOTHING;

-- Recharge bonus campaign rules are stored as JSON through the existing payment
-- config path. Empty array keeps current recharge behavior unchanged.
INSERT INTO settings (key, value)
VALUES ('RECHARGE_BONUS_RULES', '[]')
ON CONFLICT (key) DO NOTHING;
