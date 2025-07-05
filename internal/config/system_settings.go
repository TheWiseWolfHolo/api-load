package config

import (
	"fmt"
	"os"
	"gpt-load/internal/db"
	"gpt-load/internal/models"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

// SystemSettings 定义所有系统配置项
// 使用结构体标签作为唯一事实来源
type SystemSettings struct {
	// 基础参数
	AppUrl                  string `json:"app_url" default:"" name:"项目地址" category:"基础参数" desc:"项目的基础 URL，用于拼接分组终端节点地址。系统配置优先于环境变量 APP_URL。"`
	BlacklistThreshold      int    `json:"blacklist_threshold" default:"1" name:"黑名单阈值" category:"基础参数" desc:"一个 Key 连续失败多少次后进入黑名单" validate:"min=0"`
	MaxRetries              int `json:"max_retries" default:"3" name:"最大重试次数" category:"基础参数" desc:"单个请求使用不同 Key 的最大重试次数" validate:"min=0"`
	RequestLogRetentionDays int `json:"request_log_retention_days" default:"30" name:"日志保留天数" category:"基础参数" desc:"请求日志在数据库中的保留天数" validate:"min=1"`

	// 服务超时
	ServerReadTimeout             int `json:"server_read_timeout" default:"120" name:"读取超时" category:"服务超时" desc:"HTTP 服务器读取超时时间（秒）" validate:"min=1"`
	ServerWriteTimeout            int `json:"server_write_timeout" default:"1800" name:"写入超时" category:"服务超时" desc:"HTTP 服务器写入超时时间（秒）" validate:"min=1"`
	ServerIdleTimeout             int `json:"server_idle_timeout" default:"120" name:"空闲超时" category:"服务超时" desc:"HTTP 服务器空闲超时时间（秒）" validate:"min=1"`
	ServerGracefulShutdownTimeout int `json:"server_graceful_shutdown_timeout" default:"60" name:"优雅关闭超时" category:"服务超时" desc:"服务优雅关闭的等待超时时间（秒）" validate:"min=1"`

	// 请求超时
	RequestTimeout  int `json:"request_timeout" default:"30" name:"请求超时" category:"请求超时" desc:"请求处理的总体超时时间（秒）" validate:"min=1"`
	ResponseTimeout int `json:"response_timeout" default:"30" name:"响应超时" category:"请求超时" desc:"TLS 握手和响应头的超时时间（秒）" validate:"min=1"`
	IdleConnTimeout int `json:"idle_conn_timeout" default:"120" name:"空闲连接超时" category:"请求超时" desc:"空闲连接的超时时间（秒）" validate:"min=1"`

	// 密钥验证
	KeyValidationIntervalMinutes    int `json:"key_validation_interval_minutes" default:"60" name:"定时验证周期" category:"密钥验证" desc:"后台定时验证密钥的默认周期（分钟）" validate:"min=5"`
	KeyValidationConcurrency        int `json:"key_validation_concurrency" default:"10" name:"验证并发数" category:"密钥验证" desc:"执行密钥验证时的并发 goroutine 数量" validate:"min=1,max=100"`
	KeyValidationTaskTimeoutMinutes int `json:"key_validation_task_timeout_minutes" default:"60" name:"手动验证超时" category:"密钥验证" desc:"手动触发的全量验证任务的超时时间（分钟）" validate:"min=10"`
}

// GenerateSettingsMetadata 使用反射从 SystemSettings 结构体动态生成元数据
func GenerateSettingsMetadata(s *SystemSettings) []models.SystemSettingInfo {
	var settingsInfo []models.SystemSettingInfo
	v := reflect.ValueOf(s).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
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
func DefaultSystemSettings() SystemSettings {
	s := SystemSettings{}
	v := reflect.ValueOf(&s).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
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
	settings      SystemSettings
	settingsCache map[string]string // Cache for raw string values
	mu            sync.RWMutex
}

var globalSystemSettings *SystemSettingsManager
var once sync.Once

// GetSystemSettingsManager 获取全局系统配置管理器单例
func GetSystemSettingsManager() *SystemSettingsManager {
	once.Do(func() {
		globalSystemSettings = &SystemSettingsManager{}
	})
	return globalSystemSettings
}

// InitializeSystemSettings 初始化系统配置到数据库
func (sm *SystemSettingsManager) InitializeSystemSettings() error {
	if db.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	defaultSettings := DefaultSystemSettings()
	metadata := GenerateSettingsMetadata(&defaultSettings)

	for _, meta := range metadata {
		var existing models.SystemSetting
		err := db.DB.Where("setting_key = ?", meta.Key).First(&existing).Error
		if err != nil { // Not found
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

	// 加载配置到内存
	return sm.LoadFromDatabase()
}

// LoadFromDatabase 从数据库加载系统配置到内存
func (sm *SystemSettingsManager) LoadFromDatabase() error {
	if db.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	var settings []models.SystemSetting
	if err := db.DB.Find(&settings).Error; err != nil {
		return fmt.Errorf("failed to load system settings: %w", err)
	}

	settingsMap := make(map[string]string)
	for _, setting := range settings {
		settingsMap[setting.SettingKey] = setting.SettingValue
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.settingsCache = settingsMap

	// 使用默认值，然后用数据库中的值覆盖
	sm.settings = DefaultSystemSettings()
	sm.mapToStruct(settingsMap, &sm.settings)

	logrus.Info("System settings loaded from database")
	return nil
}

// GetSettings 获取当前系统配置
func (sm *SystemSettingsManager) GetSettings() SystemSettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings
}

// GetAppUrl returns the effective App URL.
// It prioritizes the value from system settings (database) over the APP_URL environment variable.
func (sm *SystemSettingsManager) GetAppUrl() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 1. 优先级: 数据库中的系统配置
	if sm.settings.AppUrl != "" {
		return sm.settings.AppUrl
	}

	// 2. 回退: 环境变量
	return os.Getenv("APP_URL")
}

// UpdateSettings 更新系统配置
func (sm *SystemSettingsManager) UpdateSettings(settingsMap map[string]any) error {
	if db.DB == nil {
		return fmt.Errorf("database not initialized")
	}

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

	// 重新加载配置到内存
	return sm.LoadFromDatabase()
}

// GetEffectiveConfig 获取有效配置 (系统配置 + 分组覆盖)
func (sm *SystemSettingsManager) GetEffectiveConfig(groupConfig datatypes.JSONMap) SystemSettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 从系统配置开始
	effectiveConfig := sm.settings
	v := reflect.ValueOf(&effectiveConfig).Elem()
	t := v.Type()

	// 创建一个从 json 标签到字段名的映射
	jsonToField := make(map[string]string)
	for i := 0; i < t.NumField(); i++ {
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
func (sm *SystemSettingsManager) DisplayCurrentSettings() {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	logrus.Info("Current System Settings:")
	logrus.Infof("   App URL: %s", sm.settings.AppUrl)
	logrus.Infof("   Blacklist threshold: %d", sm.settings.BlacklistThreshold)
	logrus.Infof("   Max retries: %d", sm.settings.MaxRetries)
	logrus.Infof("   Server timeouts: read=%ds, write=%ds, idle=%ds, shutdown=%ds",
		sm.settings.ServerReadTimeout, sm.settings.ServerWriteTimeout,
		sm.settings.ServerIdleTimeout, sm.settings.ServerGracefulShutdownTimeout)
	logrus.Infof("   Request timeouts: request=%ds, response=%ds, idle_conn=%ds",
		sm.settings.RequestTimeout, sm.settings.ResponseTimeout, sm.settings.IdleConnTimeout)
	logrus.Infof("   Request log retention: %d days", sm.settings.RequestLogRetentionDays)
	logrus.Infof("   Key validation: interval=%dmin, concurrency=%d, task_timeout=%dmin",
		sm.settings.KeyValidationIntervalMinutes, sm.settings.KeyValidationConcurrency, sm.settings.KeyValidationTaskTimeoutMinutes)
}

// 辅助方法

func (sm *SystemSettingsManager) mapToStruct(m map[string]string, s *SystemSettings) {
	v := reflect.ValueOf(s).Elem()
	t := v.Type()

	// 创建一个从 json 标签到字段名的映射
	jsonToField := make(map[string]string)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonTag != "" {
			jsonToField[jsonTag] = field.Name
		}
	}

	for key, valStr := range m {
		if fieldName, ok := jsonToField[key]; ok {
			fieldValue := v.FieldByName(fieldName)
			if fieldValue.IsValid() && fieldValue.CanSet() {
				if err := setFieldFromString(fieldValue, valStr); err != nil {
					logrus.Warnf("Failed to set value from map for field %s: %v", fieldName, err)
				}
			}
		}
	}
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
	case int:
		return v, nil
	case float64:
		// JSON unmarshals numbers into float64
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
