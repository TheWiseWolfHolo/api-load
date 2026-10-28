package control

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"

	"gorm.io/gorm"

	"gpt-load/internal/channel"
	"gpt-load/internal/platform/config"
	app_errors "gpt-load/internal/platform/errors"
	"gpt-load/internal/protocol"
	"gpt-load/internal/state"
	stateloader "gpt-load/internal/state/loader"
	"gpt-load/internal/storage/models"
)

func normalizeValidationModel(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" || len([]byte(normalized)) > 255 {
		return "", app_errors.ErrValidation
	}
	for _, character := range normalized {
		if unicode.IsControl(character) {
			return "", app_errors.ErrValidation
		}
	}
	return normalized, nil
}

func mapGroupRowToState(group models.Group) (state.GroupConfig, error) {
	var storedModels []GroupModel
	if err := decodeGroupDiscoveryJSON(group.Models, &storedModels); err != nil {
		return state.GroupConfig{}, err
	}
	settings := make(config.Settings)
	if len(group.Overrides) > 0 {
		if err := decodeGroupDiscoveryJSON(group.Overrides, &settings); err != nil {
			return state.GroupConfig{}, err
		}
	}
	runtimeModels := make([]state.ModelConfig, 0, len(storedModels))
	for _, model := range storedModels {
		runtimeModels = append(runtimeModels, state.ModelConfig{ID: model.ID, Alias: model.Alias})
	}
	validationModel := ""
	if group.ValidationModel != nil {
		validationModel = *group.ValidationModel
	}
	multiplier := priceMultiplierFromStorage(group.PriceMultiplierMicros)
	result := state.GroupConfig{
		PriceMultiplier:    &multiplier,
		ID:                 group.ID,
		Name:               group.Name,
		ChannelID:          channel.ID(group.ChannelID),
		ConnectionType:     string(group.ConnectionType),
		Params:             append(json.RawMessage(nil), group.Params...),
		ValidationProtocol: protocol.Protocol(stringValue(group.ValidationProtocol)),
		ValidationModel:    validationModel,
		Models:             runtimeModels,
		Settings:           settings,
		Priority:           group.Priority,
		WeightManual:       cloneInt(group.WeightManual), Enabled: group.Enabled,
	}
	return result, nil
}

func validateGroupRowCandidate(
	ctx context.Context,
	tx *gorm.DB,
	group models.Group,
	registry *channel.Registry,
) error {
	candidate, err := mapGroupRowToState(group)
	if err != nil {
		return err
	}
	systemSettings, err := stateloader.LoadSystemSettings(ctx, tx)
	if err != nil {
		return err
	}
	// A fixed transparent credential is a group configuration dependency. The
	// local candidate compile needs that row just as the full publication does.
	var options struct {
		Transparent state.TransparentProxyConfig `json:"transparent_proxy"`
	}
	if len(group.Overrides) > 0 {
		if err := json.Unmarshal(group.Overrides, &options); err != nil {
			return err
		}
	}
	var credentials []state.CredentialConfig
	if options.Transparent.Enabled && options.Transparent.Strategy == "fixed" {
		var credential models.Credential
		if err := tx.WithContext(ctx).Select("id", "group_id", "status", "weight_manual", "secret_version", "identity_fingerprint", "fingerprint").Where("id = ? AND group_id = ?", options.Transparent.FixedCredentialID, group.ID).Take(&credential).Error; err != nil {
			return err
		}
		credentials = []state.CredentialConfig{{ID: credential.ID, GroupID: credential.GroupID, Status: state.CredentialStatus(credential.Status), WeightManual: credential.WeightManual, Version: groupCollectionCredentialVersion(credential.SecretVersion), IdentityGeneration: groupCollectionCredentialIdentity(credential.IdentityFingerprint, group), Fingerprint: credential.Fingerprint}}
	}
	_, err = state.Compile(state.CompileInput{
		SystemSettings:  systemSettings,
		ChannelRegistry: registry,
		Groups:          []state.GroupConfig{candidate},
		Credentials:     credentials,
	})
	return err
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
