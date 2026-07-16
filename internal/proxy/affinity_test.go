package proxy

import (
	"api-load/internal/encryption"
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func affinityTestContext(body []byte) *gin.Context {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	req.Header.Set("User-Agent", "claude-code/2.1.193")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	return ctx
}

func affinityTestEncryption(t *testing.T) encryption.Service {
	t.Helper()
	service, err := encryption.NewService("")
	if err != nil {
		t.Fatalf("create encryption service: %v", err)
	}
	return service
}

func TestAFF001ExplicitProjectAffinityWins(t *testing.T) {
	body := []byte(`{"metadata":{"user_id":"session-body"},"messages":[{"role":"user","content":"hello"}]}`)
	ctx := affinityTestContext(body)
	ctx.Request.Header.Set(affinityHeader, "project-api-load")
	affinity := deriveRequestAffinity(ctx, body, "shared-key", affinityTestEncryption(t))
	if affinity.Source != "explicit" || affinity.Hash == "" || affinity.Hash == "project-api-load" {
		t.Fatalf("unexpected explicit affinity: %#v", affinity)
	}
}

func TestAFF002MetadataKeepsGrowingConversationStable(t *testing.T) {
	first := []byte(`{"metadata":{"user_id":"session-123"},"messages":[{"role":"user","content":"hello"}]}`)
	second := []byte(`{"metadata":{"user_id":"session-123"},"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"hi"},{"role":"user","content":"next"}]}`)
	service := affinityTestEncryption(t)
	a := deriveRequestAffinity(affinityTestContext(first), first, "shared-key", service)
	b := deriveRequestAffinity(affinityTestContext(second), second, "shared-key", service)
	if a.Source != "metadata" || a.Hash != b.Hash {
		t.Fatalf("metadata affinity changed across turns: %#v %#v", a, b)
	}
}

func TestAFF003CacheControlPrecedesDigestFallback(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"project context","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"hello"}]}`)
	affinity := deriveRequestAffinity(affinityTestContext(body), body, "shared-key", affinityTestEncryption(t))
	if affinity.Source != "cache_control" || affinity.Hash == "" {
		t.Fatalf("unexpected cache affinity: %#v", affinity)
	}
}

func TestAFF004DigestUsesStableFirstUserMessage(t *testing.T) {
	first := []byte(`{"system":"coding","messages":[{"role":"user","content":"start"}]}`)
	second := []byte(`{"system":"coding","messages":[{"role":"user","content":"start"},{"role":"assistant","content":"done"},{"role":"user","content":"continue"}]}`)
	service := affinityTestEncryption(t)
	a := deriveRequestAffinity(affinityTestContext(first), first, "shared-key", service)
	b := deriveRequestAffinity(affinityTestContext(second), second, "shared-key", service)
	if a.Source != "request_digest" || a.Hash != b.Hash {
		t.Fatalf("digest fallback changed across turns: %#v %#v", a, b)
	}
}
