package gateway

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"gpt-load/internal/channel"
	"gpt-load/internal/dialect"
	"gpt-load/internal/state"
)

type memoryResourceBindingStore struct {
	mu           sync.Mutex
	rows         map[string]state.ResponseBinding
	saveErr      error
	loadErr      error
	loads        int
	claimStarted chan struct{}
	claimRelease chan struct{}
	loadHook     func()
}

func (store *memoryResourceBindingStore) Claim(_ context.Context, binding state.ResponseBinding) (state.ResponseBinding, error) {
	if store.claimStarted != nil {
		close(store.claimStarted)
		<-store.claimRelease
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.saveErr != nil {
		return state.ResponseBinding{}, store.saveErr
	}
	if store.rows == nil {
		store.rows = map[string]state.ResponseBinding{}
	}
	if existing, found := store.rows[binding.ResponseID]; found && existing.ExpiresAt.After(time.Now()) {
		if existing.AccessKeyID != binding.AccessKeyID || existing.GroupID != binding.GroupID || existing.CredentialID != binding.CredentialID || existing.IdentityGeneration != binding.IdentityGeneration {
			return state.ResponseBinding{}, state.ErrResourceBindingConflict
		}
		return existing, nil
	}
	store.rows[binding.ResponseID] = binding
	return binding, nil
}

func (store *memoryResourceBindingStore) Load(_ context.Context, accessKeyID uint, key string) (state.ResponseBinding, bool, error) {
	if store.loadHook != nil {
		store.loadHook()
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.loads++
	if store.loadErr != nil {
		return state.ResponseBinding{}, false, store.loadErr
	}
	binding, found := store.rows[key]
	if !found || binding.AccessKeyID != accessKeyID {
		return state.ResponseBinding{}, false, nil
	}
	return binding, true, nil
}

func TestResourceBindingClaimKeepsDurableOwnerAndExpiry(t *testing.T) {
	key := dialect.NativeResourceKey("mistral", "files", "synthetic-owned-file")
	owner := state.ResponseBinding{AccessKeyID: 1, ResponseID: key, GroupID: 1, CredentialID: 1, IdentityGeneration: 1, ExpiresAt: time.Now().Add(time.Hour)}
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-owner-keeps-expiry", true: "conflicting-owner-rejected"}[conflict], func(t *testing.T) {
			h, _, _ := newHandlerForTest(t, &scriptedForwarder{})
			h.SetResourceBindingStore(&memoryResourceBindingStore{rows: map[string]state.ResponseBinding{key: owner}})
			ref := state.CredentialRef{ID: 1, GroupID: 1, IdentityGeneration: 1}
			if conflict {
				ref.ID, ref.IdentityGeneration = 2, 2
			}
			if h.recordResourceBinding(1, key, ref) == conflict {
				t.Fatal("unexpected durable ownership decision")
			}
			cached, found := h.responseBindings.Lookup(1, key)
			if conflict && found {
				t.Fatal("rejected ownership was published to the cache")
			}
			if !conflict && (!found || !cached.ExpiresAt.Equal(owner.ExpiresAt)) {
				t.Fatal("cache extended the durable ownership expiry")
			}
			loaded, found, err := h.lookupResourceBinding(t.Context(), 1, key)
			if err != nil || !found || loaded.CredentialID != owner.CredentialID || !loaded.ExpiresAt.Equal(owner.ExpiresAt) {
				t.Fatal("durable owner changed")
			}
		})
	}
}

func TestResourceBindingDoesNotPublishUnconfirmedOwner(t *testing.T) {
	h, _, _ := newHandlerForTest(t, &scriptedForwarder{})
	started, release := make(chan struct{}), make(chan struct{})
	h.SetResourceBindingStore(&memoryResourceBindingStore{claimStarted: started, claimRelease: release})
	key := dialect.NativeResourceKey("mistral", "files", "synthetic-pending-file")
	done := make(chan bool, 1)
	go func() {
		done <- h.recordResourceBinding(1, key, state.CredentialRef{ID: 1, GroupID: 1, IdentityGeneration: 1})
	}()
	<-started
	_, visible := h.responseBindings.Lookup(1, key)
	close(release)
	if !<-done {
		t.Fatal("confirmed owner could not be recorded")
	}
	if visible {
		t.Fatal("owner was visible before the durable claim completed")
	}
}

func TestResourceBindingExistingCacheAdoptsShorterDurableExpiry(t *testing.T) {
	h, _, _ := newHandlerForTest(t, &scriptedForwarder{})
	key := dialect.NativeResourceKey("mistral", "files", "synthetic-existing-cache")
	owner := state.ResponseBinding{AccessKeyID: 1, ResponseID: key, GroupID: 1, CredentialID: 1, IdentityGeneration: 1, ExpiresAt: time.Now().Add(time.Hour)}
	fallback := owner
	fallback.ExpiresAt = owner.ExpiresAt.Add(24 * time.Hour)
	if !h.responseBindings.Remember(fallback) {
		t.Fatal("fixture owner could not be cached")
	}
	h.SetResourceBindingStore(&memoryResourceBindingStore{rows: map[string]state.ResponseBinding{key: owner}})
	if !h.recordResourceBinding(1, key, state.CredentialRef{ID: 1, GroupID: 1, IdentityGeneration: 1}) {
		t.Fatal("same durable owner was rejected")
	}
	cached, found := h.responseBindings.Lookup(1, key)
	if !found || !cached.ExpiresAt.Equal(owner.ExpiresAt) {
		t.Fatal("cached fallback outlived the confirmed durable ownership")
	}
}

