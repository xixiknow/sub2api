-- 222_channel_video_model_prices.sql
-- 渠道自定义定价新增视频模型定价列，支持按模型族×分辨率覆盖视频每秒单价。
-- 与 groups.video_model_prices 列保持一致的数据结构。
-- Shape: {"grok-imagine-video":{"480p":0.05,"720p":0.07},"grok-imagine-video-1.5":{"480p":0.08,"720p":0.14,"1080p":0.25}}
-- 计费回退顺序：channel_model_pricing.video_model_prices → groups.video_model_prices → groups.video_price_* 列 → 代码默认值

ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS video_model_prices JSONB;

COMMENT ON COLUMN channel_model_pricing.video_model_prices IS
    '可选：渠道级按模型族×分辨率覆盖视频每秒单价 (USD/s)。key 为规范模型族 (grok-imagine-video / grok-imagine-video-1.5)，value 为分辨率→单价映射；NULL/空表示不覆盖，回退到分组的 video_model_prices 或默认定价';
