-- 模型智力测试（fork 扩展）
-- 题库、定时计划、测试结果、画图题参考样本、分类器模型

CREATE TABLE IF NOT EXISTS iq_questions (
    id          BIGSERIAL PRIMARY KEY,
    slug        VARCHAR(64)  NOT NULL UNIQUE,
    kind        VARCHAR(32)  NOT NULL,             -- candy | random_reasoning | svg_draw
    title       VARCHAR(128) NOT NULL DEFAULT '',
    prompt      TEXT         NOT NULL DEFAULT '',
    keywords    JSONB        NOT NULL DEFAULT '[]', -- svg_draw 的部件词
    builtin     BOOLEAN      NOT NULL DEFAULT FALSE,
    enabled     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS iq_test_plans (
    id                BIGSERIAL PRIMARY KEY,
    account_id        BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    model_id          VARCHAR(128) NOT NULL DEFAULT '',
    question_slug     VARCHAR(64)  NOT NULL,            -- 题目 slug；random 表示随机推理题
    parallel_count    INT          NOT NULL DEFAULT 1,
    cron_expression   VARCHAR(64)  NOT NULL DEFAULT '0 */6 * * *',
    enabled           BOOLEAN      NOT NULL DEFAULT TRUE,
    fail_action       VARCHAR(32)  NOT NULL DEFAULT 'observe_only', -- observe_only | health_penalty | disable_scheduling
    fail_threshold    INT          NOT NULL DEFAULT 2,  -- 连续失败轮数阈值
    consecutive_fails INT          NOT NULL DEFAULT 0,
    action_applied    BOOLEAN      NOT NULL DEFAULT FALSE, -- 本功能当前是否已对账号施加处置
    max_results       INT          NOT NULL DEFAULT 50,
    last_verdict      VARCHAR(16)  NOT NULL DEFAULT '',
    last_run_at       TIMESTAMPTZ,
    next_run_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_iq_test_plans_account ON iq_test_plans(account_id);
CREATE INDEX IF NOT EXISTS idx_iq_test_plans_due ON iq_test_plans(enabled, next_run_at);

CREATE TABLE IF NOT EXISTS iq_test_results (
    id             BIGSERIAL PRIMARY KEY,
    plan_id        BIGINT       REFERENCES iq_test_plans(id) ON DELETE CASCADE, -- 手动测试为 NULL
    account_id     BIGINT       NOT NULL,
    model_id       VARCHAR(128) NOT NULL DEFAULT '',
    question_slug  VARCHAR(64)  NOT NULL,
    round_id       VARCHAR(64)  NOT NULL DEFAULT '',
    prompt         TEXT         NOT NULL DEFAULT '',
    response_text  TEXT         NOT NULL DEFAULT '',
    verdict        VARCHAR(16)  NOT NULL,               -- passed | failed | inconclusive | error
    score          DOUBLE PRECISION,
    detail         JSONB        NOT NULL DEFAULT '{}',
    error_message  TEXT         NOT NULL DEFAULT '',
    label          VARCHAR(16)  NOT NULL DEFAULT '',    -- 人工标注 good | bad
    latency_ms     BIGINT       NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_iq_test_results_plan ON iq_test_results(plan_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_iq_test_results_account ON iq_test_results(account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS iq_reference_samples (
    id             BIGSERIAL PRIMARY KEY,
    question_slug  VARCHAR(64)  NOT NULL,
    model_id       VARCHAR(128) NOT NULL DEFAULT '',   -- 空表示通用（不区分模型）
    label          VARCHAR(16)  NOT NULL DEFAULT 'good', -- good | bad
    source         VARCHAR(32)  NOT NULL DEFAULT 'upload', -- upload | result
    note           VARCHAR(255) NOT NULL DEFAULT '',
    svg            TEXT         NOT NULL,
    features       JSONB        NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_iq_reference_samples_q ON iq_reference_samples(question_slug, model_id, label);

CREATE TABLE IF NOT EXISTS iq_classifier_models (
    question_slug  VARCHAR(64)  PRIMARY KEY,
    model          JSONB        NOT NULL,
    n_good         INT          NOT NULL DEFAULT 0,
    n_bad          INT          NOT NULL DEFAULT 0,
    accuracy       DOUBLE PRECISION NOT NULL DEFAULT 0,
    trained_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
