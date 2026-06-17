package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gpt-load/internal/encryption"
	"gpt-load/internal/models"
	"gpt-load/internal/utils"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ExportModePlain      = "plain"
	ExportModeEncrypted  = "encrypted"
	ExportModeMasked     = "masked"
	ExportModeConfigOnly = "config_only"
)

var ErrPlainExportRequiresConfirmation = errors.New("plain export requires confirmation")

type SystemExportOptions struct {
	Mode         string
	ConfirmPlain bool
}

type SystemExportEnvelope struct {
	Version        string              `json:"version"`
	ExportedAt     time.Time           `json:"exported_at"`
	Groups         []SystemExportGroup `json:"groups"`
	ProxyPools     []any               `json:"proxy_pools"`
	SystemSettings []any               `json:"system_settings"`
}

type SystemExportGroup struct {
	Name                string            `json:"name"`
	DisplayName         string            `json:"display_name"`
	Description         string            `json:"description,omitempty"`
	GroupType           string            `json:"group_type"`
	ChannelType         string            `json:"channel_type"`
	Sort                int               `json:"sort"`
	TestModel           string            `json:"test_model,omitempty"`
	ValidationEndpoint  string            `json:"validation_endpoint,omitempty"`
	ProxyKeys           string            `json:"proxy_keys,omitempty"`
	Upstreams           datatypes.JSON    `json:"upstreams,omitempty"`
	ParamOverrides      datatypes.JSONMap `json:"param_overrides,omitempty"`
	Config              datatypes.JSONMap `json:"config,omitempty"`
	HeaderRules         datatypes.JSON    `json:"header_rules,omitempty"`
	ModelRedirectRules  datatypes.JSONMap `json:"model_redirect_rules,omitempty"`
	ModelRedirectStrict bool              `json:"model_redirect_strict"`
	Models              []string          `json:"models,omitempty"`
	ModelMappings       datatypes.JSON    `json:"model_mappings,omitempty"`
	Keys                []SystemExportKey `json:"keys,omitempty"`
}

type SystemExportKey struct {
	Key          string `json:"key,omitempty"`
	EncryptedKey string `json:"encrypted_key,omitempty"`
	MaskedKey    string `json:"masked_key,omitempty"`
	Notes        string `json:"notes"`
	Status       string `json:"status"`
	RequestCount int64  `json:"request_count"`
	FailureCount int64  `json:"failure_count"`
}

type SystemExportService struct {
	db            *gorm.DB
	encryptionSvc encryption.Service
}

func NewSystemExportService(db *gorm.DB, encryptionSvc encryption.Service) *SystemExportService {
	return &SystemExportService{db: db, encryptionSvc: encryptionSvc}
}

func (s *SystemExportService) Export(ctx context.Context, options SystemExportOptions) (SystemExportEnvelope, error) {
	mode := options.Mode
	if mode == "" {
		mode = ExportModeMasked
	}
	if mode == ExportModePlain && !options.ConfirmPlain {
		return SystemExportEnvelope{}, ErrPlainExportRequiresConfirmation
	}

	var groups []models.Group
	if err := s.db.WithContext(ctx).Order("id asc").Find(&groups).Error; err != nil {
		return SystemExportEnvelope{}, err
	}

	envelope := SystemExportEnvelope{
		Version:        "gpt-load-migration/v1",
		ExportedAt:     time.Now().UTC(),
		Groups:         make([]SystemExportGroup, 0, len(groups)),
		ProxyPools:     []any{},
		SystemSettings: []any{},
	}
	for _, group := range groups {
		exportGroup := SystemExportGroup{
			Name:                group.Name,
			DisplayName:         group.DisplayName,
			Description:         group.Description,
			GroupType:           group.GroupType,
			ChannelType:         group.ChannelType,
			Sort:                group.Sort,
			TestModel:           group.TestModel,
			ValidationEndpoint:  group.ValidationEndpoint,
			ProxyKeys:           group.ProxyKeys,
			Upstreams:           cloneJSON(group.Upstreams),
			ParamOverrides:      cloneJSONMap(group.ParamOverrides),
			Config:              cloneJSONMap(group.Config),
			HeaderRules:         cloneJSON(group.HeaderRules),
			ModelRedirectRules:  cloneJSONMap(group.ModelRedirectRules),
			ModelRedirectStrict: group.ModelRedirectStrict,
			ModelMappings:       cloneJSON(group.ModelMappings),
		}
		if len(group.Models) > 0 {
			_ = jsonUnmarshal(group.Models, &exportGroup.Models)
		}
		if mode != ExportModeConfigOnly {
			var keys []models.APIKey
			if err := s.db.WithContext(ctx).Where("group_id = ?", group.ID).Order("id asc").Find(&keys).Error; err != nil {
				return SystemExportEnvelope{}, err
			}
			for _, key := range keys {
				exportGroup.Keys = append(exportGroup.Keys, s.exportKey(key, mode))
			}
		}
		envelope.Groups = append(envelope.Groups, exportGroup)
	}

	return envelope, nil
}

