-- 请求时刻的上游声明倍率快照（含当时生效的高峰系数），用于盈利统计成本核算。
-- NULL 表示历史数据或账号未探测上游倍率，统计时回退到账号当前声明倍率。
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS upstream_rate_multiplier NUMERIC;
