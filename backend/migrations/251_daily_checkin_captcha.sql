-- Nullable for historical claims. All new claims require a verified fingerprint.
ALTER TABLE daily_checkins ADD COLUMN IF NOT EXISTS captcha_hash VARCHAR(64);
CREATE UNIQUE INDEX IF NOT EXISTS daily_checkins_captcha_hash_unique
    ON daily_checkins(captcha_hash) WHERE captcha_hash IS NOT NULL;
