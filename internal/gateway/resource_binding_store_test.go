package gateway

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"gpt-load/internal/channel"
	"gpt-load/internal/dialect"
	"gpt-load/internal/state"
)

type memoryResourceBindingStore struct {
	mu      sync.Mutex
	rows    map[string]state.ResponseBinding
	saveErr error
	loadErr error
	loads   int
}

func (store *memoryResourceBindingStore) Save(_ context.Context, binding state.ResponseBinding) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.saveErr != nil {
		return store.saveErr
	}
	if store.rows == nil {
		store.rows = map[string]state.ResponseBinding{}
	}
	store.rows[binding.ResponseID] = binding
	return nil
}

func (store *memoryResourceBindingStore) Load(_ context.Context, accessKeyID uint, key string) (state.ResponseBinding, bool, error) {
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

func TestNativeResourceOwnershipSurvivesLostMemoryBindings(t *testing.T) {
	tests := []struct {
		name       string
		saveErr    error
		loadErr    error
		wantOCR    int
		wantCalls  int
		wantStored bool
	}{
		{name: "persisted", wantOCR: 200, wantCalls: 2, wantStored: true},
		{name: "store unavailable on read", loadErr: errors.New("database is down"), wantOCR: 503, wantCalls: 1, wantStored: true},
		{name: "store unavailable on write", saveErr: errors.New("database is down"), wantOCR: 404, wantCalls: 1},
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
			if upload.Code != 200 {
				t.Fatalf("upload status=%d body=%s", upload.Code, upload.Body.String())
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
