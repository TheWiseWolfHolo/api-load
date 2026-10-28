package gateway

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/channel"
	"gpt-load/internal/execution"
	"gpt-load/internal/health"
	"gpt-load/internal/platform/config"
	"gpt-load/internal/protocol"
	"gpt-load/internal/scheduler"
	"gpt-load/internal/state"
)

func transparentFixture(t *testing.T, target, strategy string) (*Handler, *gin.Engine, *recordingRequestLogSink) {
	t.Helper()
	h, manager, _ := newHandlerForTest(t, &scriptedForwarder{}, "managed-key-one", "managed-key-two")
	sink := &recordingRequestLogSink{}
	h.requestLogSink = sink
	fixed := 0
	if strategy == "fixed" {
		fixed = 2
	}
	_, err := manager.Publish(state.CompileInput{ChannelRegistry: channel.NewRegistry(), Groups: []state.GroupConfig{{ID: 1, Name: "transparent-source", ChannelID: channel.Cohere, ConnectionType: "api_key", Params: json.RawMessage(`{}`), Models: []state.ModelConfig{{ID: "embed-synthetic"}}, Enabled: true, Settings: config.Settings{"transparent_proxy": map[string]any{"enabled": true, "base_url": target, "auth_header": "X-Provider-Key", "auth_prefix": "Token ", "strategy": strategy, "fixed_credential_id": fixed}}}}, Credentials: []state.CredentialConfig{{ID: 1, GroupID: 1, Status: state.CredentialStatusActive, Version: 1, IdentityGeneration: 1, Fingerprint: "one"}, {ID: 2, GroupID: 1, Status: state.CredentialStatusActive, Version: 1, IdentityGeneration: 2, Fingerprint: "two"}}, AccessKeys: []state.AccessKeyConfig{{ID: 1, KeyHash: h.encryption.Hash("gl-client"), Status: state.AccessKeyStatusActive, Filters: state.FilterSet{Protocols: map[protocol.Protocol]struct{}{protocol.TransparentHTTP: {}}, Transparent: &state.TransparentScope{GroupIDs: []uint{1}, Methods: []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, Paths: []state.TransparentPathRule{{Path: "/", Match: "subtree"}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	bindGatewayRoutesForTest(t, engine, h)
	return h, engine, sink
}

func transparentCall(engine http.Handler, method, path string, body []byte, headers http.Header) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header = headers.Clone()
	if r.Header == nil {
		r.Header = make(http.Header)
	}
	r.Header.Set("Authorization", "Bearer gl-client")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, r)
	return w
}

func TestTransparentPreservesOpaqueRequestsAndResponses(t *testing.T) {
	var encoded bytes.Buffer
	zw := gzip.NewWriter(&encoded)
	_, _ = zw.Write([]byte("compressed synthetic bytes"))
	_ = zw.Close()
	for _, test := range []struct {
		name, contentType, encoding string
		body                        []byte
	}{
		{"unknown-json", "application/json", "", []byte("{ \"model\":\"vendor/v123\",\"api_key\":\"business value\", \"future\": true }")},
		{"invalid-json", "application/json", "", []byte("{not parsed at all")},
		{"multipart", "multipart/form-data; boundary=opaque-boundary", "", []byte("--opaque-boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"synthetic.bin\"\r\n\r\n\x00\x01\xff\r\n--opaque-boundary--\r\n")},
		{"binary", "application/octet-stream", "", []byte{0, 255, 1, 0x1f, 0x8b, 2}},
		{"compressed", "application/octet-stream", "gzip", encoded.Bytes()},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if !bytes.Equal(body, test.body) {
					t.Error("request body changed")
				}
				if r.RequestURI != "/root/v1/custom/a%20b?key=business&a=1&a=2&space=+&space=%20&slash=%2f" {
					t.Errorf("raw URI changed: %s", r.RequestURI)
				}
				if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" || r.Header.Get("X-Provider-Key") == "spoofed" || !strings.HasPrefix(r.Header.Get("X-Provider-Key"), "Token managed-key-") {
					t.Error("credential substitution failed")
				}
				if r.Header.Get("Content-Type") != test.contentType || r.Header.Get("Content-Encoding") != test.encoding || r.Header.Get("Accept-Encoding") != "" {
					t.Error("representation request headers changed")
				}
				w.Header().Set("Content-Type", test.contentType)
				if test.encoding != "" {
					w.Header().Set("Content-Encoding", test.encoding)
				}
				w.Header().Add("X-Vendor", "one")
				w.Header().Add("X-Vendor", "two")
				w.WriteHeader(422)
				_, _ = w.Write(test.body)
			}))
			defer server.Close()
			_, engine, sink := transparentFixture(t, server.URL+"/root", "rotate")
			response := transparentCall(engine, "POST", "/proxy/1/v1/custom/a%20b?key=business&key=gl-client&a=1&a=2&space=+&space=%20&slash=%2f", test.body, http.Header{"Content-Type": {test.contentType}, "Content-Encoding": {test.encoding}, "X-Provider-Key": {"spoofed"}, "X-Api-Key": {"gl-client"}})
			if response.Code != 422 || !bytes.Equal(response.Body.Bytes(), test.body) || response.Header().Get("Content-Encoding") != test.encoding || len(response.Header().Values("X-Vendor")) != 2 {
				t.Fatalf("response changed: status=%d body=%q", response.Code, response.Body.Bytes())
			}
			events := sink.snapshot()
			if len(events) != 1 || len(events[0].Attempts) != 1 || events[0].Attempts[0].WillRetry {
				t.Fatal("missing single-attempt request diagnostics")
			}
			data, _ := json.Marshal(events)
			if bytes.Contains(data, []byte("business value")) || bytes.Contains(data, []byte("not parsed at all")) {
				t.Fatal("opaque payload reached logs")
			}
		})
	}
}

