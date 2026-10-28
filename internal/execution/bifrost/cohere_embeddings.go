package bifrost

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/maximhq/bifrost/core/schemas"

	"gpt-load/internal/channel"
	"gpt-load/internal/dialect"
	"gpt-load/internal/execution"
	"gpt-load/internal/platform/contentcoding"
	"gpt-load/internal/protocol"
)

func prepareCohereEmbeddings(
	spec execution.AttemptSpec,
	resolved channel.ResolvedTarget,
	provider schemas.ModelProvider,
	directKey schemas.Key,
	secrets []string,
) (preparedAttempt, *execution.AttemptResult) {
	request := &dialect.ParsedRequest{Method: http.MethodPost, Path: "/v2/embed", Header: spec.Header.Clone(), Body: spec.Body}
	if spec.Operation == execution.OperationProbe {
		body, err := json.Marshal(map[string]any{
			"model": spec.UpstreamModel, "texts": []string{"ping"}, "input_type": "search_document", "embedding_types": []string{"float"},
		})
		if err != nil {
			failure := notSentUnaryFailure(execution.ErrorKindInternal, "encode Cohere Embed probe")
			return preparedAttempt{}, &failure
		}
		request.Body = body
	}
	request, err := dialect.NewCohereEmbeddings().RewriteRequestModel(request, spec.UpstreamModel)
	if err != nil {
		failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "invalid Cohere Embed request body")
		failure.Error.OriginHint = execution.ErrorOriginClient
		failure.Error.ScopeHint = execution.ErrorScopeRequest
		return preparedAttempt{}, &failure
	}
	request.Body, err = sanitizeNativeChatBody(request.Body, spec.UpstreamModel)
	if err != nil {
		failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "invalid Cohere Embed body")
		return preparedAttempt{}, &failure
	}
	baseURL, configured, err := targetBaseURL(resolved.TargetConfig)
	if err != nil || !configured {
		failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "invalid Cohere Embed target")
		return preparedAttempt{}, &failure
	}
	path := "/embed"
	if resolved.ProviderKind == channel.ProviderMultiProtocolGateway {
		path = "/v2/embed"
	}
	return preparedAttempt{
		provider: provider, mode: channel.RouteNative, upstreamProtocol: protocol.CohereEmbeddings,
		clientProtocol: protocol.CohereEmbeddings, directKey: directKey, secrets: secrets,
		passthrough: &schemas.BifrostPassthroughRequest{
			Provider: provider, Model: spec.UpstreamModel, Method: http.MethodPost,
			Path: path, UpstreamURL: baseURL, RawQuery: safeAttemptQuery(spec),
			Body: request.Body, SafeHeaders: safePassthroughHeaders(request.Header),
		},
	}, nil
}

func normalizeCohereEmbeddingsAttemptResult(spec execution.AttemptSpec, result *execution.AttemptResult) {
	if result == nil || spec.ClientProtocol != protocol.CohereEmbeddings {
		return
	}
	if result.Error != nil {
		if result.Error.ReplaySafety == "" {
			result.Error.ReplaySafety = execution.ReplaySafetyUnknown
		}
		return
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		return
	}
	encoding, err := contentcoding.ParseContentEncoding(result.Header.Values("Content-Encoding"))
	var body []byte
	if err == nil {
		body, err = contentcoding.DecodeLimited(encoding, result.Body, execution.UnaryResponseBodyLimit(protocol.CohereEmbeddings))
	}
	if err != nil || !validCohereEmbeddingResponse(spec, body) {
		*result = startedUnaryFailure(result.StatusCode, result.Header, execution.ErrorKindProvider, "upstream returned incomplete or invalid Cohere embeddings")
		result.UpstreamProtocol = protocol.CohereEmbeddings
		result.Error.Code = "invalid_embedding_response"
		result.Error.ReplaySafety = execution.ReplaySafetyUnknown
	}
}

func validCohereEmbeddingResponse(spec execution.AttemptSpec, body []byte) bool {
	var request struct {
		Texts     []json.RawMessage `json:"texts"`
		Images    []json.RawMessage `json:"images"`
		Inputs    []json.RawMessage `json:"inputs"`
		Types     []string          `json:"embedding_types"`
		Dimension int               `json:"output_dimension"`
	}
	count := 1
	if spec.Operation != execution.OperationProbe {
		if json.Unmarshal(spec.Body, &request) != nil {
			return false
		}
		count = len(request.Texts) + len(request.Images) + len(request.Inputs)
	}
	if len(request.Types) == 0 {
		request.Types = []string{"float"}
	}
	var response struct {
		Embeddings map[string]json.RawMessage `json:"embeddings"`
	}
	if json.Unmarshal(body, &response) != nil {
		return false
	}
	for _, kind := range request.Types {
		if kind == "base64" {
			var values []string
			if json.Unmarshal(response.Embeddings[kind], &values) != nil || len(values) != count {
				return false
			}
			for _, value := range values {
				decoded, err := base64.StdEncoding.DecodeString(value)
				if err != nil || len(decoded) == 0 || len(decoded)%4 != 0 || request.Dimension > 0 && len(decoded)/4 != request.Dimension {
					return false
				}
			}
			continue
		}
		var values [][]float64
		if json.Unmarshal(response.Embeddings[kind], &values) != nil || len(values) != count {
			return false
		}
		dimension := request.Dimension
		if kind == "binary" || kind == "ubinary" {
			dimension /= 8
		}
		for _, value := range values {
			if len(value) == 0 || dimension > 0 && len(value) != dimension {
				return false
			}
		}
	}
	return count > 0
}
