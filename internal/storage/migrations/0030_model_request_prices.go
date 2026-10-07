package migrations

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const ID0030 = "0030_model_request_prices"

type modelPrice0030 struct {
	BillingUnit         string `gorm:"type:varchar(16);not null;default:token;check:chk_model_price_billing_unit,billing_unit IN ('token','request')"`
	RequestPriceNanoUSD *int64 `gorm:"check:chk_model_price_request_nano,request_price_nano_usd IS NULL OR request_price_nano_usd >= 0"`
}

func (modelPrice0030) TableName() string { return "model_prices" }

// Up0030 keeps existing token prices and adds an optional fixed request price.
func Up0030(db *gorm.DB) error {
	if err := ValidateRecoverable0030(db); err != nil {
		return err
	}
	for _, field := range []string{"BillingUnit", "RequestPriceNanoUSD"} {
		if !db.Migrator().HasColumn(&modelPrice0030{}, field) {
			var err error
			if db.Dialector.Name() == "sqlite" {
				statement := "ALTER TABLE model_prices ADD COLUMN billing_unit varchar(16) NOT NULL DEFAULT 'token' CONSTRAINT chk_model_price_billing_unit CHECK (billing_unit IN ('token','request'))"
				if field == "RequestPriceNanoUSD" {
					statement = "ALTER TABLE model_prices ADD COLUMN request_price_nano_usd integer CONSTRAINT chk_model_price_request_nano CHECK (request_price_nano_usd IS NULL OR request_price_nano_usd >= 0)"
				}
				err = db.Exec(statement).Error
			} else {
				err = db.Migrator().AddColumn(&modelPrice0030{}, field)
			}
			if err != nil {
				return fmt.Errorf("add request pricing %s: %w", field, err)
			}
		}
	}
	for _, name := range []string{"chk_model_price_billing_unit", "chk_model_price_request_nano"} {
		if !db.Migrator().HasConstraint(&modelPrice0030{}, name) {
			if db.Dialector.Name() == "sqlite" {
				return fmt.Errorf("request price column lacks %s", name)
			}
			if err := db.Migrator().CreateConstraint(&modelPrice0030{}, name); err != nil {
				return err
			}
		}
	}
	if err := migrateRequestPricingLog0030(db); err != nil {
		return err
	}
	return Validate0030(db)
}

func ValidateRecoverable0030(db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "sqlite", "mysql", "postgres":
	default:
		return fmt.Errorf("unsupported request pricing driver %q", db.Dialector.Name())
	}
	if !db.Migrator().HasTable(&modelPrice0030{}) {
		return fmt.Errorf("model_prices table is missing")
	}
	return validateRequestPriceColumns0030(db, false)
}

func Validate0030(db *gorm.DB) error {
	if err := validateRequestPriceColumns0030(db, true); err != nil {
		return err
	}
	if !db.Migrator().HasColumn("request_logs", "billing_unit") || !db.Migrator().HasConstraint("request_logs", "chk_request_log_billing_unit") {
		return fmt.Errorf("request log billing unit is missing")
	}
	definition, err := requestPricingConstraint0030(db)
	if err != nil {
		return err
	}
	if !upgradedRequestPricing0030(definition) {
		return fmt.Errorf("request log pricing constraint has not been upgraded")
	}
	return nil
}

func validateRequestPriceColumns0030(db *gorm.DB, required bool) error {
	columns, err := db.Migrator().ColumnTypes(&modelPrice0030{})
	if err != nil {
		return err
	}
	found := map[string]bool{}
	for _, column := range columns {
		switch column.Name() {
		case "billing_unit":
			found[column.Name()] = true
			switch strings.ToLower(column.DatabaseTypeName()) {
			case "varchar", "text", "character varying":
			default:
				return fmt.Errorf("billing unit must be text")
			}
			if nullable, known := column.Nullable(); !known || nullable {
				return fmt.Errorf("billing unit must not be nullable")
			}
			value, known := column.DefaultValue()
			if db.Dialector.Name() == "sqlite" {
				if err := db.Raw("SELECT dflt_value FROM pragma_table_info('model_prices') WHERE name = 'billing_unit'").Scan(&value).Error; err != nil {
					return err
				}
				known = value != ""
			}
			if !known || strings.Trim(strings.SplitN(value, "::", 2)[0], "'\"() ") != "token" {
				return fmt.Errorf("billing unit default must be token")
			}
		case "request_price_nano_usd":
			found[column.Name()] = true
			switch strings.ToLower(column.DatabaseTypeName()) {
			case "integer":
				if db.Dialector.Name() != "sqlite" {
					return fmt.Errorf("request price must be a signed 64-bit integer")
				}
			case "bigint", "int8":
			default:
				return fmt.Errorf("request price must be a signed 64-bit integer")
			}
			if kind, known := column.ColumnType(); known && strings.Contains(strings.ToLower(kind), "unsigned") {
				return fmt.Errorf("request price must be signed")
			}
			if nullable, known := column.Nullable(); !known || !nullable {
				return fmt.Errorf("request price must be nullable")
			}
		}
	}
	if required && (!found["billing_unit"] || !found["request_price_nano_usd"]) {
		return fmt.Errorf("request pricing columns are missing")
	}
	return nil
}