func TestTransparentNeverReplaysOrFollowsRedirects(t *testing.T) {
	for _, status := range []int{302, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls, redirects atomic.Int32
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirects.Add(1); w.WriteHeader(200) }))
			defer other.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", other.URL+"/must-not-receive-key")
				w.WriteHeader(status)
				_, _ = w.Write([]byte("native error remains unchanged"))
			}))
			defer server.Close()
			_, engine, _ := transparentFixture(t, server.URL, "rotate")
			response := transparentCall(engine, "POST", "/proxy/1/tasks", []byte("opaque task"), nil)
			if response.Code != status || response.Body.String() != "native error remains unchanged" || calls.Load() != 1 || redirects.Load() != 0 {
				t.Fatalf("status=%d calls=%d redirects=%d", response.Code, calls.Load(), redirects.Load())
			}
		})
	}
}

func TestTransparentStreamsWithoutSSEParsingAndCancelsUpstream(t *testing.T) {
	for _, contentType := range []string{"text/event-stream", "application/octet-stream"} {
		t.Run(contentType, func(t *testing.T) {
			canceled := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				_, _ = w.Write([]byte("opaque-first-chunk\n"))
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				close(canceled)
			}))
			defer upstream.Close()
			h, engine, _ := transparentFixture(t, upstream.URL, "rotate")
			gateway := httptest.NewServer(engine)
			defer gateway.Close()
			request, _ := http.NewRequest("POST", gateway.URL+"/proxy/1/stream", strings.NewReader("opaque"))
			request.Header.Set("Authorization", "Bearer gl-client")
			client := &http.Client{Timeout: 3 * time.Second}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			line, err := bufio.NewReader(response.Body).ReadString('\n')
			if err != nil || line != "opaque-first-chunk\n" {
				t.Fatalf("first streaming bytes delayed or changed: %q %v", line, err)
			}
			_ = response.Body.Close()
			select {
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("downstream close did not cancel upstream")
			}
			deadline := time.Now().Add(time.Second)
			for h.manager.Concurrency().Snapshot().Global != 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if h.manager.Concurrency().Snapshot().Global != 0 {
				t.Fatal("request concurrency was not released")
			}
		})
	}
}

