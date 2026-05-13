-- 兼容历史实例：部分 settings 表由早期 schema 创建，只有 updated_at 而没有 created_at。
-- 该补丁从 135_affiliate_multilevel.sql 拆出，保持已发布迁移文件不可变。
ALTER TABLE settings
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
