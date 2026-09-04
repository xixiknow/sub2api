-- 225_channel_model_pricing_jsonb.sql
-- 渠道自定义定价新增通用模型定价 JSONB 列和长上下文定价开关。
-- 与 groups 表的 model_pricing 和 long_context_pricing_enabled 保持一致。
-- model_pricing: 按模型提供完整的定价覆盖（input/output/cache/image 等）
-- long_context_pricing_enabled: 控制是否启用长上下文分层定价
-- 计费回退顺序：channel_model_pricing.model_pricing → groups.model_pricing → 代码默认值

ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS long_context_pricing_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS model_pricing JSONB;

COMMENT ON COLUMN channel_model_pricing.long_context_pricing_enabled IS
    '是否为 token 定价选择官方/预设的长上下文分层；默认 true 保持现有长上下文计费行为';

COMMENT ON COLUMN channel_model_pricing.model_pricing IS
    '可选：渠道级按模型的完整定价覆盖，优先级高于渠道和内置模型定价';