func TestResourceBindingLookupRejectsCacheConflict(t *testing.T) {
	h, _, _ := newHandlerForTest(t, &scriptedForwarder{})
	key := dialect.NativeResourceKey("mistral", "files", "synthetic-racing-file")
	owner := state.ResponseBinding{AccessKeyID: 1, ResponseID: key, GroupID: 1, CredentialID: 1, IdentityGeneration: 1, ExpiresAt: time.Now().Add(time.Hour)}
	h.SetResourceBindingStore(&memoryResourceBindingStore{rows: map[string]state.ResponseBinding{key: owner}, loadHook: func() {
		h.responseBindings.Record(1, key, state.CredentialRef{ID: 2, GroupID: 1, IdentityGeneration: 2})
	}})
	if _, found, err := h.lookupResourceBinding(t.Context(), 1, key); found || err == nil {
		t.Fatal("rejected cache ownership was still returned as usable")
	}
}

func TestResourceBindingWriteFailureKeepsOwnerWithoutLoggingDatabaseDetails(t *testing.T) {
	h, _, _ := newHandlerForTest(t, &scriptedForwarder{})
	var output bytes.Buffer
	h.logger = logrus.New()
	h.logger.SetOutput(&output)
	h.SetResourceBindingStore(&memoryResourceBindingStore{saveErr: errors.New("synthetic-private-database-details")})
	key := dialect.NativeResourceKey("mistral", "files", "synthetic-unsaved-file")
	if !h.recordResourceBinding(1, key, state.CredentialRef{ID: 1, GroupID: 1, IdentityGeneration: 1}) {
		t.Fatal("database outage discarded the accepted resource owner")
	}
	if _, found := h.responseBindings.Lookup(1, key); !found {
		t.Fatal("accepted resource owner is missing")
	}
	if strings.Contains(output.String(), "synthetic-private-database-details") || !strings.Contains(output.String(), "resource_binding.persist_failed") {
		t.Fatal("persistence diagnostics contain raw database details or omit the failure stage")
	}
}

func TestNativeResourceOwnershipSurvivesLostMemoryBindings(t *testing.T) {
	tests := []struct {
		name       string
		saveErr    error
		loadErr    error
		wantOCR    int
		wantCalls  int
		wantStored bool
		wantUpload int
	}{
		{name: "persisted", wantOCR: 200, wantCalls: 2, wantStored: true},
		{name: "store unavailable on read", loadErr: errors.New("database is down"), wantOCR: 503, wantCalls: 1, wantStored: true},
		{name: "store unavailable on write", saveErr: errors.New("database is down"), wantOCR: 404, wantCalls: 1},
		{name: "durable ownership conflict is not replayed", saveErr: state.ErrResourceBindingConflict, wantUpload: 502, wantCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var credentials []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				credentials = append(credentials, r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/files":
					_, _ = w.Write([]byte(`{"id":"ocr-persisted-synthetic"}`))
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
			store := &memoryResourceBindingStore{saveErr: test.saveErr}
			handler.SetResourceBindingStore(store)
			send := func(method, path, contentType, body string) *httptest.ResponseRecorder {
				request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
				request.Header.Set("Authorization", "Bearer gl-client")
				request.Header.Set("Content-Type", contentType)
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				return response
			}

			upload := send("POST", "/native/mistral/v1/files", "multipart/form-data; boundary=test", "--test\r\nContent-Disposition: form-data; name=\"purpose\"\r\n\r\nocr\r\n--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"synthetic.pdf\"\r\nContent-Type: application/pdf\r\n\r\nsynthetic PDF fixture\r\n--test--\r\n")
			wantUpload := test.wantUpload
			if wantUpload == 0 {
				wantUpload = 200
			}
			if upload.Code != wantUpload {
				t.Fatalf("upload status=%d body=%s", upload.Code, upload.Body.String())
			}
			if wantUpload != 200 {
				if len(credentials) != test.wantCalls {
					t.Fatal("accepted resource creation was replayed after an ownership conflict")
				}
				if _, found := handler.responseBindings.Lookup(1, dialect.NativeResourceKey("mistral", "files", "ocr-persisted-synthetic")); found {
					t.Fatal("conflicting owner remained cached")
				}
				return
			}
			if _, stored := store.rows[dialect.NativeResourceKey("mistral", "files", "ocr-persisted-synthetic")]; stored != test.wantStored {
				t.Fatalf("persisted = %t, want %t", stored, test.wantStored)
			}

			// A restart or LRU eviction drops the in-memory owner.
			handler.responseBindings = state.NewResponseBindings()
			store.loadErr = test.loadErr
			response := send("POST", "/v1/ocr", "application/json", `{"model":"gpt-4o","document":{"type":"file","file_id":"ocr-persisted-synthetic"}}`)
			if response.Code != test.wantOCR || len(credentials) != test.wantCalls {
				t.Fatalf("OCR status=%d upstream_calls=%d body=%s", response.Code, len(credentials), response.Body.String())
			}
			if test.wantOCR == 200 && credentials[0] != credentials[1] {
				t.Fatal("OCR did not stay on the creating credential")
			}
			if test.wantOCR != 200 {
				return
			}
			loads := store.loads
			if again := send("POST", "/v1/ocr", "application/json", `{"model":"gpt-4o","document":{"type":"file","file_id":"ocr-persisted-synthetic"}}`); again.Code != 200 || store.loads != loads {
				t.Fatalf("restored owner was not cached in memory: status=%d store_loads=%d->%d", again.Code, loads, store.loads)
			}
		})
	}
}

func TestResourceBindingKeysMatchNativeResourceKeys(t *testing.T) {
	if !state.IsResourceBindingKey(dialect.NativeResourceKey("mistral", "files", "f")) {
		t.Fatal("native resource keys are not recognized as persistent resource bindings")
	}
	if state.IsResourceBindingKey("resp_abc") {
		t.Fatal("Responses continuation IDs must stay in memory")
	}
}
