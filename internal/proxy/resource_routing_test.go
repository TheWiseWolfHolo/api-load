package proxy

import (
	"api-load/internal/channel"
	"api-load/internal/encryption"
	"api-load/internal/failover"
	"api-load/internal/models"
	"api-load/internal/resourcepool"
	"api-load/internal/store"
	"api-load/internal/types"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type resourceRoutingChannel struct {
	*channel.BaseChannel
}

func TestBAT001BatchAndFilesStayOnCreatingPhysicalResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var failBatchQuery atomic.Bool
	var failFileUpload atomic.Bool
	var serverAHits atomic.Int64
	var serverBHits atomic.Int64
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverAHits.Add(1)
		if got := r.Header.Get("x-api-key"); got != "key-a" {
			t.Errorf("server A received crossed key %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/v1/files" && failFileUpload.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"uncertain upload failure"}}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/v1/batches/batch-1" && failBatchQuery.Load() {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"batch-1","object":"batch","input_file_id":"file-input","output_file_id":"file-output"}`))
	}))
	defer serverA.Close()
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverBHits.Add(1)
		if got := r.Header.Get("x-api-key"); got != "key-b" {
			t.Errorf("server B received crossed key %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/v1/files" && failFileUpload.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"uncertain upload failure"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"wrong-resource"}`))
	}))
	defer serverB.Close()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:proxy-batch-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ResourcePool{}, &models.UpstreamResource{}, &models.UpstreamObjectBinding{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool := models.ResourcePool{Name: "batch-shared", Strategy: "round_robin", AffinityTTLSeconds: 3600, BusyWaitMilliseconds: 0}
	if err := db.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	crypto, err := encryption.NewService("")
	if err != nil {
		t.Fatalf("create encryption service: %v", err)
	}
	provider := resourcepool.NewProvider(db, store.NewMemoryStore(), crypto)
	if err := provider.AddResources(pool.ID, []models.UpstreamResource{
		{Name: "a", UpstreamURL: serverA.URL, KeyValue: "key-a"},
		{Name: "b", UpstreamURL: serverB.URL, KeyValue: "key-b"},
	}); err != nil {
		t.Fatalf("add resources: %v", err)
	}
	if err := provider.LoadResourcesFromDB(); err != nil {
		t.Fatalf("load resources: %v", err)
	}
	var resourceA models.UpstreamResource
	if err := db.Where("resource_pool_id = ? AND name = ?", pool.ID, "a").First(&resourceA).Error; err != nil {
		t.Fatalf("load resource A: %v", err)
	}
	if err := provider.BindObject(context.Background(), models.UpstreamObjectBinding{
		GroupID: 1, ResourcePoolID: pool.ID, ResourceID: resourceA.ID,
		ObjectType: models.UpstreamObjectTypeFile, ObjectID: "file-input",
	}); err != nil {
		t.Fatalf("bind input file owner: %v", err)
	}

	matcher, err := failover.ParseStatusCodeMatcher("429,500-599")
	if err != nil {
		t.Fatalf("parse matcher: %v", err)
	}
	group := &models.Group{
		ID: 1, Name: "batch-route", ChannelType: "openai", ResourcePoolID: &pool.ID,
		FailoverStatusCodeMatcher: matcher,
		EffectiveConfig:           types.SystemSettings{MaxRetries: 2, RequestTimeout: 5},
	}
	proxyServer := &ProxyServer{resourceProvider: provider, encryptionSvc: crypto}
	channelHandler := &resourceRoutingChannel{BaseChannel: &channel.BaseChannel{
		Name: "openai", HTTPClient: &http.Client{Timeout: 5 * time.Second}, StreamClient: &http.Client{},
	}}

	createBody := []byte(`{"input_file_id":"file-input","endpoint":"/v1/responses","completion_window":"24h"}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/proxy/batch-route/v1/batches", bytes.NewReader(createBody))
	createRecorder := httptest.NewRecorder()
	createCtx, _ := gin.CreateTestContext(createRecorder)
	createCtx.Request = createRequest
	createCtx.Params = gin.Params{{Key: "group_name", Value: group.Name}, {Key: "path", Value: "/v1/batches"}}
	createRouting, apiErr := proxyServer.resolveUpstreamObjectRouting(createCtx, group, group, createBody)
	if apiErr != nil {
		t.Fatalf("resolve batch create owner: %v", apiErr)
	}
	if createRouting.ForcedResourceID != resourceA.ID {
		t.Fatalf("batch create did not follow input file owner: %#v", createRouting)
	}
	createCtx.Set(upstreamObjectRoutingContextKey, createRouting)
	proxyServer.executeRequestWithRetry(createCtx, channelHandler, group, group, createBody, false, time.Now(), 0, nil)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("batch create failed: status=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	batchBinding, err := provider.FindObjectBinding(context.Background(), group.ID, models.UpstreamObjectTypeBatch, "batch-1")
	if err != nil || batchBinding.ResourceID != resourceA.ID {
		t.Fatalf("batch owner was not persisted: %#v %v", batchBinding, err)
	}
	outputBinding, err := provider.FindObjectBinding(context.Background(), group.ID, models.UpstreamObjectTypeFile, "file-output")
	if err != nil || outputBinding.ResourceID != resourceA.ID {
		t.Fatalf("batch output file owner was not persisted: %#v %v", outputBinding, err)
	}

	failBatchQuery.Store(true)
	queryRequest := httptest.NewRequest(http.MethodGet, "/proxy/batch-route/v1/batches/batch-1", nil)
	queryRecorder := httptest.NewRecorder()
	queryCtx, _ := gin.CreateTestContext(queryRecorder)
	queryCtx.Request = queryRequest
	queryCtx.Params = gin.Params{{Key: "group_name", Value: group.Name}, {Key: "path", Value: "/v1/batches/batch-1"}}
	queryRouting, apiErr := proxyServer.resolveUpstreamObjectRouting(queryCtx, group, group, nil)
	if apiErr != nil {
		t.Fatalf("resolve batch query owner: %v", apiErr)
	}
	queryCtx.Set(upstreamObjectRoutingContextKey, queryRouting)
	proxyServer.executeRequestWithRetry(queryCtx, channelHandler, group, group, nil, false, time.Now(), 0, nil)
	if queryRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected original owner 429, got status=%d body=%s", queryRecorder.Code, queryRecorder.Body.String())
	}
	if hits := serverBHits.Load(); hits != 0 {
		t.Fatalf("batch request migrated to a different physical resource %d time(s)", hits)
	}
	if hits := serverAHits.Load(); hits != 2 {
		t.Fatalf("unexpected original resource hit count: got %d want 2", hits)
	}

	failFileUpload.Store(true)
	beforeUploadHits := serverAHits.Load() + serverBHits.Load()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/proxy/batch-route/v1/files", bytes.NewReader([]byte("multipart-placeholder")))
	uploadRecorder := httptest.NewRecorder()
	uploadCtx, _ := gin.CreateTestContext(uploadRecorder)
	uploadCtx.Request = uploadRequest
	uploadCtx.Params = gin.Params{{Key: "group_name", Value: group.Name}, {Key: "path", Value: "/v1/files"}}
	uploadRouting, apiErr := proxyServer.resolveUpstreamObjectRouting(uploadCtx, group, group, []byte("multipart-placeholder"))
	if apiErr != nil || !uploadRouting.NonReplayableOnUncertain {
		t.Fatalf("file upload was not marked non-replayable: %#v %v", uploadRouting, apiErr)
	}
	uploadCtx.Set(upstreamObjectRoutingContextKey, uploadRouting)
	proxyServer.executeRequestWithRetry(uploadCtx, channelHandler, group, group, []byte("multipart-placeholder"), false, time.Now(), 0, nil)
	if uploadRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected uncertain file upload failure, got status=%d body=%s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	afterUploadHits := serverAHits.Load() + serverBHits.Load()
	if afterUploadHits-beforeUploadHits != 1 {
		t.Fatalf("uncertain non-replayable file upload was retried across resources: before=%d after=%d", beforeUploadHits, afterUploadHits)
	}
}

