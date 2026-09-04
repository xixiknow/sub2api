-- 224_channel_search_price_per_1k.sql
-- 渠道自定义定价新增搜索工具定价列（per 1000 calls，USD）。
-- 与 groups 表的搜索定价列保持一致的数据结构。
-- NULL = 使用分组或代码默认 $10/1k；显式 0 = 免费；>0 = 渠道覆盖价。
-- 计费回退顺序：channel_model_pricing → groups → 代码默认值

ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS search_price_per_1k DECIMAL(20,8);

COMMENT ON COLUMN channel_model_pricing.search_price_per_1k IS
    '可选：渠道级搜索工具每千次调用价格 (USD/1k calls)，NULL 表示使用分组或默认 $10/1k';
