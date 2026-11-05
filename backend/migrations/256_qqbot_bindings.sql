-- QQ official bot: one QQ openid <-> one site account per bot app.
-- Kept separate from auth_identities (login providers) because QQ bot openids
-- are not a login method, only a chat-side shortcut identity.
CREATE TABLE IF NOT EXISTS qqbot_bindings (
    id BIGSERIAL PRIMARY KEY,
    app_id VARCHAR(64) NOT NULL,
    openid VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scene VARCHAR(16) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT qqbot_bindings_openid_unique UNIQUE (app_id, openid),
    CONSTRAINT qqbot_bindings_user_unique UNIQUE (app_id, user_id)
);