func (s *SystemExportService) exportKey(key models.APIKey, mode string) SystemExportKey {
	exportKey := SystemExportKey{
		Notes:        key.Notes,
		Status:       key.Status,
		RequestCount: key.RequestCount,
		FailureCount: key.FailureCount,
	}
	rawKey := key.KeyValue
	if s.encryptionSvc != nil {
		if decrypted, err := s.encryptionSvc.Decrypt(key.KeyValue); err == nil {
			rawKey = decrypted
		}
	}
	switch mode {
	case ExportModePlain:
		exportKey.Key = rawKey
	case ExportModeEncrypted:
		exportKey.EncryptedKey = key.KeyValue
		exportKey.MaskedKey = utils.MaskAPIKey(rawKey)
	case ExportModeMasked:
		exportKey.MaskedKey = utils.MaskAPIKey(rawKey)
	}
	return exportKey
}

type SystemImportPreview struct {
	NewKeys           int `json:"new_keys"`
	DuplicateKeys     int `json:"duplicate_keys"`
	NotesUpdates      int `json:"notes_updates"`
	NewProxyPools     int `json:"new_proxy_pools"`
	OverwrittenGroups int `json:"overwritten_groups"`
}

type SystemImportService struct {
	db            *gorm.DB
	keyService    *KeyService
	encryptionSvc encryption.Service
}

func NewSystemImportService(db *gorm.DB, keyService *KeyService, encryptionSvc encryption.Service) *SystemImportService {
	return &SystemImportService{db: db, keyService: keyService, encryptionSvc: encryptionSvc}
}

func (s *SystemImportService) Preview(ctx context.Context, envelope SystemExportEnvelope) (SystemImportPreview, error) {
	preview := SystemImportPreview{}
	for _, group := range envelope.Groups {
		var existingGroup models.Group
		groupExists := s.db.WithContext(ctx).Where("name = ?", group.Name).First(&existingGroup).Error == nil
		if groupExists {
			preview.OverwrittenGroups++
		}
		for _, key := range group.Keys {
			if key.Key == "" {
				continue
			}
			keyHash := s.encryptionSvc.Hash(key.Key)
			var existing models.APIKey
			if groupExists && s.db.WithContext(ctx).Where("group_id = ? AND key_hash = ?", existingGroup.ID, keyHash).First(&existing).Error == nil {
				preview.DuplicateKeys++
				if existing.Notes != key.Notes {
					preview.NotesUpdates++
				}
			} else {
				preview.NewKeys++
			}
		}
	}
	return preview, nil
}

func (s *SystemImportService) Import(ctx context.Context, envelope SystemExportEnvelope) error {
	for _, exportGroup := range envelope.Groups {
		group := models.Group{
			Name:                exportGroup.Name,
			DisplayName:         exportGroup.DisplayName,
			Description:         exportGroup.Description,
			GroupType:           exportGroup.GroupType,
			ChannelType:         exportGroup.ChannelType,
			Sort:                exportGroup.Sort,
			TestModel:           exportGroup.TestModel,
			ValidationEndpoint:  exportGroup.ValidationEndpoint,
			ProxyKeys:           exportGroup.ProxyKeys,
			Upstreams:           cloneJSON(exportGroup.Upstreams),
			ParamOverrides:      cloneJSONMap(exportGroup.ParamOverrides),
			Config:              cloneJSONMap(exportGroup.Config),
			HeaderRules:         cloneJSON(exportGroup.HeaderRules),
			ModelRedirectRules:  cloneJSONMap(exportGroup.ModelRedirectRules),
			ModelRedirectStrict: exportGroup.ModelRedirectStrict,
			ModelMappings:       cloneJSON(exportGroup.ModelMappings),
		}
		if group.GroupType == "" {
			group.GroupType = "standard"
		}
		if group.ChannelType == "" {
			group.ChannelType = "openai"
		}
		if group.TestModel == "" {
			group.TestModel = "-"
		}
		if len(group.Upstreams) == 0 {
			group.Upstreams = []byte(`[]`)
		}
		if len(exportGroup.Models) > 0 {
			modelsJSON, err := json.Marshal(exportGroup.Models)
			if err != nil {
				return err
			}
			group.Models = modelsJSON
		}
		var existing models.Group
		if err := s.db.WithContext(ctx).Where("name = ?", exportGroup.Name).First(&existing).Error; err == nil {
			group.ID = existing.ID
			group.CreatedAt = existing.CreatedAt
			if err := s.db.WithContext(ctx).Save(&group).Error; err != nil {
				return err
			}
			group.ID = existing.ID
		} else {
			if err := s.db.WithContext(ctx).Create(&group).Error; err != nil {
				return err
			}
		}
		for _, exportKey := range exportGroup.Keys {
			if exportKey.Key == "" {
				continue
			}
			status := exportKey.Status
			if status == "" {
				status = models.KeyStatusActive
			}
			key := models.APIKey{
				GroupID:      group.ID,
				KeyValue:     exportKey.Key,
				KeyHash:      s.encryptionSvc.Hash(exportKey.Key),
				Notes:        exportKey.Notes,
				Status:       status,
				RequestCount: exportKey.RequestCount,
				FailureCount: exportKey.FailureCount,
			}
			if err := s.keyService.KeyProvider.AddKeys(group.ID, []models.APIKey{key}); err != nil {
				return err
			}
		}
	}
	return nil
}

func jsonUnmarshal(data []byte, target any) error {
	return json.Unmarshal(data, target)
}

func cloneJSON(data datatypes.JSON) datatypes.JSON {
	if len(data) == 0 {
		return nil
	}
	return append(datatypes.JSON(nil), data...)
}

func cloneJSONMap(data datatypes.JSONMap) datatypes.JSONMap {
	if len(data) == 0 {
		return nil
	}
	cloned := make(datatypes.JSONMap, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}
