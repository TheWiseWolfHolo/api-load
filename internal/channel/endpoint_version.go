package channel

import (
	"net/url"
	"strings"
)

// normalizeEndpointVersion 让 base_url 带不带版本段都能用。
// 不同渠道对 base_url 的约定不同：网关类和由 SDK 自己补 /v1 的渠道要根地址，
// OpenAI 兼容类要带版本的完整前缀，Gemini 要带 /v1beta。这里按渠道类型把
// 用户填写的地址改成执行层需要的形式；只调整末尾版本段，不动其他路径。
func normalizeEndpointVersion(definition definition, baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return baseURL
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch definition.providerKind {
	case ProviderMultiProtocolGateway:
		path = trimVersionSuffix(path, "/v1", "/v1beta")
	case ProviderOpenAI, ProviderAnthropic, ProviderClaude, ProviderOpenRouter, ProviderGroq, ProviderXAI:
		path = trimVersionSuffix(path, "/v1")
	case ProviderOpenAICompatible, ProviderCline, ProviderJev:
		if path == "" {
			path = defaultVersionPath(definition.fixedBaseURL, "/v1")
		}
	case ProviderGemini:
		if path == "" {
			path = "/v1beta"
		}
	default:
		return baseURL
	}
	if path == parsed.Path {
		return baseURL
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String()
}

func trimVersionSuffix(path string, suffixes ...string) string {
	for _, suffix := range suffixes {
		if trimmed, ok := strings.CutSuffix(path, suffix); ok {
			return trimmed
		}
	}
	return path
}

// defaultVersionPath 取预设官方地址里的路径（如 /api/v3、/compatible-mode/v1），
// 用户只填域名或换成镜像域名时沿用它。
func defaultVersionPath(presetBaseURL, fallback string) string {
	if presetBaseURL == "" {
		return fallback
	}
	parsed, err := url.Parse(presetBaseURL)
	if err != nil || strings.TrimRight(parsed.Path, "/") == "" {
		return fallback
	}
	return strings.TrimRight(parsed.Path, "/")
}
