package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

const ID0031 = "0031_usage_units"

type requestLog0031 struct {
	MeasuredUnits   *string `gorm:"column:measured_units;type:json"`
	ServiceEvidence *string `gorm:"column:service_evidence;type:json"`
}

func (requestLog0031) TableName() string { return "request_logs" }

// Up0031 仅增加可空 JSON 列，历史记录保持未报告，不回填零。
func Up0031(db *gorm.DB) error {
	if err := ValidateRecoverable0031(db); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&requestLog0031{}, "measured_units") {
		if err := db.Migrator().AddColumn(&requestLog0031{}, "MeasuredUnits"); err != nil {
			return err
		}
	}
	if !db.Migrator().HasColumn(&requestLog0031{}, "service_evidence") {
		if err := db.Migrator().AddColumn(&requestLog0031{}, "ServiceEvidence"); err != nil {
			return err
		}
	}
	return Validate0031(db)
}
func ValidateRecoverable0031(db *gorm.DB) error {
	if !db.Migrator().HasTable(&requestLog0031{}) {
		return fmt.Errorf("request_logs table is missing")
	}
	return nil
}
func Validate0031(db *gorm.DB) error {
	columns, err := db.Migrator().ColumnTypes(&requestLog0031{})
	if err != nil {
		return err
	}
	for _, name := range []string{"measured_units", "service_evidence"} {
		found := false
		for _, column := range columns {
			if column.Name() == name {
				if nullable, known := column.Nullable(); known && !nullable {
					return fmt.Errorf("%s must be nullable", name)
				}
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s column is missing", name)
		}
	}
	return nil
}
