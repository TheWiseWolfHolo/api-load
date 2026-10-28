package bifrost

import (
	"encoding/json"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"

	"gpt-load/internal/channel"
	"gpt-load/internal/dialect"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

func prepareNativeServices(spec execution.AttemptSpec, resolved channel.ResolvedTarget, provider schemas.ModelProvider, key schemas.Key, secrets []string) (preparedAttempt, *execution.AttemptResult) {
	e, err := dialect.ClassifyNativeService(spec.Method, spec.Path)
	if err != nil || e.Service != spec.ChannelID {
		failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "native service does not match the selected channel")
		failure.Error.OriginHint = execution.ErrorOriginClient
		failure.Error.ScopeHint = execution.ErrorScopeRequest
		return preparedAttempt{}, &failure
	}
	request, err := dialect.NewNativeServices().RewriteRequestModel(&dialect.ParsedRequest{Method: spec.Method, Path: spec.Path, Header: spec.Header.Clone(), Body: spec.Body}, spec.UpstreamModel)
	if err != nil {
		failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "invalid native service request")
		return preparedAttempt{}, &failure
	}
	if len(request.Body) > 0 && !strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/") {
		object, decodeErr := decodeNativeJSONObject(request.Body)
		if decodeErr != nil {
			failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "invalid native service JSON")
			return preparedAttempt{}, &failure
		}
		if stripNativeControlFields(object) {
			request.Body, err = encodeNativeJSONObject(object)
			if err != nil {
				failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "invalid native service JSON")
				return preparedAttempt{}, &failure
			}
		}
	}
	base, ok, err := targetBaseURL(resolved.TargetConfig)
	if err != nil || !ok {
		failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "native service target is required")
		return preparedAttempt{}, &failure
	}
	if e.Collection == "tts" {
		var config struct {
			TTSBaseURL string `json:"tts_base_url"`
		}
		if err := json.Unmarshal(resolved.TargetConfig, &config); err != nil {
			failure := notSentUnaryFailure(execution.ErrorKindInvalidRequest, "invalid TTS target")
			return preparedAttempt{}, &failure
		}
		base = config.TTSBaseURL
		if base == "" {
			base = "https://tts-rt.soniox.com"
		}
	}
	return preparedAttempt{provider: provider, mode: channel.RouteNative, upstreamProtocol: protocol.NativeServices, clientProtocol: protocol.NativeServices, directKey: key, secrets: secrets,
		passthrough: &schemas.BifrostPassthroughRequest{Provider: provider, Model: spec.UpstreamModel, Method: spec.Method, Path: strings.TrimPrefix(e.Path, "/v1"), UpstreamURL: base, RawQuery: safeAttemptQuery(spec), Body: request.Body, SafeHeaders: safePassthroughHeaders(request.Header)}}, nil
}