func TestTransparentScopeAndFixedCredentialDoNotEscape(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("X-Provider-Key") != "Token managed-key-two" {
			t.Error("fixed source changed credential")
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	h, engine, _ := transparentFixture(t, server.URL, "fixed")
	for _, path := range []string{"/proxy/2/v1/files", "/proxy/1/../admin", "/proxy/1/%2e%2e/admin", "/proxy/1/v1%2ffiles", "/proxy/1/%252e%252e/admin", "/proxy/1/http://other.example", "http://gateway.invalid/proxy/1/v1/files"} {
		response := transparentCall(engine, "POST", path, nil, nil)
		if response.Code < 400 {
			t.Fatalf("unsafe path accepted: %s", path)
		}
	}
	for _, method := range []string{"CONNECT", "TRACE"} {
		if response := transparentCall(engine, method, "/proxy/1/v1/files", nil, nil); response.Code != 405 {
			t.Fatalf("method %s=%d", method, response.Code)
		}
	}
	if response := transparentCall(engine, "GET", "/proxy/1/ws", nil, http.Header{"Connection": {"Upgrade"}, "Upgrade": {"websocket"}}); response.Code != 405 {
		t.Fatal("WebSocket accepted")
	}
	for i := 0; i < 2; i++ {
		if response := transparentCall(engine, "POST", "/proxy/1/tasks", nil, nil); response.Code != 200 {
			t.Fatalf("fixed request=%d %s", response.Code, response.Body.String())
		}
	}
	h.registry.SetCooldownWithChange(2, time.Now().Add(time.Minute))
	if response := transparentCall(engine, "POST", "/proxy/1/tasks", nil, nil); response.Code != 503 {
		t.Fatalf("fixed cooldown=%d", response.Code)
	}
	if calls.Load() != 2 {
		t.Fatalf("unexpected upstream calls=%d", calls.Load())
	}
	key := h.manager.Current().AccessKeysByID[1]
	key.Filters.Models = map[string]struct{}{"restricted-model": {}}
	h.manager.Current().AccessKeysByHash[h.encryption.Hash("gl-client")] = key
	if response := transparentCall(engine, "POST", "/proxy/1/tasks", []byte("unknown model"), nil); response.Code != 403 {
		t.Fatal("model-restricted key entered opaque route")
	}
}

func TestTransparentRequiresExplicitGrantAndHeaderAuthentication(t *testing.T) {
	h, engine, _ := transparentFixture(t, "http://127.0.0.1:1", "rotate")
	r := httptest.NewRequest("GET", "/proxy/1/files?key=gl-client", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("query-only auth accepted")
	}
	key := h.manager.Current().AccessKeysByID[1]
	key.Filters.Protocols = map[protocol.Protocol]struct{}{protocol.OpenAICompletions: {}}
	h.manager.Current().AccessKeysByHash[h.encryption.Hash("gl-client")] = key
	if response := transparentCall(engine, "GET", "/proxy/1/files", nil, nil); response.Code != 403 {
		t.Fatal("existing protocol restriction bypassed")
	}
	key.Filters.Protocols = nil
	h.manager.Current().AccessKeysByHash[h.encryption.Hash("gl-client")] = key
	if response := transparentCall(engine, "GET", "/proxy/1/files", nil, nil); response.Code != 403 {
		t.Fatal("empty legacy protocol scope implicitly permitted transparent forwarding")
	}
	key.Filters.Protocols = map[protocol.Protocol]struct{}{protocol.TransparentHTTP: {}}
	key.Filters.Transparent = &state.TransparentScope{GroupIDs: []uint{1}, Methods: []string{"GET"}, Paths: []state.TransparentPathRule{{Path: "/v1/files", Match: "subtree"}}}
	h.manager.Current().AccessKeysByHash[h.encryption.Hash("gl-client")] = key
	if response := transparentCall(engine, "POST", "/proxy/1/v1/files", nil, nil); response.Code != 403 {
		t.Fatal("method restriction bypassed")
	}
	if response := transparentCall(engine, "GET", "/proxy/1/v1/filessibling", nil, nil); response.Code != 403 {
		t.Fatal("path segment boundary bypassed")
	}
	key.Filters.Transparent = nil
	h.manager.Current().AccessKeysByHash[h.encryption.Hash("gl-client")] = key
	if response := transparentCall(engine, "GET", "/proxy/1/files", nil, nil); response.Code != 403 {
		t.Fatal("legacy unrestricted key received implicit transparent grant")
	}
}

func TestTransparentDoesNotReplayAmbiguousEmptyIdempotencyPOST(t *testing.T) {
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			_, _ = w.Write([]byte("warm"))
			return
		}
		posts.Add(1)
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = connection.Close()
	}))
	defer server.Close()
	_, engine, _ := transparentFixture(t, server.URL, "rotate")
	if response := transparentCall(engine, "GET", "/proxy/1/warm", nil, nil); response.Code != 200 {
		t.Fatal("warmup failed")
	}
	response := transparentCall(engine, "POST", "/proxy/1/task", nil, http.Header{"Idempotency-Key": {"synthetic-task"}})
	if response.Code != 502 || posts.Load() != 1 {
		t.Fatalf("empty POST replayed after ambiguous acceptance: status=%d calls=%d", response.Code, posts.Load())
	}
}

