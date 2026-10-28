package gateway

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/app"
	"gpt-load/internal/channel"
	"gpt-load/internal/dialect"
	"gpt-load/internal/execution"
	"gpt-load/internal/parameteroverride"
	"gpt-load/internal/state"
	"gpt-load/internal/telemetry"
)

func TestResponsesContinuationPinsCredentialWithoutSoftAffinity(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			forwarder := &scriptedForwarder{results: []UpstreamResult{
				storedResponse("first"), storedResponse("second"), storedResponse("third"),
			}}
			handler, engine, sink := newContinuationFixture(t, forwarder)
			group := handler.manager.Current().Groups[1]
			group.AffinityEnabled = enabled
			handler.manager.Current().Groups[1] = group
			handler.manager.Current().Settings.AffinityEnabled = enabled

			serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial","store":true}`, http.StatusOK)
			serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"continue","previous_response_id":"first","prompt_cache_key":"other-cache"}`, http.StatusOK)
			serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"continue"}`, http.StatusOK)

			assertAffinityAttemptKeys(t, forwarder.inputs, []string{"sk-one", "sk-one", "sk-two"})
			assertAffinityHits(t, sink.snapshot(), []bool{false, true, false})
			if sink.snapshot()[1].AffinityKind != telemetry.AffinityResponseContinuity {
				t.Fatal("missing continuation kind")
			}
			if got := handler.registry.SchedulingState().CaptureCheckpoint().Sequence; got != 3 {
				t.Fatalf("scheduling allocations = %d, want 3", got)
			}
		})
	}
}

func TestResponsesContinuationUsesNativeStorageCapabilities(t *testing.T) {
	for _, channelID := range []channel.ID{
		channel.OpenAI, channel.GPTLoad, channel.XAI, channel.NewAPI, channel.CLIProxyAPI, channel.Sub2API,
	} {
		t.Run(string(channelID), func(t *testing.T) {
			type observedRequest struct {
				PreviousResponseID string `json:"previous_response_id"`
				Store              *bool  `json:"store"`
				authorization      string
			}
			requests := make(chan observedRequest, 8)
			var count atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				var observed observedRequest
				if err := json.NewDecoder(request.Body).Decode(&observed); err != nil {
					t.Error(err)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				observed.authorization = request.Header.Get("Authorization")
				requests <- observed
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write(storedResponse(fmt.Sprintf("response-%d", count.Add(1))).Body)
			}))
			defer server.Close()
			handler, engine, sink := newContinuationFixture(t, newTestExecutionForwarder(t))
			setContinuationChannel(t, handler, channelID, server.URL)
			serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial"}`, http.StatusOK)
			serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"response-1","input":"continue"}`, http.StatusOK)
			serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"response-2","input":"continue","store":false}`, http.StatusOK)
			if count.Load() != 3 {
				t.Fatalf("upstream attempts = %d, want 3", count.Load())
			}
			for index, id := range []string{"", "response-1", "response-2"} {
				observed := <-requests
				if observed.PreviousResponseID != id || observed.authorization != "Bearer sk-one" ||
					(index == 2 && (observed.Store == nil || *observed.Store)) {
					t.Fatalf("request %d lost continuation semantics: %+v", index, observed)
				}
			}
			assertAffinityHits(t, sink.snapshot(), []bool{false, true, true})
			if _, ok := handler.responseBindings.Lookup(1, "response-3"); ok {
				t.Fatal("store:false registered a new continuation ID")
			}
		})
	}
}

