package handler

import (
	"api-load/internal/encryption"
	"api-load/internal/i18n"
	"api-load/internal/keypool"
	"api-load/internal/models"
	"api-load/internal/services"
	"api-load/internal/store"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func init() {
	if err := i18n.Init(); err != nil {
		panic(err)
	}
}

func servicesTestKeyService(t *testing.T) (*services.KeyService, *gorm.DB, store.Store, encryption.Service) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:handler-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Group{}, &models.APIKey{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	encryptionSvc, err := encryption.NewService("")
	if err != nil {
		t.Fatalf("create encryption service: %v", err)
	}
	memStore := store.NewMemoryStore()
	provider := keypool.NewProvider(db, memStore, nil, encryptionSvc)
	return services.NewKeyService(db, provider, nil, encryptionSvc), db, memStore, encryptionSvc
}

func handlerTestGroup(t *testing.T, db *gorm.DB) models.Group {
	t.Helper()

	group := models.Group{
		Name:               fmt.Sprintf("handler-group-%d", time.Now().UnixNano()),
		GroupType:          "standard",
		Upstreams:          []byte(`[{"url":"https://example.invalid","weight":1}]`),
		ChannelType:        "openai",
		TestModel:          "gpt-test",
		ValidationEndpoint: "/v1/models",
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	return group
}

func servicesSeedKey(t *testing.T, svc *services.KeyService, groupID uint, rawKey, notes, status string, failureCount, requestCount int64) models.APIKey {
	t.Helper()
	enabled := true
	if status == models.KeyStatusDisabled {
		status = models.KeyStatusActive
		enabled = false
	}

	key := models.APIKey{
		GroupID:      groupID,
		KeyValue:     rawKey,
		KeyHash:      svc.EncryptionSvc.Hash(rawKey),
		Notes:        notes,
		Status:       status,
		Enabled:      models.Bool(enabled),
		Priority:     models.DefaultCredentialPriority,
		Weight:       models.DefaultCredentialWeight,
		FailureCount: failureCount,
		RequestCount: requestCount,
	}
	if err := svc.KeyProvider.AddKeys(groupID, []models.APIKey{key}); err != nil {
		t.Fatalf("seed key: %v", err)
	}
	var stored models.APIKey
	if err := svc.DB.Where("key_hash = ?", key.KeyHash).First(&stored).Error; err != nil {
		t.Fatalf("reload seed key: %v", err)
	}
	return stored
}

func newKeyHandlerTestServer(t *testing.T) (*Server, *httptest.ResponseRecorder, *gin.Context, uint) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	svc, db, _, encryptionSvc := servicesTestKeyService(t)
	group := handlerTestGroup(t, db)
	key := servicesSeedKey(t, svc, group.ID, "sk-test-handler", "", models.KeyStatusActive, 2, 0)

	server := &Server{
		DB:            db,
		KeyService:    svc,
		EncryptionSvc: encryptionSvc,
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	return server, recorder, ctx, key.ID
}

func TestKEY006UpdateKeyStatusHandlerDisablesKey(t *testing.T) {
	server, recorder, ctx, keyID := newKeyHandlerTestServer(t)
	body := bytes.NewBufferString(`{"status":"disabled"}`)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/keys/1/status", body)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}

	server.UpdateKeyStatus(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var stored models.APIKey
	if err := server.DB.First(&stored, keyID).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if stored.Status != models.KeyStatusActive || models.CredentialEnabled(stored.Enabled) {
		t.Fatalf("expected key manually disabled with healthy status, got %#v", stored)
	}
}

func TestKEY006BatchUpdateKeyStatusHandlerReportsChangedAndIgnored(t *testing.T) {
	server, recorder, ctx, keyID := newKeyHandlerTestServer(t)
	bodyBytes, err := json.Marshal(map[string]any{
		"key_ids": []uint{keyID},
		"status":  "disabled",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/keys/status", bytes.NewReader(bodyBytes))
	ctx.Request.Header.Set("Content-Type", "application/json")

	server.BatchUpdateKeyStatus(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte(`"changed_count":1`)) {
		t.Fatalf("expected changed_count in response, got %s", recorder.Body.String())
	}
}
