package migrations

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const ID0033 = "0033_resource_bindings"

type resourceBinding0033 struct {
	ID                 uint   `gorm:"primaryKey;autoIncrement"`
	AccessKeyID        uint   `gorm:"not null;check:chk_resource_bindings_access_key,access_key_id > 0;uniqueIndex:idx_resource_bindings_identity,priority:1"`
	ResourceHash       string `gorm:"type:char(64);not null;uniqueIndex:idx_resource_bindings_identity,priority:2"`
	ResourceKey        string `gorm:"type:text;not null"`
	GroupID            uint   `gorm:"not null;check:chk_resource_bindings_group,group_id > 0"`
	CredentialID       uint   `gorm:"not null;check:chk_resource_bindings_credential,credential_id > 0"`
	IdentityGeneration uint64 `gorm:"not null;check:chk_resource_bindings_generation,identity_generation > 0"`
	ExpiresAtMS        int64  `gorm:"column:expires_at_ms;not null;check:chk_resource_bindings_expiry,expires_at_ms > 0;index:idx_resource_bindings_expiry"`
}

func (resourceBinding0033) TableName() string { return "resource_bindings" }

// Up0033 为文件、批任务等原生资源保存访问令牌归属，进程崩溃或内存淘汰后仍能找回凭据。
func Up0033(db *gorm.DB) error {
	if Validate0033(db) == nil {
		return nil
	}
	if err := ValidateRecoverable0033(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(&resourceBinding0033{}); err != nil {
		return fmt.Errorf("create resource bindings: %w", err)
	}
	return Validate0033(db)
}

func ValidateRecoverable0033(db *gorm.DB) error {
	if !db.Migrator().HasTable(&resourceBinding0033{}) {
		return nil
	}
	if Validate0033(db) == nil {
		return nil
	}
	var count int64
	if err := db.Model(&resourceBinding0033{}).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("incomplete resource binding schema contains data")
	}
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(&resourceBinding0033{}); err != nil {
		return err
	}
	columns, err := db.Migrator().ColumnTypes(&resourceBinding0033{})
	if err != nil {
		return err
	}
	for _, column := range columns {
		if statement.Schema.LookUpField(column.Name()) == nil {
			return fmt.Errorf("resource bindings contains unexpected column %q", column.Name())
		}
	}
	return nil
}

func Validate0033(db *gorm.DB) error {
	model := &resourceBinding0033{}
	if !db.Migrator().HasTable(model) {
		return fmt.Errorf("resource bindings table is missing")
	}
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(model); err != nil {
		return err
	}
	columns, err := db.Migrator().ColumnTypes(model)
	if err != nil {
		return err
	}
	found := make(map[string]bool)
	for _, column := range columns {
		field := statement.Schema.LookUpField(column.Name())
		if field == nil {
			return fmt.Errorf("resource bindings contains unexpected column %q", column.Name())
		}
		found[column.Name()] = true
		integer := field.DataType == "int" || field.DataType == "uint"
		if integer && !strings.Contains(strings.ToLower(column.DatabaseTypeName()), "int") {
			return fmt.Errorf("resource bindings column %s must be integer", column.Name())
		}
		if field.NotNull {
			if nullable, known := column.Nullable(); known && nullable {
				return fmt.Errorf("resource bindings column %s must be non-null", column.Name())
			}
		}
	}
	for _, field := range statement.Schema.Fields {
		if !found[field.DBName] {
			return fmt.Errorf("resource bindings column %s is missing", field.DBName)
		}
	}
	for _, index := range statement.Schema.ParseIndexes() {
		if !db.Migrator().HasIndex(model, index.Name) {
			return fmt.Errorf("resource bindings index %s is missing", index.Name)
		}
	}
	for name := range statement.Schema.ParseCheckConstraints() {
		if !db.Migrator().HasConstraint(model, name) {
			return fmt.Errorf("resource bindings constraint %s is missing", name)
		}
	}
	return nil
}