func TestResponsesContinuationRejectsUnknownAndOtherAccessKeyIDs(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{storedResponse("first")}}
	handler, engine, sink := newContinuationFixture(t, forwarder)
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial"}`, http.StatusOK)
	other := handler.manager.Current().AccessKeysByHash[handler.encryption.Hash("gl-client")]
	other.ID = 2
	handler.manager.Current().AccessKeysByHash[handler.encryption.Hash("gl-other")] = other
	handler.manager.Current().AccessKeysByID[other.ID] = other
	for _, request := range []struct{ key, id string }{{"gl-other", "first"}, {"gl-client", "unknown"}} {
		serveContinuation(t, engine, request.key, fmt.Sprintf(`{"model":"gpt-4o","input":"continue","previous_response_id":%q}`, request.id), http.StatusBadRequest)
	}
	if len(forwarder.inputs) != 1 {
		t.Fatalf("upstream attempts = %d, want only the initial request", len(forwarder.inputs))
	}
	assertAffinityHits(t, sink.snapshot(), []bool{false, false, false})
}

func TestResponsesContinuationUsesCurrentCandidateIntersection(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{storedResponse("first")}}
	handler, engine, sink := newContinuationFixture(t, forwarder)
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial"}`, http.StatusOK)
	if err := handler.registry.(*state.CredentialRegistry).SetCredentialStatus(1, state.CredentialStatusDisabled); err != nil {
		t.Fatal(err)
	}
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"continue","previous_response_id":"first"}`, http.StatusServiceUnavailable)
	if len(forwarder.inputs) != 1 {
		t.Fatal("continuation escaped to the other credential")
	}
	assertAffinityHits(t, sink.snapshot(), []bool{false, false})
}

func TestResponsesContinuationRegistersSSEBeforeDelivery(t *testing.T) {
	var engine *gin.Engine
	writer := httptest.NewRecorder()
	var credentials []uint
	created := "event: response.created\r\n" + `data: {"type":"response.created","response":{"id":"first","object":"response","store":true}}` + "\r\n\r\n"
	completed := "event: response.completed\r\n" + `data: {"type":"response.completed","response":{"id":"first","object":"response","store":true}}` + "\r\n\r\n"
	executor := fakeExecutionExecutor{
		unary: func(_ context.Context, spec execution.AttemptSpec) execution.AttemptResult {
			credentials = append(credentials, spec.Credential.ID)
			return execution.AttemptResult{
				StatusCode: http.StatusOK, DispatchState: execution.DispatchMaybeSent, ResponseStarted: true,
				Header: http.Header{"Content-Type": {"application/json"}}, Body: storedResponse("second").Body,
			}
		},
		stream: func(_ context.Context, spec execution.AttemptSpec, sink execution.StreamSink) execution.StreamResult {
			credentials = append(credentials, spec.Credential.ID)
			for _, event := range []execution.StreamEvent{
				{Sequence: 1, Kind: execution.StreamEventReady, StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}},
				{Sequence: 2, Kind: execution.StreamEventData, Data: []byte(created[:len(created)-4])},
			} {
				if err := sink(event); err != nil {
					t.Fatal(err)
				}
			}
			if writer.Body.Len() != 0 {
				t.Fatal("partial response ID was delivered before registration")
			}
			if err := sink(execution.StreamEvent{Sequence: 3, Kind: execution.StreamEventData, Data: []byte(created[len(created)-4:])}); err != nil {
				t.Fatal(err)
			}
			if writer.Body.String() != created {
				t.Fatalf("created event was not delivered intact: %q", writer.Body.String())
			}
			// 客户端收到 created 即可续接，无需等待原请求 completed。
			serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"first","input":"continue"}`, http.StatusOK)
			if err := sink(execution.StreamEvent{Sequence: 4, Kind: execution.StreamEventData, Data: []byte(completed)}); err != nil {
				t.Fatal(err)
			}
			return execution.StreamResult{StatusCode: http.StatusOK, DispatchState: execution.DispatchMaybeSent, ResponseStarted: true}
		},
	}
	handler, runtime, _ := newContinuationFixture(t, NewExecutionForwarder(executor))
	engine = runtime
	setContinuationChannel(t, handler, channel.NewAPI, "https://upstream.example")
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"gpt-4o","input":"initial","stream":true}`))
	request.Header.Set("Authorization", "Bearer gl-client")
	engine.ServeHTTP(writer, request)
	if writer.Body.String() != created+completed || fmt.Sprint(credentials) != "[1 1]" {
		t.Fatalf("SSE response = %q, credentials = %v", writer.Body.String(), credentials)
	}
}

func TestResponsesContinuationKeepsBindingAcrossModelCooldownAndConfigPublication(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{storedResponse("first"), storedResponse("second")}}
	handler, engine, sink := newContinuationFixture(t, forwarder)
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial"}`, http.StatusOK)
	ref, _ := handler.registry.CredentialRef(1)
	registry := handler.registry.(*state.CredentialRegistry)
	if ok, _ := registry.SetModelCooldown(ref, "gpt-4o", time.Now().Add(time.Hour), time.Now()); !ok {
		t.Fatal("set model cooldown failed")
	}
	response := serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"first","input":"continue"}`, http.StatusTooManyRequests)
	if response.Header().Get("Retry-After") == "" || len(forwarder.inputs) != 1 {
		t.Fatal("cooldown did not reuse existing candidate rejection")
	}
	registry.ClearModelCooldowns(1)
	handler.manager.Current().Revision++
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"first","input":"continue"}`, http.StatusOK)
	assertAffinityAttemptKeys(t, forwarder.inputs, []string{"sk-one", "sk-one"})
	assertAffinityHits(t, sink.snapshot(), []bool{false, false, true})
}