func (c *resourceRoutingChannel) ModifyRequest(req *http.Request, apiKey *models.APIKey, _ *models.Group) {
	req.Header.Set("x-api-key", apiKey.KeyValue)
}

func (c *resourceRoutingChannel) IsStreamRequest(*gin.Context, []byte) bool { return false }
func (c *resourceRoutingChannel) ExtractModel(*gin.Context, []byte) string  { return "test-model" }
func (c *resourceRoutingChannel) ValidateKey(context.Context, *models.APIKey, *models.Group) (bool, error) {
	return true, nil
}

func TestRES005ProxyFailoverKeepsURLAndKeyAtomicAndMigratesAffinity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var mu sync.Mutex
	failingKey := ""
	mismatches := make([]string, 0)
	newUpstream := func(name, expectedKey string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actualKey := r.Header.Get("x-api-key")
			mu.Lock()
			if actualKey != expectedKey {
				mismatches = append(mismatches, fmt.Sprintf("%s received %s want %s", name, actualKey, expectedKey))
			}
			shouldFail := actualKey == failingKey
			mu.Unlock()
			if shouldFail {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"upstream":"` + name + `"}`))
		}))
	}
	serverA := newUpstream("a", "key-a")
	defer serverA.Close()
	serverB := newUpstream("b", "key-b")
	defer serverB.Close()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:proxy-resource-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ResourcePool{}, &models.UpstreamResource{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool := models.ResourcePool{Name: "shared", Strategy: "round_robin", AffinityTTLSeconds: 3600, BusyWaitMilliseconds: 2000}
	if err := db.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	crypto, err := encryption.NewService("")
	if err != nil {
		t.Fatalf("create encryption service: %v", err)
	}
	provider := resourcepool.NewProvider(db, store.NewMemoryStore(), crypto)
	if err := provider.AddResources(pool.ID, []models.UpstreamResource{
		{Name: "a", UpstreamURL: serverA.URL, KeyValue: "key-a"},
		{Name: "b", UpstreamURL: serverB.URL, KeyValue: "key-b"},
	}); err != nil {
		t.Fatalf("add resources: %v", err)
	}
	if err := provider.LoadResourcesFromDB(); err != nil {
		t.Fatalf("load resources: %v", err)
	}

	affinityHash := crypto.Hash("project-a")
	first, err := provider.SelectResource(pool.ID, resourcepool.SelectionRequest{Route: "anthropic", Affinity: affinityHash})
	if err != nil {
		t.Fatalf("bind initial affinity: %v", err)
	}
	mu.Lock()
	failingKey = first.KeyValue
	mu.Unlock()

	matcher, err := failover.ParseStatusCodeMatcher("429,500-599")
	if err != nil {
		t.Fatalf("parse failover matcher: %v", err)
	}
	group := &models.Group{
		ID:                        1,
		Name:                      "shared-route",
		ChannelType:               "anthropic",
		ResourcePoolID:            &pool.ID,
		FailoverStatusCodeMatcher: matcher,
		EffectiveConfig: types.SystemSettings{
			MaxRetries:     1,
			RequestTimeout: 5,
		},
	}
	body := []byte(`{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`)
	request := httptest.NewRequest(http.MethodPost, "/proxy/shared-route/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "group_name", Value: "shared-route"}}
	ctx.Set(requestAffinityContextKey, requestAffinity{Hash: affinityHash, Source: "explicit"})

	proxyServer := &ProxyServer{resourceProvider: provider, encryptionSvc: crypto}
	channelHandler := &resourceRoutingChannel{BaseChannel: &channel.BaseChannel{
		Name:         "anthropic",
		HTTPClient:   &http.Client{Timeout: 5 * time.Second},
		StreamClient: &http.Client{},
	}}
	proxyServer.executeRequestWithRetry(ctx, channelHandler, group, group, body, false, time.Now(), 0, nil)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	mu.Lock()
	observedMismatches := append([]string(nil), mismatches...)
	mu.Unlock()
	if len(observedMismatches) != 0 {
		t.Fatalf("URL/key pairs crossed: %v", observedMismatches)
	}
	migrated, err := provider.SelectResource(pool.ID, resourcepool.SelectionRequest{Route: "anthropic", Affinity: affinityHash})
	if err != nil {
		t.Fatalf("select migrated affinity: %v", err)
	}
	if migrated.ID == first.ID {
		t.Fatalf("affinity did not migrate away from failed resource %d", first.ID)
	}
}