func TestTransparentProtectsCredentialHeaders(t *testing.T) {
	for _, name := range []string{"Location", "Set-Cookie", "WWW-Authenticate"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Debug-Access") != "" {
					t.Error("gateway credential leaked in unrelated request header")
				}
				secret := strings.TrimPrefix(r.Header.Get("X-Provider-Key"), "Token ")
				value := "secret=" + secret
				if name == "Location" {
					value = "https://untrusted.example/?key=" + url.QueryEscape(secret)
				}
				w.Header().Set(name, value)
				_, _ = w.Write([]byte("upstream-body-must-not-be-sent"))
			}))
			defer server.Close()
			_, engine, sink := transparentFixture(t, server.URL, "fixed")
			response := transparentCall(engine, "GET", "/proxy/1/file", nil, http.Header{"X-Debug-Access": {"Bearer gl-client"}})
			if response.Code != 502 || strings.Contains(response.Body.String(), "upstream-body-must-not-be-sent") || response.Header().Get(name) != "" {
				t.Fatalf("unsafe upstream header/body forwarded: %d %s", response.Code, response.Body.String())
			}
			data, _ := json.Marshal(sink.snapshot())
			if bytes.Contains(data, []byte("managed-key")) || bytes.Contains(data, []byte("gl-client")) {
				t.Fatal("credential echo reached request logs")
			}
		})
	}
}

func TestTransparentHTTPFailuresRespectExplicitPolicyWithoutNativeCredentialDamage(t *testing.T) {
	for _, test := range []struct {
		name     string
		status   int
		explicit bool
	}{
		{"business-unauthorized", 401, false},
		{"business-forbidden", 403, false},
		{"administrator-rule", 401, true},
		{"temporary-rate-limit", 429, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "5")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte("private business authentication error"))
			}))
			defer server.Close()
			h, engine, sink := transparentFixture(t, server.URL, "fixed")
			if test.explicit {
				rules, err := health.CompileErrorRules([]health.ErrorRule{{StatusCodes: []int{401}, Retry: health.RetryNone, Effect: health.EffectRecordCredentialFailure}}, "group")
				if err != nil {
					t.Fatal(err)
				}
				group := h.manager.Current().Groups[1]
				group.ErrorRules = rules
				h.manager.Current().Groups[1] = group
			}
			response := transparentCall(engine, "POST", "/proxy/1/business-login", []byte("synthetic"), nil)
			if response.Code != test.status {
				t.Fatalf("status=%d", response.Code)
			}
			entries, err := h.registry.(*state.CredentialRegistry).SnapshotGroupCredentialEntriesExact(1, []uint{2})
			if err != nil {
				t.Fatal(err)
			}
			entry := entries[0]
			if test.explicit {
				if entry.FailureCount != 1 {
					t.Fatalf("explicit rule did not apply: failures=%d", entry.FailureCount)
				}
			} else if test.status == 429 {
				if !entry.CooldownUntil.After(time.Now()) || entry.Blacklisted || entry.FailureCount != 0 {
					t.Fatal("rate limit did not apply a temporary cooldown only")
				}
			} else {
				if entry.FailureCount != 0 || entry.Blacklisted || entry.Status != state.CredentialStatusActive || !entry.CooldownUntil.IsZero() {
					t.Fatal("business status damaged shared native credential")
				}
				model := "embed-synthetic"
				selection, selectErr := scheduler.New(h.manager.Current(), h.registry, scheduler.Query{ClientProtocol: protocol.CohereEmbeddings, Operation: execution.OperationEmbeddingsCreate, ExternalModel: &model,
					AccessKey: state.AccessKeyView{Status: state.AccessKeyStatusActive}, AllowedCredentialIDs: map[uint]struct{}{2: {}}}).Next()
				if selectErr != nil || selection.CredentialID != 2 {
					t.Fatalf("native candidate was affected: %v", selectErr)
				}
			}
			logs := sink.snapshot()
			if len(logs) != 1 || len(logs[0].Attempts) != 1 || logs[0].Attempts[0].WillRetry {
				t.Fatal("transparent error replayed")
			}
			data, _ := json.Marshal(logs)
			if bytes.Contains(data, []byte("private business")) {
				t.Fatal("opaque error body reached logs")
			}
		})
	}
}

