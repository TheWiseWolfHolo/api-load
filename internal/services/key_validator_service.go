package services

import (
	"context"
	"fmt"
	"gpt-load/internal/channel"
	"gpt-load/internal/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// KeyValidatorService provides methods to validate API keys.
type KeyValidatorService struct {
	DB             *gorm.DB
	channelFactory *channel.Factory
}

// NewKeyValidatorService creates a new KeyValidatorService.
func NewKeyValidatorService(db *gorm.DB, factory *channel.Factory) *KeyValidatorService {
	return &KeyValidatorService{
		DB:             db,
		channelFactory: factory,
	}
}

// ValidateSingleKey performs a validation check on a single API key.
// It does not modify the key's state in the database.
// It returns true if the key is valid, and an error if it's not.
func (s *KeyValidatorService) ValidateSingleKey(ctx context.Context, key *models.APIKey, group *models.Group) (bool, error) {
	// 添加超时保护
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

	// 记录验证开始
	logrus.WithFields(logrus.Fields{
		"key_id":     key.ID,
		"group_id":   group.ID,
		"group_name": group.Name,
	}).Debug("Starting key validation")

	isValid, validationErr := ch.ValidateKey(ctx, key.KeyValue)
	if validationErr != nil {
		logrus.WithFields(logrus.Fields{
			"key_id":     key.ID,
			"group_id":   group.ID,
			"group_name": group.Name,
			"error":      validationErr,
		}).Warn("Key validation failed")
		return false, validationErr
	}

	// 记录验证结果
	logrus.WithFields(logrus.Fields{
		"key_id":     key.ID,
		"group_id":   group.ID,
		"group_name": group.Name,
		"is_valid":   isValid,
	}).Debug("Key validation completed")

	return isValid, nil
}

// TestSingleKeyByID performs a synchronous validation test for a single API key by its ID.
// It is intended for handling user-initiated "Test" actions.
// It does not modify the key's state in the database.
func (s *KeyValidatorService) TestSingleKeyByID(ctx context.Context, keyID uint) (bool, error) {
	var apiKey models.APIKey
	if err := s.DB.First(&apiKey, keyID).Error; err != nil {
		return false, fmt.Errorf("failed to find api key with id %d: %w", keyID, err)
	}

	var group models.Group
	if err := s.DB.First(&group, apiKey.GroupID).Error; err != nil {
		return false, fmt.Errorf("failed to find group with id %d: %w", apiKey.GroupID, err)
	}

	return s.ValidateSingleKey(ctx, &apiKey, &group)
}
