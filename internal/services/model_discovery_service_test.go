package services

import (
	"api-load/internal/models"
	"encoding/json"
	"errors"
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

func TestMOD004AnthropicDiscoveryIsManualOnly(t *testing.T) {
	service := NewModelDiscoveryService(http.DefaultClient)
	_, err := service.DiscoverModels(&models.Group{ChannelType: "anthropic"}, []models.APIKey{{KeyValue: "sk-ant-test"}})
	if !errors.Is(err, ErrModelDiscoveryUnsupported) {
		t.Fatalf("expected unsupported discovery error, got %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "sk-ant-test") {
		t.Fatalf("error exposed key: %v", err)
	}
}
