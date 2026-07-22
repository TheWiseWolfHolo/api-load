package services

import (
	"api-load/internal/channel"
	"api-load/internal/config"
	"api-load/internal/encryption"
	app_errors "api-load/internal/errors"
	"api-load/internal/models"
	"api-load/internal/resourcepool"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ResourceValidationGroup describes one bound route that can validate a
// physical resource using its own protocol, model and validation endpoint.
type ResourceValidationGroup struct {
	ID                 uint   `json:"id"`
	Name               string `json:"name"`
	DisplayName        string `json:"display_name"`
	ChannelType        string `json:"channel_type"`
	TestModel          string `json:"test_model"`
	ValidationEndpoint string `json:"validation_endpoint"`
}

type ResourceValidationResult struct {
	ResourceID  uint   `json:"resource_id"`
	GroupID     uint   `json:"group_id"`
	GroupName   string `json:"group_name"`
	ChannelType string `json:"channel_type"`
	IsValid     bool   `json:"is_valid"`
	Error       string `json:"error,omitempty"`
	DurationMS  int64  `json:"duration_ms"`
}

type ResourceValidationService struct {
	db              *gorm.DB
	channelFactory  *channel.Factory
	settingsManager *config.SystemSettingsManager
	provider        *resourcepool.Provider
	encryptionSvc   encryption.Service
}

func NewResourceValidationService(
	db *gorm.DB,
	channelFactory *channel.Factory,
	settingsManager *config.SystemSettingsManager,
	provider *resourcepool.Provider,
	encryptionSvc encryption.Service,
) *ResourceValidationService {
	return &ResourceValidationService{
		db: db, channelFactory: channelFactory, settingsManager: settingsManager,
		provider: provider, encryptionSvc: encryptionSvc,
	}
}

func (s *ResourceValidationService) ListValidationGroups(ctx context.Context, poolID uint) ([]ResourceValidationGroup, error) {
	if err := s.ensurePoolExists(ctx, poolID); err != nil {
		return nil, err
	}
	var groups []models.Group
	if err := s.db.WithContext(ctx).
		Where("resource_pool_id = ? AND group_type = ?", poolID, "standard").
		Order("sort asc, id asc").Find(&groups).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}
	result := make([]ResourceValidationGroup, 0, len(groups))
	for i := range groups {
		result = append(result, validationGroupView(&groups[i]))
	}
	return result, nil
}

// TestResource validates one resource through one group bound to the same pool.
// Validation probes update health state but never usage counters.
func (s *ResourceValidationService) TestResource(ctx context.Context, poolID, resourceID, groupID uint) (*ResourceValidationResult, error) {
	var resource models.UpstreamResource
	if err := s.db.WithContext(ctx).
		Where("id = ? AND resource_pool_id = ?", resourceID, poolID).
		First(&resource).Error; err != nil {
		return nil, app_errors.ParseDBError(err)
	}

	var group models.Group
	if err := s.db.WithContext(ctx).
		Where("id = ? AND resource_pool_id = ? AND group_type = ?", groupID, poolID, "standard").
		First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, resourcePoolValidationError("validation group must be bound to this resource pool")
		}
		return nil, app_errors.ParseDBError(err)
	}

	plainKey, err := s.encryptionSvc.Decrypt(resource.KeyValue)
	if err != nil {
		return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, "failed to decrypt resource credential")
	}
	upstreams, err := json.Marshal([]map[string]any{{"url": resource.UpstreamURL, "weight": 1}})
	if err != nil {
		return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
	}
	testGroup := group
	testGroup.ResourcePoolID = nil
	testGroup.Upstreams = datatypes.JSON(upstreams)
	testGroup.EffectiveConfig = s.settingsManager.GetEffectiveConfig(group.Config)
	if len(group.HeaderRules) > 0 {
		if err := json.Unmarshal(group.HeaderRules, &testGroup.HeaderRuleList); err != nil {
			return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, "failed to parse validation header rules")
		}
	}

	proxyChannel, err := s.channelFactory.CreateChannel(&testGroup)
	if err != nil {
		return nil, resourcePoolValidationError(err.Error())
	}
	timeout := time.Duration(testGroup.EffectiveConfig.KeyValidationTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	validationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	isValid, validationErr := proxyChannel.ValidateKey(validationCtx, &models.APIKey{
		GroupID: group.ID, KeyValue: plainKey, Enabled: models.Bool(true), Status: models.KeyStatusActive,
	}, &testGroup)
	durationMS := time.Since(started).Milliseconds()
	errorMessage := ""
	if validationErr != nil {
		errorMessage = validationErr.Error()
	}

	if isValid {
		if err := s.provider.MarkHealthy(&resource); err != nil {
			return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
		}
		if err := s.provider.ClearRouteCooldown(resource.ID, group.ChannelType); err != nil {
			return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
		}
	} else {
		if err := s.provider.HandleFailure(&resource, group.ChannelType, validationStatusCode(errorMessage), errorMessage, nil); err != nil {
			return nil, app_errors.NewAPIError(app_errors.ErrInternalServer, err.Error())
		}
	}

	return &ResourceValidationResult{
		ResourceID:  resource.ID,
		GroupID:     group.ID,
		GroupName:   group.Name,
		ChannelType: group.ChannelType,
		IsValid:     isValid,
		Error:       errorMessage,
		DurationMS:  durationMS,
	}, nil
}

func (s *ResourceValidationService) ensurePoolExists(ctx context.Context, poolID uint) error {
	if poolID == 0 {
		return app_errors.ErrResourceNotFound
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.ResourcePool{}).Where("id = ?", poolID).Count(&count).Error; err != nil {
		return app_errors.ParseDBError(err)
	}
	if count == 0 {
		return app_errors.ErrResourceNotFound
	}
	return nil
}

func validationGroupView(group *models.Group) ResourceValidationGroup {
	return ResourceValidationGroup{
		ID: group.ID, Name: group.Name, DisplayName: group.DisplayName,
		ChannelType: group.ChannelType, TestModel: group.TestModel,
		ValidationEndpoint: group.ValidationEndpoint,
	}
}

func validationStatusCode(message string) int {
	message = strings.TrimSpace(message)
	if !strings.HasPrefix(message, "[status ") {
		return 0
	}
	var statusCode int
	_, _ = fmt.Sscanf(message, "[status %d]", &statusCode)
	return statusCode
}
