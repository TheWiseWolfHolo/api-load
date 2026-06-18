package proxy

import (
	"encoding/json"

	"api-load/internal/services"
)

const upstreamTokenUsageContextKey = "upstream_token_usage"

func extractUpstreamTokenUsage(body []byte) (services.TokenUsage, bool) {
	var payload struct {
		Usage struct {
			PromptTokens             int64 `json:"prompt_tokens"`
			CompletionTokens         int64 `json:"completion_tokens"`
			TotalTokens              int64 `json:"total_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			PromptTokensDetails      struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CompletionTokensDetails struct {
				ReasoningTokens int64 `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return services.TokenUsage{}, false
	}
	usage := payload.Usage
	total := usage.TotalTokens
	if total == 0 {
		total = usage.PromptTokens + usage.CompletionTokens
	}
	cacheRead := usage.CacheReadInputTokens
	if cacheRead == 0 {
		cacheRead = usage.PromptTokensDetails.CachedTokens
	}
	tokenUsage := services.TokenUsage{
		InputTokens:      usage.PromptTokens,
		OutputTokens:     usage.CompletionTokens,
		TotalTokens:      total,
		CacheReadTokens:  cacheRead,
		CacheWriteTokens: usage.CacheCreationInputTokens,
		ThinkingTokens:   usage.CompletionTokensDetails.ReasoningTokens,
	}
	if tokenUsage.InputTokens == 0 && tokenUsage.OutputTokens == 0 && tokenUsage.TotalTokens == 0 &&
		tokenUsage.CacheReadTokens == 0 && tokenUsage.CacheWriteTokens == 0 && tokenUsage.ThinkingTokens == 0 {
		return services.TokenUsage{}, false
	}
	return tokenUsage, true
}
