package services

import "api-load/internal/models"

type TokenUsage struct {
	InputTokens      int64
	OutputTokens     int64
	TotalTokens      int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	ThinkingTokens   int64
}

func ApplyUpstreamTokenUsage(log *models.RequestLog, usage TokenUsage) {
	if log == nil {
		return
	}
	applyTokenUsage(log, usage, models.TokenUsageSourceUpstream)
}

func ApplyEstimatedTokenUsage(log *models.RequestLog, usage TokenUsage) {
	if log == nil || log.TokenUsageSource == models.TokenUsageSourceUpstream {
		return
	}
	applyTokenUsage(log, usage, models.TokenUsageSourceEstimated)
}

func applyTokenUsage(log *models.RequestLog, usage TokenUsage, source string) {
	log.InputTokens = usage.InputTokens
	log.OutputTokens = usage.OutputTokens
	log.TotalTokens = usage.TotalTokens
	log.CacheReadTokens = usage.CacheReadTokens
	log.CacheWriteTokens = usage.CacheWriteTokens
	log.ThinkingTokens = usage.ThinkingTokens
	log.TokenUsageSource = source
}
