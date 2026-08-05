package services

import (
	"api-load/internal/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	return s.DiscoverModelsWithContext(context.Background(), group, keys)
}

func (s *ModelDiscoveryService) DiscoverModelsWithContext(
	ctx context.Context,
	group *models.Group,
	keys []models.APIKey,
) ([]string, error) {
	activeKey, ok := firstActiveDiscoveryKey(keys)
	if !ok {
		return nil, fmt.Errorf("no active keys available for model discovery")
	}
	baseURL, err := firstActiveUpstreamURL(group)
	if err != nil {
		return nil, err
	}

	return s.DiscoverEndpointModels(ctx, group.ChannelType, baseURL, activeKey.KeyValue)
}

func (s *ModelDiscoveryService) DiscoverEndpointModels(
	ctx context.Context,
	channelType string,
	baseURL string,
	key string,
) ([]string, error) {
	switch channelType {
	case "openai", "openai-response", "openrouter", "deepseek", "qwen", "xai", "azure-openai":
		return s.discoverOpenAICompatible(ctx, NormalizeOpenAIModelBaseURL(baseURL), key)
	case "gemini":
		return s.discoverGemini(ctx, strings.TrimRight(baseURL, "/"), key)
	case "anthropic":
		return s.discoverAnthropic(ctx, strings.TrimRight(baseURL, "/"), key)
	default:
		return nil, fmt.Errorf("%w: channel type %s", ErrModelDiscoveryUnsupported, channelType)
	}
}

func NormalizeOpenAIModelBaseURL(baseURL string) string {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.EqualFold(normalized, "https://openrouter.ai") || strings.EqualFold(normalized, "http://openrouter.ai") {
		return normalized + "/api"
	}
	return normalized
}

func (s *ModelDiscoveryService) discoverOpenAICompatible(ctx context.Context, baseURL, key string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, inspectionURL(baseURL, "/v1/models"), nil)
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

func (s *ModelDiscoveryService) discoverGemini(ctx context.Context, baseURL, key string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, inspectionURL(baseURL, "/v1beta/models"), nil)
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

func (s *ModelDiscoveryService) discoverAnthropic(ctx context.Context, baseURL, key string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, inspectionURL(baseURL, "/v1/models"), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := s.doJSON(req, &body); err != nil {
		return nil, err
	}
	modelIDs := make([]string, 0, len(body.Data))
	for _, item := range body.Data {
		if id := strings.TrimSpace(item.ID); id != "" {
			modelIDs = append(modelIDs, id)
		}
	}
	return modelIDs, nil
}

func inspectionURL(baseURL, apiPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(apiPath, "/v1/") {
		return base + strings.TrimPrefix(apiPath, "/v1")
	}
	if strings.HasSuffix(base, "/v1beta") && strings.HasPrefix(apiPath, "/v1beta/") {
		return base + strings.TrimPrefix(apiPath, "/v1beta")
	}
	return base + apiPath
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
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(target)
}

func firstActiveDiscoveryKey(keys []models.APIKey) (models.APIKey, bool) {
	for _, key := range keys {
		if models.CredentialEnabled(key.Enabled) && (key.Status == "" || key.Status == models.KeyStatusActive) {
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
