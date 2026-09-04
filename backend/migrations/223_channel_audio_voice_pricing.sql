-- 223_channel_audio_voice_pricing.sql
-- 渠道自定义定价新增 Grok Voice 语音定价列：realtime / TTS / STT。
-- 与 groups 表的语音定价列保持一致的数据结构。
-- NULL = 使用分组或代码默认单价；显式 0 = 免费；>0 = 渠道覆盖价。
-- 计费回退顺序：channel_model_pricing → groups → 代码默认值

ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS audio_realtime_price_per_min DECIMAL(20,8);

ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS audio_tts_price_per_million_chars DECIMAL(20,8);

ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS audio_stt_price_per_hour DECIMAL(20,8);

COMMENT ON COLUMN channel_model_pricing.audio_realtime_price_per_min IS
    '可选：渠道级实时语音每分钟价格 (USD/min)，NULL 表示使用分组或默认价格';

COMMENT ON COLUMN channel_model_pricing.audio_tts_price_per_million_chars IS
    '可选：渠道级 TTS 每百万字符价格 (USD/1M chars)，NULL 表示使用分组或默认价格';

COMMENT ON COLUMN channel_model_pricing.audio_stt_price_per_hour IS
    '可选：渠道级 STT 每小时价格 (USD/hour)，NULL 表示使用分组或默认价格';