func TestResponsesContinuationFailureExhaustsBoundCandidate(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{
		storedResponse("first"),
		{StatusCode: http.StatusServiceUnavailable, Body: []byte(`{"error":{"message":"upstream unavailable"}}`)},
		storedResponse("must-not-retry"),
	}}
	handler, engine, sink := newContinuationFixture(t, forwarder)
	handler.manager.Current().Settings.RetryCount = 3
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial"}`, http.StatusOK)
	response := serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"first","input":"continue"}`, http.StatusServiceUnavailable)
	if response.Body.String() != string(forwarder.results[1].Body) {
		t.Fatal("the original upstream error was replaced")
	}
	assertAffinityAttemptKeys(t, forwarder.inputs, []string{"sk-one", "sk-one"})
	events := sink.snapshot()
	if len(events[1].Attempts) != 1 || events[1].Attempts[0].WillRetry || !events[1].AffinityHit {
		t.Fatalf("failure telemetry = %#v", events[1])
	}
}

func TestResponsesContinuationRejectsChangedCredentialIdentity(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{storedResponse("first")}}
	handler, engine, _ := newContinuationFixture(t, forwarder)
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial"}`, http.StatusOK)
	ref, _ := handler.registry.CredentialRef(1)
	registry := handler.registry.(*state.CredentialRegistry)
	if err := registry.ApplyCredentialImport(1, []state.CredentialEntry{{
		ID: 1, GroupID: 1, Status: state.CredentialStatusActive, Version: ref.Version + 1,
		IdentityGeneration: ref.IdentityGeneration + 10, Fingerprint: ref.Fingerprint, EncryptedValue: ref.EncryptedValue,
	}}); err != nil {
		t.Fatal(err)
	}
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"first","input":"continue"}`, http.StatusServiceUnavailable)
	if len(forwarder.inputs) != 1 {
		t.Fatal("old ID inherited a replacement identity")
	}
}

func TestResponsesContinuationDoesNotLearnUnstoredOrUnrelatedResponses(t *testing.T) {
	for _, test := range []struct{ request, response string }{
		{`{"model":"gpt-4o","input":"initial","store":false}`, string(storedResponse("first").Body)},
		{`{"model":"gpt-4o","input":"initial"}`, `{"id":"first","object":"response","store":false}`},
		{`{"model":"gpt-4o","input":"initial"}`, `{"id":"first","object":"chat.completion"}`},
	} {
		forwarder := &scriptedForwarder{results: []UpstreamResult{{StatusCode: http.StatusOK, Body: []byte(test.response)}}}
		_, engine, _ := newContinuationFixture(t, forwarder)
		serveContinuation(t, engine, "gl-client", test.request, http.StatusOK)
		serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"first","input":"continue"}`, http.StatusBadRequest)
		if len(forwarder.inputs) != 1 {
			t.Fatal("unavailable response ID reached an upstream")
		}
	}
}

