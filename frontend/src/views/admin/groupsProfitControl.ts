// 分组利润控制表单辅助：百分比 <-> 小数换算与提交前校验。
// 后端按小数存 decimal(10,4)（0.30 = 30%），界面按百分比输入展示；
// 固定 4 位小数精度，避免 0.3 * 100 = 30.000000000000004 之类的浮点尾数回显。

export const profitPercentToDecimal = (
  value: number | string | null | undefined,
): number => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }
  return Math.round(parsed * 100) / 10000;
};

export const profitDecimalToPercent = (
  value: number | null | undefined,
): number => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }
  return Math.round(parsed * 1e6) / 1e4;
};

export type ProfitControlFormState = {
  platform: string;
  profit_control_enabled: boolean;
  profit_min_margin_percent: number | string | null;
  profit_safety_buffer_percent: number | string | null;
  // 最高调度倍率（绝对上限）；空值表示不限
  profit_max_account_rate?: number | string | null;
};

const isBlank = (value: unknown): boolean =>
  value === null || value === undefined || String(value).trim() === "";

// 最高调度倍率提交值：空 → null（后端清除为不限），否则按 4 位小数取整。
export const profitMaxAccountRateToPayload = (
  value: number | string | null | undefined,
): number | null => {
  if (isBlank(value)) {
    return null;
  }
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }
  return Math.round(parsed * 1e4) / 1e4;
};

// 实际阈值预览：min(分组倍率 × (1 − margin − buffer), 最高调度倍率)。
// 仅用于界面提示（不含用户专属倍率与高峰倍率），输入非法时返回 null。
export const profitControlEffectiveThreshold = (
  rateMultiplier: number | string | null | undefined,
  form: ProfitControlFormState,
): number | null => {
  if (isBlank(rateMultiplier)) {
    return null;
  }
  const rate = Number(rateMultiplier);
  if (!Number.isFinite(rate) || rate < 0) {
    return null;
  }
  const margin = profitPercentToDecimal(form.profit_min_margin_percent);
  const buffer = profitPercentToDecimal(form.profit_safety_buffer_percent);
  let threshold = Math.max(0, rate * (1 - margin - buffer));
  const maxRate = profitMaxAccountRateToPayload(form.profit_max_account_rate);
  if (maxRate !== null && maxRate < threshold) {
    threshold = maxRate;
  }
  return Math.round(threshold * 1e4) / 1e4;
};

export const isProfitControlPlatform = (platform: string): boolean =>
  ["openai", "anthropic", "gemini", "grok", "antigravity"].includes(platform);

// 提交前校验：margin/buffer 各自 ∈ [0,1)，且相加 < 1（否则阈值 <= 0，
// 所有可核价账号都会被排除）。返回 null 表示通过，否则返回错误信息的 i18n key
//（相对 admin.groups.profitControl 前缀）。仅支持平台且开关开启时才校验。
//
// 上界校验必须落在 profitPercentToDecimal 的换算结果上，而不是界面百分比：
// 后端 validProfitControlRatio 校验的是小数 [0,1)，而 99.999% 会被四舍五入
// 进位成 1.0——按百分比判 `< 100` 会让前端放行、后端 400。
export const validateProfitControlFormState = (
  form: ProfitControlFormState,
): string | null => {
  if (!isProfitControlPlatform(form.platform) || !form.profit_control_enabled) {
    return null;
  }
  const marginPercent = Number(form.profit_min_margin_percent || 0);
  const bufferPercent = Number(form.profit_safety_buffer_percent || 0);
  if (!Number.isFinite(marginPercent) || marginPercent < 0) {
    return "marginRangeError";
  }
  if (!Number.isFinite(bufferPercent) || bufferPercent < 0) {
    return "bufferRangeError";
  }
  // 校验实际提交给后端的小数值，边界按构造对齐。
  const margin = profitPercentToDecimal(marginPercent);
  const buffer = profitPercentToDecimal(bufferPercent);
  if (margin >= 1) {
    return "marginRangeError";
  }
  if (buffer >= 1) {
    return "bufferRangeError";
  }
  if (margin + buffer >= 1) {
    return "sumTooHigh";
  }
  if (!isBlank(form.profit_max_account_rate)) {
    const maxRate = Number(form.profit_max_account_rate);
    if (!Number.isFinite(maxRate) || maxRate < 0) {
      return "maxAccountRateRangeError";
    }
  }
  return null;
};
