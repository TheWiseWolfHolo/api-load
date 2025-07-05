package services

import (
	"context"
	"fmt"
	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	"gpt-load/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// KeyTestResult holds the validation result for a single key.
type KeyTestResult struct {
	KeyValue string `json:"key_value"`
	IsValid  bool   `json:"is_valid"`
	Error    string `json:"error,omitempty"`
}

// KeyValidatorService provides methods to validate API keys.
type KeyValidatorService struct {
	DB              *gorm.DB
	channelFactory  *channel.Factory
	SettingsManager *config.SystemSettingsManager
}

// NewKeyValidatorService creates a new KeyValidatorService.
func NewKeyValidatorService(db *gorm.DB, factory *channel.Factory, settingsManager *config.SystemSettingsManager) *KeyValidatorService {
	return &KeyValidatorService{
		DB:              db,
		channelFactory:  factory,
		SettingsManager: settingsManager,
	}
}

// ValidateSingleKey performs a validation check on a single API key.
func (s *KeyValidatorService) ValidateSingleKey(ctx context.Context, key *models.APIKey, group *models.Group) (bool, error) {
	if ctx.Err() != nil {
		return false, fmt.Errorf("context cancelled or timed out: %w", ctx.Err())
	}

	ch, err := s.channelFactory.GetChannel(group)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"group_id":     group.ID,
			"group_name":   group.Name,
			"channel_type": group.ChannelType,
			"error":        err,
		}).Error("Failed to get channel for key validation")
		return false, fmt.Errorf("failed to get channel for group %s: %w", group.Name, err)
	}

	effectiveSettings := s.SettingsManager.GetEffectiveConfig(group.Config)
	retries := effectiveSettings.BlacklistThreshold
	if retries <= 0 {
		retries = 1
	}

	var lastErr error
	for range retries {
		isValid, validationErr := ch.ValidateKey(ctx, key.KeyValue)
		if validationErr == nil && isValid {
			logrus.WithFields(logrus.Fields{
				"key_id":   key.ID,
				"is_valid": isValid,
			}).Debug("Key validation successful")
			return true, nil
		}

		lastErr = validationErr
	}

	logrus.WithFields(logrus.Fields{
		"error":       lastErr,
		"key_id":      key.ID,
		"group_id":    group.ID,
		"max_retries": retries,
	}).Debug("Key validation failed after all retries")

	return false, lastErr
}

// TestMultipleKeys performs a synchronous validation for a list of key values within a specific group.
func (s *KeyValidatorService) TestMultipleKeys(ctx context.Context, group *models.Group, keyValues []string) ([]KeyTestResult, error) {
	results := make([]KeyTestResult, len(keyValues))

	// Find which of the provided keys actually exist in the database for this group
	var existingKeys []models.APIKey
	if err := s.DB.Where("group_id = ? AND key_value IN ?", group.ID, keyValues).Find(&existingKeys).Error; err != nil {
		return nil, fmt.Errorf("failed to query keys from DB: %w", err)
	}
	existingKeyMap := make(map[string]models.APIKey)
	for _, k := range existingKeys {
		existingKeyMap[k.KeyValue] = k
	}

	for i, kv := range keyValues {
		apiKey, exists := existingKeyMap[kv]
		if !exists {
			results[i] = KeyTestResult{
				KeyValue: kv,
				IsValid:  false,
				Error:    "Key does not exist in this group or has been removed.",
			}
			continue
		}

		isValid, validationErr := s.ValidateSingleKey(ctx, &apiKey, group)
		results[i] = KeyTestResult{
			KeyValue: kv,
			IsValid:  isValid,
			Error:    "",
		}
		if validationErr != nil {
			results[i].Error = validationErr.Error()
		}
	}

	return results, nil
}
