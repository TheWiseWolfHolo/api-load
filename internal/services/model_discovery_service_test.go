package services

import (
	"api-load/internal/models"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMOD001OpenAICompatibleModelDiscoveryCallsV1Models(t *testing.T) {
	var gotPath string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "gpt-test-a"}, {"id": "gpt-test-b"}},
		})
	}))
	defer server.Close()

	group := models.Group{ChannelType: "openai", Upstreams: []byte(`[{"url":"` + server.URL + `","weight":1}]`)}
	service := NewModelDiscoveryService(http.DefaultClient)
	models, err := service.DiscoverModels(&group, []models.APIKey{{KeyValue: "sk-test-discovery", Status: models.KeyStatusActive}})
	if err != nil {
		t.Fatalf("discover models: %v", err)
	}

	if gotPath != "/v1/models" {
		t.Fatalf("expected /v1/models request, got %q", gotPath)
	}
	if gotAuth != "Bearer sk-test-discovery" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if strings.Join(models, ",") != "gpt-test-a,gpt-test-b" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestMOD002NormalizeOpenRouterBaseURL(t *testing.T) {
	if got := NormalizeOpenAIModelBaseURL("https://openrouter.ai/"); got != "https://openrouter.ai/api" {
		t.Fatalf("expected openrouter /api base, got %q", got)
	}
	if got := NormalizeOpenAIModelBaseURL("https://openrouter.ai/api/"); got != "https://openrouter.ai/api" {
		t.Fatalf("expected no double /api, got %q", got)
	}
}

func TestMOD003GeminiDiscoveryCallsV1BetaModelsAndStripsPrefix(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]string{
				{"name": "models/gemini-2.5-pro"},
				{"name": "gemini-2.0-flash"},
				{"name": ""},
			},
		})
	}))
	defer server.Close()

	group := models.Group{ChannelType: "gemini", Upstreams: []byte(`[{"url":"` + server.URL + `","weight":1}]`)}
	service := NewModelDiscoveryService(http.DefaultClient)
	models, err := service.DiscoverModels(&group, []models.APIKey{{KeyValue: "AIzaSyDummyDiscovery", Status: models.KeyStatusActive}})
	if err != nil {
		t.Fatalf("discover gemini models: %v", err)
	}

	if gotPath != "/v1beta/models" {
		t.Fatalf("expected /v1beta/models request, got %q", gotPath)
	}
	if strings.Join(models, ",") != "gemini-2.5-pro,gemini-2.0-flash" {
		t.Fatalf("unexpected gemini models: %#v", models)
	}
}

func TestMOD004AnthropicModelsUseNativeHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-ant-test" {
			t.Fatalf("missing Anthropic key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("missing Anthropic version header")
		}
		_, _ = w.Write([]byte("{\"data\":[{\"id\":\"claude-test-a\"},{\"id\":\"claude-test-b\"}]}"))
	}))
	defer server.Close()

	service := NewModelDiscoveryService(server.Client())
	modelIDs, err := service.DiscoverEndpointModels(
		context.Background(),
		"anthropic",
		server.URL,
		"sk-ant-test",
	)
	if err != nil {
		t.Fatalf("discover Anthropic models: %v", err)
	}
	if strings.Join(modelIDs, ",") != "claude-test-a,claude-test-b" {
		t.Fatalf("unexpected Anthropic models: %#v", modelIDs)
	}
}

func TestMOD005DiscoverySkipsManuallyDisabledKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-enabled" {
			t.Fatalf("used disabled discovery key: %q", got)
		}
		_, _ = w.Write([]byte("{\"data\":[{\"id\":\"gpt-enabled\"}]}"))
	}))
	defer server.Close()

	group := models.Group{ChannelType: "openai", Upstreams: []byte(`[{"url":"` + server.URL + `","weight":1}]`)}
	service := NewModelDiscoveryService(server.Client())
	modelIDs, err := service.DiscoverModels(&group, []models.APIKey{
		{KeyValue: "sk-disabled", Enabled: models.Bool(false), Status: models.KeyStatusActive},
		{KeyValue: "sk-enabled", Enabled: models.Bool(true), Status: models.KeyStatusActive},
	})
	if err != nil {
		t.Fatalf("discover with enabled key: %v", err)
	}
	if strings.Join(modelIDs, ",") != "gpt-enabled" {
		t.Fatalf("unexpected models: %#v", modelIDs)
	}
}
