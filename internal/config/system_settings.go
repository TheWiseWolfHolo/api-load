package config

import (
	"encoding/json"
	"fmt"
	"gpt-load/internal/db"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"gpt-load/internal/syncer"
	"gpt-load/internal/types"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

const SettingsUpdateChannel = "system_settings:updated"

// GenerateSettingsMetadata 使用反射从 SystemSettings 结构体动态生成元数据
func GenerateSettingsMetadata(s *types.SystemSettings) []models.SystemSettingInfo {
	var settingsInfo []models.SystemSettingInfo
	v := reflect.ValueOf(s).Elem()
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		fieldValue := v.Field(i)

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			continue
		}

		nameTag := field.Tag.Get("name")
		descTag := field.Tag.Get("desc")
		defaultTag := field.Tag.Get("default")
		validateTag := field.Tag.Get("validate")
		categoryTag := field.Tag.Get("category")

		var minValue *int
		if strings.HasPrefix(validateTag, "min=") {
			valStr := strings.TrimPrefix(validateTag, "min=")
			if val, err := strconv.Atoi(valStr); err == nil {
				minValue = &val
			}
		}

		info := models.SystemSettingInfo{
			Key:          jsonTag,
			Name:         nameTag,
			Value:        fieldValue.Interface(),
			Type:         field.Type.String(),
			DefaultValue: defaultTag,
			Description:  descTag,
			Category:     categoryTag,
			MinValue:     minValue,
		}
		settingsInfo = append(settingsInfo, info)
	}
	return settingsInfo
}

// DefaultSystemSettings 返回默认的系统配置
func DefaultSystemSettings() types.SystemSettings {
	s := types.SystemSettings{}
	v := reflect.ValueOf(&s).Elem()
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		defaultTag := field.Tag.Get("default")
		if defaultTag == "" {
			continue
		}

		fieldValue := v.Field(i)
		if fieldValue.CanSet() {
			if err := setFieldFromString(fieldValue, defaultTag); err != nil {
				logrus.Warnf("Failed to set default value for field %s: %v", field.Name, err)
			}
		}
	}
	return s
}

// SystemSettingsManager 管理系统配置
type SystemSettingsManager struct {
	syncer *syncer.CacheSyncer[types.SystemSettings]
}

// NewSystemSettingsManager creates a new, uninitialized SystemSettingsManager.
func NewSystemSettingsManager() *SystemSettingsManager {
	return &SystemSettingsManager{}
}

type gm interface {
	Invalidate() error
}

// Initialize initializes the SystemSettingsManager with database and store dependencies.
func (sm *SystemSettingsManager) Initialize(store store.Store, gm gm) error {
	settingsLoader := func() (types.SystemSettings, error) {
		var dbSettings []models.SystemSetting
		if err := db.DB.Find(&dbSettings).Error; err != nil {
			return types.SystemSettings{}, fmt.Errorf("failed to load system settings from db: %w", err)
		}

		settingsMap := make(map[string]string)
		for _, setting := range dbSettings {
			settingsMap[setting.SettingKey] = setting.SettingValue
		}

		// Start with default settings, then override with values from the database.
		settings := DefaultSystemSettings()
		v := reflect.ValueOf(&settings).Elem()
		t := v.Type()
		jsonToField := make(map[string]string)
		for i := range t.NumField() {
			field := t.Field(i)
			jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
			if jsonTag != "" {
				jsonToField[jsonTag] = field.Name
			}
		}

		for key, valStr := range settingsMap {
			if fieldName, ok := jsonToField[key]; ok {
				fieldValue := v.FieldByName(fieldName)
				if fieldValue.IsValid() && fieldValue.CanSet() {
					if err := setFieldFromString(fieldValue, valStr); err != nil {
						logrus.Warnf("Failed to set value from map for field %s: %v", fieldName, err)
					}
				}
			}
		}

		sm.DisplayCurrentSettings(settings)

		return settings, nil
	}

	afterLoader := func(newData types.SystemSettings) {
		if err := gm.Invalidate(); err != nil {
			logrus.Debugf("Failed to invalidate group manager cache after settings update: %v", err)
		}
	}

	syncer, err := syncer.NewCacheSyncer(
		settingsLoader,
		store,
		SettingsUpdateChannel,
		logrus.WithField("syncer", "system_settings"),
		afterLoader,
	)
	if err != nil {
		return fmt.Errorf("failed to create system settings syncer: %w", err)
	}

	sm.syncer = syncer
	return nil
}

