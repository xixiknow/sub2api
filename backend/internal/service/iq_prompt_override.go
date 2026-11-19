package service

import "context"

// 智力测试复用账号测试链路（代理、TLS 指纹、头覆写与真实转发一致），
// 通过 context 注入自定义 prompt 与输出上限，对上游 account_test_service.go 只增加一行钩子。

type iqPromptOverrideKey struct{}

type iqPromptOverride struct {
	Prompt    string
	MaxTokens int
}

func withIQPromptOverride(ctx context.Context, prompt string, maxTokens int) context.Context {
	return context.WithValue(ctx, iqPromptOverrideKey{}, &iqPromptOverride{Prompt: prompt, MaxTokens: maxTokens})
}

func iqPromptOverrideFrom(ctx context.Context) *iqPromptOverride {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(iqPromptOverrideKey{}).(*iqPromptOverride)
	if v == nil || v.Prompt == "" {
		return nil
	}
	return v
}

// applyIQPromptToClaudePayload 替换 Anthropic Messages 测试载荷的用户消息与 max_tokens。
func applyIQPromptToClaudePayload(ctx context.Context, payload map[string]any) {
	o := iqPromptOverrideFrom(ctx)
	if o == nil || payload == nil {
		return
	}
	payload["messages"] = []map[string]any{{
		"role":    "user",
		"content": []map[string]any{{"type": "text", "text": o.Prompt}},
	}}
	if o.MaxTokens > 0 {
		payload["max_tokens"] = o.MaxTokens
	}
}

// applyIQPromptToResponsesPayload 替换 OpenAI Responses 测试载荷的输入。
func applyIQPromptToResponsesPayload(ctx context.Context, payload map[string]any) {
	o := iqPromptOverrideFrom(ctx)
	if o == nil || payload == nil {
		return
	}
	payload["input"] = []map[string]any{{
		"role":    "user",
		"content": []map[string]any{{"type": "input_text", "text": o.Prompt}},
	}}
	if o.MaxTokens > 0 {
		payload["max_output_tokens"] = o.MaxTokens
	}
}

// applyIQPromptToChatPayload 为 Chat Completions 测试载荷设置输出上限（prompt 已经由参数传入）。
func applyIQPromptToChatPayload(ctx context.Context, payload map[string]any) {
	o := iqPromptOverrideFrom(ctx)
	if o == nil || payload == nil {
		return
	}
	payload["messages"] = []map[string]any{{"role": "user", "content": o.Prompt}}
	if o.MaxTokens > 0 {
		payload["max_tokens"] = o.MaxTokens
	}
}