func TestTransparentBlocksCredentialTrailersAfterStreaming(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Trailer", "X-Result")
		_, _ = w.Write([]byte("opaque-safe-chunk"))
		w.(http.Flusher).Flush()
		w.Header().Set("X-Result", "managed-key-two")
	}))
	defer upstream.Close()
	_, engine, _ := transparentFixture(t, upstream.URL, "fixed")
	gateway := httptest.NewServer(engine)
	defer gateway.Close()
	request, _ := http.NewRequest("GET", gateway.URL+"/proxy/1/file", nil)
	request.Header.Set("Authorization", "Bearer gl-client")
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err == nil || string(body) != "opaque-safe-chunk" || response.Trailer.Get("X-Result") != "" {
		t.Fatal("unsafe trailer was forwarded or the truncated response appeared complete")
	}
}

func TestTransparentChecksInterimHeadersWithoutChangingFinalResponse(t *testing.T) {
	for _, echo := range []bool{false, true} {
		name := "safe-early-hints"
		if echo {
			name = "credential-early-hints"
		}
		t.Run(name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Link", "</synthetic.css>; rel=preload")
				if echo {
					w.Header().Set("Set-Cookie", "secret=managed-key-two")
				}
				w.WriteHeader(http.StatusEarlyHints)
				w.Header().Del("Link")
				w.Header().Del("Set-Cookie")
				w.Header().Set("X-Final", "preserved")
				_, _ = w.Write([]byte("opaque-final-body"))
			}))
			defer upstream.Close()
			_, engine, sink := transparentFixture(t, upstream.URL, "fixed")
			gateway := httptest.NewServer(engine)
			defer gateway.Close()
			var interim atomic.Int32
			request, _ := http.NewRequest("GET", gateway.URL+"/proxy/1/file", nil)
			request.Header.Set("Authorization", "Bearer gl-client")
			request = request.WithContext(httptrace.WithClientTrace(request.Context(), &httptrace.ClientTrace{Got1xxResponse: func(_ int, _ textproto.MIMEHeader) error { interim.Add(1); return nil }}))
			response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || interim.Load() != 0 {
				t.Fatal("interim response escaped gateway policy")
			}
			if echo {
				if response.StatusCode != 502 || response.Header.Get("Set-Cookie") != "" || bytes.Contains(body, []byte("opaque-final-body")) {
					t.Fatal("credential in interim headers was not rejected")
				}
			} else if response.StatusCode != 200 || string(body) != "opaque-final-body" || response.Header.Get("X-Final") != "preserved" {
				t.Fatal("safe interim response changed the final response")
			}
			logs, _ := json.Marshal(sink.snapshot())
			if bytes.Contains(logs, []byte("managed-key-two")) {
				t.Fatal("interim credential echo reached logs")
			}
		})
	}
}