// Stop gracefully stops the SystemSettingsManager's background syncer.
func (sm *SystemSettingsManager) Stop() {
	if sm.syncer != nil {
		sm.syncer.Stop()
	}
}

// EnsureSettingsInitialized 确保数据库中存在所有系统设置的记录。
func (sm *SystemSettingsManager) EnsureSettingsInitialized() error {
	defaultSettings := DefaultSystemSettings()
	metadata := GenerateSettingsMetadata(&defaultSettings)

	for _, meta := range metadata {
		var existing models.SystemSetting
		err := db.DB.Where("setting_key = ?", meta.Key).First(&existing).Error
		if err != nil {
			value := fmt.Sprintf("%v", meta.DefaultValue)
			if meta.Key == "app_url" {
				// Special handling for app_url initialization
				if appURL := os.Getenv("APP_URL"); appURL != "" {
					value = appURL
				} else {
					host := os.Getenv("HOST")
					if host == "" || host == "0.0.0.0" {
						host = "localhost"
					}
					port := os.Getenv("PORT")
					if port == "" {
						port = "3000"
					}
					value = fmt.Sprintf("http://%s:%s", host, port)
				}
			}
			setting := models.SystemSetting{
				SettingKey:   meta.Key,
				SettingValue: value,
				Description:  meta.Description,
			}
			if err := db.DB.Create(&setting).Error; err != nil {
				logrus.Errorf("Failed to initialize setting %s: %v", setting.SettingKey, err)
				return err
			}
			logrus.Infof("Initialized system setting: %s = %s", setting.SettingKey, setting.SettingValue)
		}
	}

	return nil
}

// GetSettings 获取当前系统配置
// If the syncer is not initialized, it returns default settings.
func (sm *SystemSettingsManager) GetSettings() types.SystemSettings {
	if sm.syncer == nil {
		logrus.Warn("SystemSettingsManager is not initialized, returning default settings.")
		return DefaultSystemSettings()
	}
	return sm.syncer.Get()
}

// GetAppUrl returns the effective App URL.
// It prioritizes the value from system settings (database) over the APP_URL environment variable.
func (sm *SystemSettingsManager) GetAppUrl() string {
	// 1. 优先级: 数据库中的系统配置
	settings := sm.GetSettings()
	if settings.AppUrl != "" {
		return settings.AppUrl
	}

	// 2. 回退: 环境变量
	return os.Getenv("APP_URL")
}

// UpdateSettings 更新系统配置
func (sm *SystemSettingsManager) UpdateSettings(settingsMap map[string]any) error {
	// 验证配置项
	if err := sm.ValidateSettings(settingsMap); err != nil {
		return err
	}

	// 更新数据库
	var settingsToUpdate []models.SystemSetting
	for key, value := range settingsMap {
		settingsToUpdate = append(settingsToUpdate, models.SystemSetting{
			SettingKey:   key,
			SettingValue: fmt.Sprintf("%v", value), // Convert any to string
		})
	}

	if len(settingsToUpdate) > 0 {
		if err := db.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "setting_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"setting_value", "updated_at"}),
		}).Create(&settingsToUpdate).Error; err != nil {
			return fmt.Errorf("failed to update system settings: %w", err)
		}
	}

	// 触发所有实例重新加载
	return sm.syncer.Invalidate()
}

// GetEffectiveConfig 获取有效配置 (系统配置 + 分组覆盖)
func (sm *SystemSettingsManager) GetEffectiveConfig(groupConfig datatypes.JSONMap) types.SystemSettings {
	// 从系统配置开始
	effectiveConfig := sm.GetSettings()
	v := reflect.ValueOf(&effectiveConfig).Elem()
	t := v.Type()

	// 创建一个从 json 标签到字段名的映射
	jsonToField := make(map[string]string)
	for i := range t.NumField() {
		field := t.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonTag != "" {
			jsonToField[jsonTag] = field.Name
		}
	}

	// 应用分组配置覆盖
	for key, val := range groupConfig {
		if fieldName, ok := jsonToField[key]; ok {
			fieldValue := v.FieldByName(fieldName)
			if fieldValue.IsValid() && fieldValue.CanSet() {
				switch fieldValue.Kind() {
				case reflect.Int:
					if intVal, err := interfaceToInt(val); err == nil {
						fieldValue.SetInt(int64(intVal))
					}
				case reflect.String:
					if strVal, ok := interfaceToString(val); ok {
						fieldValue.SetString(strVal)
					}
				case reflect.Bool:
					if boolVal, ok := interfaceToBool(val); ok {
						fieldValue.SetBool(boolVal)
					}
				}
			}
		}
	}

	return effectiveConfig
}