func TestResponsesContinuationRejectsConflictingResponseBeforeDelivery(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{
		storedResponse("shared-id"), storedResponse("shared-id"), storedResponse("continued"),
	}}
	_, engine, _ := newContinuationFixture(t, forwarder)
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"first"}`, http.StatusOK)
	response := serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"independent"}`, http.StatusBadGateway)
	if bytes.Contains(response.Body.Bytes(), []byte("shared-id")) {
		t.Fatal("conflicting response ID was exposed")
	}
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"shared-id","input":"continue"}`, http.StatusOK)
	assertAffinityAttemptKeys(t, forwarder.inputs, []string{"sk-one", "sk-two", "sk-one"})
}

func TestResponsesContinuationRejectsLegacyOverrideBeforeForward(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{storedResponse("first")}}
	handler, engine, _ := newContinuationFixture(t, forwarder)
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial"}`, http.StatusOK)
	rules, err := parameteroverride.Compile([]any{map[string]any{"set": map[string]any{"previous_response_id": "wrong"}}})
	if err != nil {
		t.Fatal(err)
	}
	group := handler.manager.Current().Groups[1]
	group.ParameterOverrides = rules
	handler.manager.Current().Groups[1] = group
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"first","input":"continue"}`, http.StatusServiceUnavailable)
	if len(forwarder.inputs) != 1 {
		t.Fatal("legacy override dispatched a changed continuation")
	}
}

func TestResponsesContinuationResumesFromRuntimeCheckpoint(t *testing.T) {
	before, engine, _ := newContinuationFixture(t, &scriptedForwarder{results: []UpstreamResult{storedResponse("before-restart")}})
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial"}`, http.StatusOK)
	dir := t.TempDir()
	checkpoint := app.NewFileRuntimeStateCheckpoint(dir, nil, nil, before.responseBindings)
	if err := checkpoint.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	forwarder := &scriptedForwarder{results: []UpstreamResult{storedResponse("new-root"), storedResponse("continued")}}
	after, restarted, _ := newContinuationFixture(t, forwarder)
	if err := app.NewFileRuntimeStateCheckpoint(dir, nil, nil, after.responseBindings).Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	serveContinuation(t, restarted, "gl-client", `{"model":"gpt-4o","input":"another root"}`, http.StatusOK)
	serveContinuation(t, restarted, "gl-client", `{"model":"gpt-4o","previous_response_id":"before-restart","input":"continue"}`, http.StatusOK)
	assertAffinityAttemptKeys(t, forwarder.inputs, []string{"sk-one", "sk-one"})
}

func TestResponsesContinuationLearnsCompressedJSONResponse(t *testing.T) {
	var compressed bytes.Buffer
	zipper := gzip.NewWriter(&compressed)
	if _, err := zipper.Write(storedResponse("compressed-response").Body); err != nil {
		t.Fatal(err)
	}
	if err := zipper.Close(); err != nil {
		t.Fatal(err)
	}
	var credentials []uint
	executor := fakeExecutionExecutor{unary: func(_ context.Context, spec execution.AttemptSpec) execution.AttemptResult {
		credentials = append(credentials, spec.Credential.ID)
		return execution.AttemptResult{
			StatusCode: http.StatusOK, DispatchState: execution.DispatchMaybeSent, ResponseStarted: true,
			Header: http.Header{"Content-Type": {"application/json"}, "Content-Encoding": {"gzip"}},
			Body:   compressed.Bytes(),
		}
	}}
	_, engine, _ := newContinuationFixture(t, NewExecutionForwarder(executor))
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","input":"initial"}`, http.StatusOK)
	serveContinuation(t, engine, "gl-client", `{"model":"gpt-4o","previous_response_id":"compressed-response","input":"continue"}`, http.StatusOK)
	if fmt.Sprint(credentials) != "[1 1]" {
		t.Fatalf("compressed response resumed through credentials %v", credentials)
	}
}

