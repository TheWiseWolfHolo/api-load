package services

import (
	"api-load/internal/models"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var ErrModelDiscoveryUnsupported = errors.New("model discovery unsupported")

type ModelDiscoveryService struct {
	client *http.Client
}

func NewModelDiscoveryService(client *http.Client) *ModelDiscoveryService {
	if client == nil {
		client = http.DefaultClient
	}
	return &ModelDiscoveryService{client: client}
}

func (s *ModelDiscoveryService) DiscoverModels(group *models.Group, keys []models.APIKey) ([]string, error) {
	if group.ChannelType == "anthropic" {
		return nil, fmt.Errorf("%w: Anthropic model discovery is manual-only in this phase", ErrModelDiscoveryUnsupported)
	}

	activeKey, ok := firstActiveDiscoveryKey(keys)
	if !ok {
		return nil, fmt.Errorf("no active keys available for model discovery")
	}
	baseURL, err := firstActiveUpstreamURL(group)
	if err != nil {
		return nil, err
	}

	switch group.ChannelType {
	case "openai", "openai-response", "openrouter", "deepseek", "qwen", "xai", "azure-openai":
		return s.discoverOpenAICompatible(NormalizeOpenAIModelBaseURL(baseURL), activeKey.KeyValue)
	case "gemini":
		return s.discoverGemini(strings.TrimRight(baseURL, "/"), activeKey.KeyValue)
	default:
		return nil, fmt.Errorf("%w: channel type %s", ErrModelDiscoveryUnsupported, group.ChannelType)
	}
}

func NormalizeOpenAIModelBaseURL(baseURL string) string {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.EqualFold(normalized, "https://openrouter.ai") || strings.EqualFold(normalized, "http://openrouter.ai") {
		return normalized + "/api"
	}
	return normalized
}

func (s *ModelDiscoveryService) discoverOpenAICompatible(baseURL, key string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := s.doJSON(req, &body); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(body.Data))
	for _, item := range body.Data {
		id := strings.TrimSpace(item.ID)
		if id != "" {
			models = append(models, id)
		}
	}
	return models, nil
}

func (s *ModelDiscoveryService) discoverGemini(baseURL, key string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1beta/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", key)

	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := s.doJSON(req, &body); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(body.Models))
	for _, item := range body.Models {
		name := strings.TrimSpace(strings.TrimPrefix(item.Name, "models/"))
		if name != "" {
			models = append(models, name)
		}
	}
	return models, nil
}

func (s *ModelDiscoveryService) doJSON(req *http.Request, target any) error {
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("model discovery failed with status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func firstActiveDiscoveryKey(keys []models.APIKey) (models.APIKey, bool) {
	for _, key := range keys {
		if key.Status == "" || key.Status == models.KeyStatusActive {
			return key, true
		}
	}
	return models.APIKey{}, false
}

func firstActiveUpstreamURL(group *models.Group) (string, error) {
	var upstreams []struct {
		URL    string `json:"url"`
		Weight int    `json:"weight"`
	}
	if err := json.Unmarshal(group.Upstreams, &upstreams); err != nil {
		return "", fmt.Errorf("invalid upstreams: %w", err)
	}
	for _, upstream := range upstreams {
		if upstream.Weight > 0 && strings.TrimSpace(upstream.URL) != "" {
			return strings.TrimSpace(upstream.URL), nil
		}
	}
	return "", fmt.Errorf("no active upstream available for model discovery")
}
