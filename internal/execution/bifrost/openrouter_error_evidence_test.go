package bifrost

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gpt-load/internal/channel"
	"gpt-load/internal/execution"
)

// OpenRouter attaches X-Generation-Id to error responses. The evidence keeps
// that header, so the result contract must accept it; otherwise a real
// upstream 429 is reported as an internal executor failure and skips cooldown.
func TestOpenRouterErrorWithGenerationIDKeepsUpstreamEvidence(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Generation-Id", "gen-rate-limited")
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, `{"error":{"message":"Provider returned error","code":429,"metadata":{"raw":"free model is temporarily rate-limited upstream"}}}`)
			}))
			defer server.Close()
			runtime := newRuntimeForTest(t, testRuntimeOptions{allowPrivateNetwork: true})
			spec := openAIChatAttempt(t, channel.OpenRouter, server.URL+"/api", "native-key")

			var status int
			var evidence *execution.ErrorEvidence
			var validateErr error
			if stream {
				spec.Body = []byte(`{"model":"upstream-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
				result := runtime.ExecuteStream(t.Context(), spec, func(execution.StreamEvent) error { return nil })
				status, evidence, validateErr = result.StatusCode, result.Error, result.Validate()
			} else {
				spec.Body = []byte(`{"model":"upstream-model","messages":[{"role":"user","content":"hi"}]}`)
				result := runtime.Execute(t.Context(), spec)
				status, evidence, validateErr = result.StatusCode, result.Error, result.Validate()
			}
			if validateErr != nil {
				t.Fatalf("result contract rejected upstream error: %v", validateErr)
			}
			if status != http.StatusTooManyRequests || evidence == nil ||
				evidence.Hint != execution.FailureHintRateLimited ||
				evidence.OriginHint != execution.ErrorOriginUpstream ||
				evidence.Header.Get("X-Generation-Id") != "gen-rate-limited" {
				t.Fatalf("status=%d evidence=%+v", status, evidence)
			}
		})
	}
}