// ValidateSettings 验证系统配置的有效性
func (sm *SystemSettingsManager) ValidateSettings(settingsMap map[string]any) error {
	tempSettings := DefaultSystemSettings()
	v := reflect.ValueOf(&tempSettings).Elem()
	t := v.Type()
	jsonToField := make(map[string]reflect.StructField)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			jsonToField[jsonTag] = field
		}
	}

	for key, value := range settingsMap {
		field, ok := jsonToField[key]
		if !ok {
			return fmt.Errorf("invalid setting key: %s", key)
		}

		validateTag := field.Tag.Get("validate")

		switch field.Type.Kind() {
		case reflect.Int:
			// JSON numbers are decoded as float64
			floatVal, ok := value.(float64)
			if !ok {
				return fmt.Errorf("invalid type for %s: expected a number, got %T", key, value)
			}
			intVal := int(floatVal)
			if floatVal != float64(intVal) {
				return fmt.Errorf("invalid value for %s: must be an integer", key)
			}

			if strings.HasPrefix(validateTag, "min=") {
				minValStr := strings.TrimPrefix(validateTag, "min=")
				minVal, _ := strconv.Atoi(minValStr)
				if intVal < minVal {
					return fmt.Errorf("value for %s (%d) is below minimum value (%d)", key, intVal, minVal)
				}
			}
		case reflect.Bool:
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("invalid type for %s: expected a boolean, got %T", key, value)
			}
		case reflect.String:
			if _, ok := value.(string); !ok {
				return fmt.Errorf("invalid type for %s: expected a string, got %T", key, value)
			}
		default:
			return fmt.Errorf("unsupported type for setting key validation: %s", key)
		}
	}

	return nil
}

// DisplayCurrentSettings 显示当前系统配置信息
func (sm *SystemSettingsManager) DisplayCurrentSettings(settings types.SystemSettings) {
	logrus.Info("Current System Settings:")
	logrus.Infof("   App URL: %s", settings.AppUrl)
	logrus.Infof("   Blacklist threshold: %d", settings.BlacklistThreshold)
	logrus.Infof("   Max retries: %d", settings.MaxRetries)
	logrus.Infof("   Server timeouts: read=%ds, write=%ds, idle=%ds, shutdown=%ds",
		settings.ServerReadTimeout, settings.ServerWriteTimeout,
		settings.ServerIdleTimeout, settings.ServerGracefulShutdownTimeout)
	logrus.Infof("   Request timeouts: request=%ds, connect=%ds, idle_conn=%ds",
		settings.RequestTimeout, settings.ConnectTimeout, settings.IdleConnTimeout)
	logrus.Infof("   HTTP Client Pool: max_idle_conns=%d, max_idle_conns_per_host=%d",
		settings.MaxIdleConns, settings.MaxIdleConnsPerHost)
	logrus.Infof("   Request log retention: %d days", settings.RequestLogRetentionDays)
	logrus.Infof("   Key validation: interval=%dmin, task_timeout=%dmin",
		settings.KeyValidationIntervalMinutes, settings.KeyValidationTaskTimeoutMinutes)
}

// setFieldFromString sets a struct field's value from a string, based on the field's kind.
func setFieldFromString(fieldValue reflect.Value, value string) error {
	if !fieldValue.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	switch fieldValue.Kind() {
	case reflect.Int:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value '%s': %w", value, err)
		}
		fieldValue.SetInt(int64(intVal))
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value '%s': %w", value, err)
		}
		fieldValue.SetBool(boolVal)
	case reflect.String:
		fieldValue.SetString(value)
	default:
		return fmt.Errorf("unsupported field kind: %s", fieldValue.Kind())
	}
	return nil
}

// 工具函数

func interfaceToInt(val any) (int, error) {
	switch v := val.(type) {
	case json.Number:
		i64, err := v.Int64()
		if err != nil {
			return 0, err
		}
		return int(i64), nil
	case int:
		return v, nil
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("value is a float, not an integer: %v", v)
		}
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

// interfaceToString is kept for GetEffectiveConfig
func interfaceToString(val any) (string, bool) {
	s, ok := val.(string)
	return s, ok
}

// interfaceToBool is kept for GetEffectiveConfig
func interfaceToBool(val any) (bool, bool) {
	switch v := val.(type) {
	case json.Number:
		if s := v.String(); s == "1" {
			return true, true
		} else if s == "0" {
			return false, true
		}
	case bool:
		return v, true
	case string:
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b, true
		}
	}
	return false, false
}
