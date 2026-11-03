-- Linked groups for channel monitors; users see these groups' rate multipliers.
ALTER TABLE channel_monitors ADD COLUMN IF NOT EXISTS group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;
