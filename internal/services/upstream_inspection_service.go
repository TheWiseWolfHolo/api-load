package services

import (
	"api-load/internal/config"
	"api-load/internal/httpclient"
	"api-load/internal/models"
	"api-load/internal/resourcepool"
	"api-load/internal/types"
	"context"
	"fmt"
	"net/http"
	"time"
)

type EndpointModelsSnapshot struct {
	ResourceID uint
	Models     []string
}

type UpstreamInspectionService struct {
	provider        *resourcepool.Provider
	clientManager   *httpclient.HTTPClientManager
	settingsManager *config.SystemSettingsManager
}

func NewUpstreamInspectionService(
	provider *resourcepool.Provider,
	clientManager *httpclient.HTTPClientManager,
	settingsManager *config.SystemSettingsManager,
) *UpstreamInspectionService {
	return &UpstreamInspectionService{
		provider: provider, clientManager: clientManager, settingsManager: settingsManager,
	}
}

func (s *UpstreamInspectionService) DiscoverPoolEndpointModels(
	ctx context.Context,
	poolID, endpointID uint,
) (EndpointModelsSnapshot, error) {
	endpoint, err := s.provider.LoadEndpointForInspection(poolID, endpointID)
	if err != nil {
		return EndpointModelsSnapshot{}, err
	}
	resource, err := s.provider.SelectResource(poolID, resourcepool.SelectionRequest{
		Route: endpoint.ChannelType, Strategy: "round_robin",
		SchedulerScope: fmt.Sprintf("inspection:models:%d", endpointID),
	})
	if err != nil {
		return EndpointModelsSnapshot{}, err
	}
	modelIDs, err := NewModelDiscoveryService(s.clientForGroup(nil)).DiscoverEndpointModels(
		ctx,
		endpoint.ChannelType,
		endpoint.BaseURL,
		resource.KeyValue,
	)
	if err != nil {
		return EndpointModelsSnapshot{}, err
	}
	return EndpointModelsSnapshot{ResourceID: resource.ID, Models: modelIDs}, nil
}

func (s *UpstreamInspectionService) DiscoverGroupModels(
	ctx context.Context,
	group *models.Group,
	keys []models.APIKey,
) ([]string, error) {
	return NewModelDiscoveryService(s.clientForGroup(group)).DiscoverModelsWithContext(ctx, group, keys)
}

func (s *UpstreamInspectionService) InspectPoolResourceBalance(
	ctx context.Context,
	poolID, endpointID, resourceID uint,
) (BalanceSnapshot, error) {
	endpoint, err := s.provider.LoadEndpointForInspection(poolID, endpointID)
	if err != nil {
		return BalanceSnapshot{}, err
	}
	adapter, origin := balanceAdapterForBaseURL(endpoint.BaseURL)
	if adapter == nil {
		return BalanceSnapshot{}, fmt.Errorf("%w for endpoint host", ErrBalanceInspectionUnsupported)
	}
	resource, err := s.provider.LoadResourceForInspection(poolID, resourceID)
	if err != nil {
		return BalanceSnapshot{}, err
	}
	snapshot, err := queryBalanceWithAdapter(
		ctx,
		s.clientForGroup(nil),
		origin,
		resource.KeyValue,
		*adapter,
	)
	if err != nil {
		return BalanceSnapshot{}, err
	}
	snapshot.ResourceID = resource.ID
	snapshot.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	return snapshot, nil
}

func (s *UpstreamInspectionService) clientForGroup(group *models.Group) *http.Client {
	settings := types.SystemSettings{}
	if s.settingsManager != nil {
		var overrides map[string]any
		if group != nil {
			overrides = group.Config
		}
		settings = s.settingsManager.GetEffectiveConfig(overrides)
	}
	return s.clientManager.GetClient(&httpclient.Config{
		ConnectTimeout:        durationOrDefault(settings.ConnectTimeout, 10) * time.Second,
		RequestTimeout:        durationOrDefault(settings.RequestTimeout, 30) * time.Second,
		IdleConnTimeout:       durationOrDefault(settings.IdleConnTimeout, 90) * time.Second,
		ResponseHeaderTimeout: durationOrDefault(settings.ResponseHeaderTimeout, 30) * time.Second,
		MaxIdleConns:          intOrDefault(settings.MaxIdleConns, 20),
		MaxIdleConnsPerHost:   intOrDefault(settings.MaxIdleConnsPerHost, 10),
		ProxyURL:              settings.ProxyURL,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: time.Second,
		WriteBufferSize:       32 * 1024,
		ReadBufferSize:        32 * 1024,
	})
}

func durationOrDefault(value, fallback int) time.Duration {
	if value <= 0 {
		return time.Duration(fallback)
	}
	return time.Duration(value)
}

func intOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
