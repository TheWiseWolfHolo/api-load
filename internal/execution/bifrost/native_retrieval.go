package bifrost

import (
	"net/http"

	"github.com/maximhq/bifrost/core/schemas"

	"gpt-load/internal/channel"
	"gpt-load/internal/dialect"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

// 原生内容对象绕过 SDK 的文字型 EmbeddingInput，保留完整请求及响应。
func prepareStructuredEmbedding(spec execution.AttemptSpec, resolved channel.ResolvedTarget, provider schemas.ModelProvider, directKey schemas.Key, secrets []string) (preparedAttempt, *execution.AttemptResult) {
	request, err := dialect.NewOpenAIEmbeddings().SanitizeRequestForAttempt(&dialect.ParsedRequest{Method: spec.Method, Path: spec.Path, Header: spec.Header.Clone(), Body: spec.Body}, spec.UpstreamModel)
	if err != nil {
		failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "invalid structured embedding request")
		return preparedAttempt{}, &failure
	}
	baseURL, configured, err := targetBaseURL(resolved.TargetConfig)
	if err == nil && !configured {
		baseURL, _, err = sdkDefaultBaseURL(resolved.ProviderKind)
	}
	if err != nil || baseURL == "" {
		failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "native embedding target is required")
		return preparedAttempt{}, &failure
	}
	path := "/embeddings"
	if resolved.ProviderKind != channel.ProviderOpenAICompatible {
		path = "/v1/embeddings"
	}
	return preparedAttempt{
		provider: provider, mode: channel.RouteNative, upstreamProtocol: protocol.OpenAIEmbeddings,
		clientProtocol: protocol.OpenAIEmbeddings, directKey: directKey, secrets: secrets,
		passthrough: &schemas.BifrostPassthroughRequest{Provider: provider, Model: spec.UpstreamModel, Method: http.MethodPost, Path: path, UpstreamURL: baseURL, RawQuery: safeAttemptQuery(spec), Body: request.Body, SafeHeaders: safePassthroughHeaders(request.Header)},
	}, nil
}
