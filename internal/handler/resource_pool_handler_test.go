package handler

import (
	"api-load/internal/models"
	"api-load/internal/resourcepool"
	"api-load/internal/services"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRES009ResourcePoolHandlerNeverReturnsRawCredential(t *testing.T) {
	_, db, memStore, encryptionSvc := servicesTestKeyService(t)
	if err := db.AutoMigrate(&models.ResourcePool{}, &models.UpstreamResource{}); err != nil {
		t.Fatalf("migrate resource pool models: %v", err)
	}
	provider := resourcepool.NewProvider(db, memStore, encryptionSvc)
	poolSvc := services.NewResourcePoolService(db, provider, encryptionSvc)
	pool, err := poolSvc.CreatePool(t.Context(), services.ResourcePoolCreateParams{Name: "handler-pool"})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	server := &Server{ResourcePoolService: poolSvc}
	body := []byte(`[{"name":"official-a","upstream_url":"https://api.example.invalid","key":"sk-handler-secret-9876"}]`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/resource-pools/1/resources", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(pool.ID), 10)}}

	server.AddResourcePoolResources(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	responseBody := recorder.Body.String()
	if strings.Contains(responseBody, "sk-handler-secret") || strings.Contains(responseBody, "key_value") {
		t.Fatalf("handler leaked raw credential: %s", responseBody)
	}
	if !strings.Contains(responseBody, `"masked_key":"****9876"`) {
		t.Fatalf("handler did not return masked identity: %s", responseBody)
	}
}

func TestRES010GroupUpdateRequestTracksExplicitPoolUnbind(t *testing.T) {
	var omitted GroupUpdateRequest
	if err := json.Unmarshal([]byte(`{"description":"unchanged"}`), &omitted); err != nil {
		t.Fatalf("decode omitted resource pool: %v", err)
	}
	if omitted.HasResourcePoolID {
		t.Fatal("omitted resource_pool_id was treated as an update")
	}

	var explicitNull GroupUpdateRequest
	if err := json.Unmarshal([]byte(`{"resource_pool_id":null}`), &explicitNull); err != nil {
		t.Fatalf("decode explicit resource pool unbind: %v", err)
	}
	if !explicitNull.HasResourcePoolID || explicitNull.ResourcePoolID != nil {
		t.Fatalf("explicit null did not preserve unbind intent: %#v", explicitNull)
	}
}
