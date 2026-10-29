package bifrost

import (
	"bytes"
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

// OpenRouter 在 200 流里推送 {"error":{"code":429}} 时，数字 code 也应被认作限流。
func TestStreamErrorEventWithNumericRateLimitCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"error\":{\"code\":429,\"message\":\"free model is temporarily unavailable\"}}\n\n")
	}))
	defer server.Close()
	runtime := newProtocolTestRuntime(t, testRuntimeOptions{allowPrivateNetwork: true})
	spec := compatibleTestSpec(t, server.URL, `{"model":"client-model","input":"hi"}`, true)
	result := runtime.ExecuteStream(t.Context(), spec, func(execution.StreamEvent) error { return nil })
	if result.Error == nil || result.Error.Hint != execution.FailureHintRateLimited ||
		result.Error.OriginHint != execution.ErrorOriginUpstream {
		t.Fatalf("stream error = %+v", result.Error)
	}
	if err := result.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRouterNativeStreamKeepsNumericErrorFrameForGateway(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Generation-Id", "gen-started-synthetic")
		_, _ = io.WriteString(w, "data: {\"id\":\"gen-started-synthetic\",\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"synthetic first output\"},\"finish_reason\":null}]}\n\n")
		w.(http.Flusher).Flush()
		_, _ = io.WriteString(w, "data: {\"error\":{\"code\":429,\"message\":\"provider temporarily unavailable\"}}\n\n")
	}))
	defer server.Close()
	runtime := newRuntimeForTest(t, testRuntimeOptions{allowPrivateNetwork: true})
	spec := openAIChatAttempt(t, channel.OpenRouter, server.URL+"/api", "native-key")
	spec.Body = []byte(`{"model":"upstream-model","stream":true,"messages":[{"role":"user","content":"synthetic"}]}`)
	var delivered bytes.Buffer
	result := runtime.ExecuteStream(t.Context(), spec, func(event execution.StreamEvent) error {
		if event.Kind == execution.StreamEventData {
			_, _ = delivered.Write(event.Data)
		}
		return nil
	})
	if !bytes.Contains(delivered.Bytes(), []byte(`"code":429`)) || result.StatusCode != 200 || result.Error != nil || result.UpstreamRequestID != "gen-started-synthetic" {
		t.Fatalf("native error frame or metadata was lost: status=%d error=%+v request_id=%q", result.StatusCode, result.Error, result.UpstreamRequestID)
	}
	if err := result.Validate(); err != nil {
		t.Fatal(err)
	}
}
