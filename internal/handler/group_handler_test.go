package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"api-load/internal/config"
	"api-load/internal/keypool"
	"api-load/internal/models"
	"api-load/internal/services"
	"api-load/internal/store"

	"github.com/gin-gonic/gin"
)

func TestMOD005GroupModelHandlersPersistEnabledModels(t *testing.T) {
	_, db, _, _ := servicesTestKeyService(t)
	group := handlerTestGroup(t, db)
	groupSvc := services.NewGroupService(db, config.NewSystemSettingsManager(), nil, nil, nil, nil, nil)
	server := &Server{
		DB:              db,
		SettingsManager: config.NewSystemSettingsManager(),
		GroupService:    groupSvc,
	}

	body, err := json.Marshal(map[string]any{"models": []string{" gpt-b ", "gpt-a", "gpt-b"}})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	saveRecorder := httptest.NewRecorder()
	saveCtx, _ := gin.CreateTestContext(saveRecorder)
	saveCtx.Request = httptest.NewRequest(http.MethodPut, "/groups/1/models", bytes.NewReader(body))
	saveCtx.Request.Header.Set("Content-Type", "application/json")
	saveCtx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(group.ID), 10)}}

	server.SaveGroupModels(saveCtx)
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("expected save status 200, got %d body=%s", saveRecorder.Code, saveRecorder.Body.String())
	}

	getRecorder := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRecorder)
	getCtx.Request = httptest.NewRequest(http.MethodGet, "/groups/1/models", nil)
	getCtx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(group.ID), 10)}}

	server.GetGroupModels(getCtx)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected get status 200, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	var response struct {
		Data struct {
			Models []string `json:"models"`
		} `json:"data"`
	}
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := response.Data.Models; len(got) != 2 || got[0] != "gpt-a" || got[1] != "gpt-b" {
		t.Fatalf("unexpected models response: %#v", got)
	}
}

func TestMOD001GroupModelDiscoveryHandlerReturnsDiscoveredModels(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "gpt-discovered"}},
		})
	}))
	defer upstream.Close()

	_, db, _, encryptionSvc := servicesTestKeyService(t)
	group := handlerTestGroup(t, db)
	group.Upstreams = []byte(`[{"url":"` + upstream.URL + `","weight":1}]`)
	if err := db.Save(&group).Error; err != nil {
		t.Fatalf("save group upstream: %v", err)
	}
	memStore := store.NewMemoryStore()
	provider := keypool.NewProvider(db, memStore, nil, encryptionSvc)
	keySvc := services.NewKeyService(db, provider, nil, encryptionSvc)
	servicesSeedKey(t, keySvc, group.ID, "sk-test-discovery-handler", "", models.KeyStatusActive, 0, 0)

	server := &Server{
		DB:            db,
		KeyService:    keySvc,
		EncryptionSvc: encryptionSvc,
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/groups/1/models/discover", nil)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(group.ID), 10)}}

	server.DiscoverGroupModels(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected discovery status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/v1/models" {
		t.Fatalf("expected discovery to call /v1/models, got %q", gotPath)
	}

	var response struct {
		Data struct {
			Models []string `json:"models"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.Models) != 1 || response.Data.Models[0] != "gpt-discovered" {
		t.Fatalf("unexpected discovered models: %#v", response.Data.Models)
	}
}