func storedResponse(id string) UpstreamResult {
	return UpstreamResult{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       []byte(fmt.Sprintf(`{"id":%q,"object":"response","status":"completed","store":true,"output":[]}`, id)),
	}
}

func newContinuationFixture(t *testing.T, forwarder AttemptForwarder) (*Handler, *gin.Engine, *recordingRequestLogSink) {
	t.Helper()
	handler, _, _ := newHandlerForTest(t, forwarder, "sk-one", "sk-two")
	handler.dialects = dialect.NewSet(dialect.NewOpenAIResponses())
	sink := &recordingRequestLogSink{}
	handler.requestLogSink = sink
	engine := gin.New()
	bindGatewayRoutesForTest(t, engine, handler)
	return handler, engine, sink
}

func setContinuationChannel(t *testing.T, handler *Handler, channelID channel.ID, baseURL string) {
	t.Helper()
	credentials := make([]state.CredentialConfig, 0, 2)
	for _, id := range []uint{1, 2} {
		credentials = append(credentials, state.CredentialConfig{
			ID: id, GroupID: 1, Status: state.CredentialStatusActive,
			Version: 1, IdentityGeneration: uint64(id), Fingerprint: fmt.Sprintf("credential-%d", id),
		})
	}
	params, err := json.Marshal(map[string]string{"base_url": baseURL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handler.manager.Publish(state.CompileInput{
		ChannelRegistry: channel.NewRegistry(),
		Groups: []state.GroupConfig{{
			ID: 1, Name: string(channelID), ChannelID: channelID, ConnectionType: "api_key",
			Params: params,
			Models: []state.ModelConfig{{ID: "gpt-4o"}}, Enabled: true,
		}},
		Credentials: credentials,
		AccessKeys: []state.AccessKeyConfig{{
			ID: 1, Name: "client", KeyHash: handler.encryption.Hash("gl-client"), Status: state.AccessKeyStatusActive,
		}},
	}); err != nil {
		t.Fatal(err)
	}
}

func serveContinuation(t *testing.T, engine http.Handler, key, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
	return response
}

func TestResponsesPromptCacheKeyAffinityAndHardContinuationPriority(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{storedResponse("first"), storedResponse("second"), storedResponse("third"), storedResponse("fourth"), storedResponse("fifth")}}
	handler, engine, sink := newContinuationFixture(t, forwarder)
	group := handler.manager.Current().Groups[1]
	group.AffinityEnabled = true
	handler.manager.Current().Groups[1] = group
	for _, body := range []string{
		`{"model":"gpt-4o","input":"first-prefix","prompt_cache_key":"a"}`,
		`{"model":"gpt-4o","input":"changed-prefix","prompt_cache_key":"a"}`,
		`{"model":"gpt-4o","input":"changed-prefix","prompt_cache_key":"b"}`,
		`{"model":"gpt-4o","input":"continue","previous_response_id":"first","prompt_cache_key":"b"}`,
		`{"model":"gpt-4o","input":"another-prefix","prompt_cache_key":"b"}`,
	} {
		serveContinuation(t, engine, "gl-client", body, http.StatusOK)
	}
	assertAffinityAttemptKeys(t, forwarder.inputs, []string{"sk-one", "sk-one", "sk-two", "sk-one", "sk-two"})
	assertAffinityHits(t, sink.snapshot(), []bool{false, true, false, true, true})
	for i, want := range []string{"", telemetry.AffinityPromptCacheKey, "", telemetry.AffinityResponseContinuity, telemetry.AffinityPromptCacheKey} {
		if sink.snapshot()[i].AffinityKind != want {
			t.Fatalf("event %d kind=%q want=%q", i, sink.snapshot()[i].AffinityKind, want)
		}
	}
}

func TestNativeResourcesStayOnCreatingCredential(t *testing.T) {
	var headers []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = append(headers, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/files":
			w.WriteHeader(201)
			w.Write([]byte(`{"id":"file-synthetic"}`))
		case "/v1/transcriptions":
			w.WriteHeader(201)
			w.Write([]byte(`{"id":"task-synthetic","status":"queued"}`))
		case "/v1/transcriptions/task-synthetic":
			w.Write([]byte(`{"id":"task-synthetic","status":"completed"}`))
		case "/v1/transcriptions/task-synthetic/transcript":
			w.Write([]byte(`{"text":"synthetic audio"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	handler, engine, _ := newContinuationFixture(t, newTestExecutionForwarder(t))
	handler.dialects = dialect.NewSet(dialect.NewNativeServices())
	setContinuationChannel(t, handler, channel.Soniox, server.URL+"/v1")
	client := "gl-client"
	setFilter := func(enabled bool, model string) {
		key := handler.manager.Current().AccessKeysByID[1]
		key.Filters = state.FilterSet{AllowModelLessResources: enabled, Models: map[string]struct{}{model: {}}}
		handler.manager.Current().AccessKeysByID[1] = key
		handler.manager.Current().AccessKeysByHash[handler.encryption.Hash("gl-client")] = key
	}
	setFilter(false, "gpt-4o")
	send := func(method, path, contentType, body string, want int) {
		request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		request.Header.Set("Authorization", "Bearer "+client)
		request.Header.Set("Content-Type", contentType)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s %s = %d %s", method, path, response.Code, response.Body.String())
		}
	}
	send("POST", "/native/soniox/v1/files", "multipart/form-data; boundary=test", "--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"test.wav\"\r\n\r\nsynthetic\r\n--test--\r\n", 400)
	setFilter(true, "gpt-4o")
	send("POST", "/native/soniox/v1/files", "multipart/form-data; boundary=test", "--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"test.wav\"\r\n\r\nsynthetic\r\n--test--\r\n", 201)
	send("POST", "/native/soniox/v1/transcriptions", "application/json", `{"model":"forbidden","file_id":"file-synthetic"}`, 409)
	send("POST", "/native/soniox/v1/transcriptions", "application/json", `{"model":"gpt-4o","file_id":"file-synthetic"}`, 201)
	other := handler.manager.Current().AccessKeysByID[1]
	other.ID = 2
	handler.manager.Current().AccessKeysByID[2] = other
	handler.manager.Current().AccessKeysByHash[handler.encryption.Hash("gl-other")] = other
	client = "gl-other"
	send("GET", "/native/soniox/v1/transcriptions/task-synthetic", "", "", 404)
	client = "gl-client"
	setFilter(true, "new-model")
	send("POST", "/native/soniox/v1/transcriptions", "application/json", `{"model":"gpt-4o","file_id":"file-synthetic"}`, 409)
	send("GET", "/native/soniox/v1/transcriptions/task-synthetic", "", "", 200)
	send("GET", "/native/soniox/v1/transcriptions/task-synthetic/transcript", "", "", 200)
	send("GET", "/native/soniox/v1/transcriptions/unknown", "", "", 404)
	if len(headers) != 4 {
		t.Fatalf("upstream request count=%d", len(headers))
	}
	for _, value := range headers {
		if value != headers[0] {
			t.Fatal("resource changed credential")
		}
	}
}

func TestMistralOCRFileReferenceRequiresOwnerAndPinsCredential(t *testing.T) {
	for _, otherToken := range []bool{false, true} {
		name := "creating-credential"
		if otherToken {
			name = "cross-access-key"
		}
		t.Run(name, func(t *testing.T) {
			var credentials []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				credentials = append(credentials, r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/files":
					_, _ = w.Write([]byte(`{"id":"ocr-owned-synthetic"}`))
				case "/v1/ocr":
					_, _ = w.Write([]byte(`{"pages":[{"index":0,"markdown":"synthetic"}],"usage_info":{"pages_processed":1}}`))
				default:
					t.Errorf("unexpected upstream path %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			handler, engine, _ := newContinuationFixture(t, newTestExecutionForwarder(t))
			handler.dialects = dialect.NewSet(dialect.NewNativeServices(), dialect.NewMistral())
			setContinuationChannel(t, handler, channel.Mistral, server.URL+"/v1")
			send := func(token, method, path, contentType, body string) *httptest.ResponseRecorder {
				request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
				request.Header.Set("Authorization", "Bearer "+token)
				request.Header.Set("Content-Type", contentType)
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				return response
			}
			if response := send("gl-client", "POST", "/native/mistral/v1/files", "multipart/form-data; boundary=test", "--test\r\nContent-Disposition: form-data; name=\"purpose\"\r\n\r\nocr\r\n--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"synthetic.pdf\"\r\nContent-Type: application/pdf\r\n\r\nsynthetic PDF fixture\r\n--test--\r\n"); response.Code != 200 {
				t.Fatalf("upload status=%d body=%s", response.Code, response.Body.String())
			}
			token := "gl-client"
			if otherToken {
				other := handler.manager.Current().AccessKeysByID[1]
				other.ID = 2
				handler.manager.Current().AccessKeysByID[2] = other
				handler.manager.Current().AccessKeysByHash[handler.encryption.Hash("gl-other")] = other
				token = "gl-other"
				if response := send(token, "GET", "/native/mistral/v1/files/ocr-owned-synthetic", "", ""); response.Code != 404 {
					t.Fatalf("native resource control did not reject the other token: %d", response.Code)
				}
			}
			response := send(token, "POST", "/v1/ocr", "application/json", `{"model":"gpt-4o","document":{"type":"file","file_id":"ocr-owned-synthetic"}}`)
			if otherToken {
				if response.Code != 404 || len(credentials) != 1 {
					t.Fatalf("OCR file reference bypassed access-key ownership: status=%d upstream_calls=%d", response.Code, len(credentials))
				}
			} else if response.Code != 200 || len(credentials) != 2 || credentials[0] != credentials[1] {
				t.Fatalf("OCR did not stay on the creating credential: status=%d upstream_calls=%d credential_changed=%t", response.Code, len(credentials), len(credentials) == 2 && credentials[0] != credentials[1])
			}
		})
	}
}

func TestNativeVideoPollingPinsSubmitCredential(t *testing.T) {
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/video/submit" {
			w.Write([]byte(`{"requestId":"video-synthetic"}`))
		} else if r.URL.Path == "/v1/video/status" {
			w.Write([]byte(`{"status":"Succeed","results":{"videos":[{"url":"https://synthetic.example/video.mp4"}]}}`))
		} else {
			t.Error("unexpected video path")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	handler, engine, _ := newContinuationFixture(t, newTestExecutionForwarder(t))
	handler.dialects = dialect.NewSet(dialect.NewNativeServices())
	setContinuationChannel(t, handler, channel.SiliconFlow, server.URL+"/v1")
	for _, item := range []struct{ path, body string }{{"submit", `{"model":"gpt-4o","prompt":"synthetic red square","image_size":"1280x720"}`}, {"status", `{"requestId":"video-synthetic"}`}} {
		req := httptest.NewRequest("POST", "/native/siliconflow/v1/video/"+item.path, bytes.NewBufferString(item.body))
		req.Header.Set("Authorization", "Bearer gl-client")
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		if res.Code != 200 {
			t.Fatalf("video %s returned %d %s", item.path, res.Code, res.Body.String())
		}
	}
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Fatalf("video switched credentials: %v", keys)
	}
}
