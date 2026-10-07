package migrations

import (
	"fmt"
	"strings"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const oldUsagePricing0030 = "(usage_state = 'not_applicable' AND cost_state = 'not_applicable' AND pricing_completeness = 'not_applicable' AND estimated_cost_nano_usd = 0) OR (usage_state = 'missing' AND cost_state = 'unpriced' AND pricing_completeness = 'unavailable' AND estimated_cost_nano_usd = 0) OR (usage_state IN ('complete','partial') AND ((cost_state = 'unpriced' AND pricing_completeness = 'unavailable' AND estimated_cost_nano_usd = 0) OR (cost_state = 'priced' AND pricing_completeness IN ('complete','partial'))))"
const usagePricing0030 = "(billing_unit = 'request' AND ((status = 'success' AND cost_state = 'priced' AND pricing_completeness = 'complete') OR (cost_state = 'unpriced' AND pricing_completeness = 'unavailable' AND estimated_cost_nano_usd = 0) OR (cost_state = 'not_applicable' AND pricing_completeness = 'not_applicable' AND estimated_cost_nano_usd = 0))) OR (billing_unit = 'token' AND (" + oldUsagePricing0030 + "))"

type requestLog0030 struct {
	BillingUnit string `gorm:"type:varchar(16);not null;default:token;check:chk_request_log_billing_unit,billing_unit IN ('token','request')"`
}

func (requestLog0030) TableName() string { return "request_logs" }

func migrateRequestPricingLog0030(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&requestLog0030{}, "billing_unit") {
		var err error
		if db.Dialector.Name() == "sqlite" {
			err = db.Exec("ALTER TABLE request_logs ADD COLUMN billing_unit varchar(16) NOT NULL DEFAULT 'token' CONSTRAINT chk_request_log_billing_unit CHECK (billing_unit IN ('token','request'))").Error
		} else {
			err = db.Migrator().AddColumn(&requestLog0030{}, "BillingUnit")
		}
		if err != nil {
			return err
		}
	}
	if !db.Migrator().HasConstraint(&requestLog0030{}, "chk_request_log_billing_unit") {
		if db.Dialector.Name() == "sqlite" {
			return fmt.Errorf("request log billing unit lacks its check constraint")
		}
		if err := db.Migrator().CreateConstraint(&requestLog0030{}, "chk_request_log_billing_unit"); err != nil {
			return err
		}
	}
	definition, err := requestPricingConstraint0030(db)
	if err != nil {
		return err
	}
	if upgradedRequestPricing0030(definition) {
		return nil
	}
	if db.Dialector.Name() == "sqlite" {
		return rebuildRequestLogs0030(db, definition)
	}
	drop := "DROP CONSTRAINT"
	if dialector, ok := db.Dialector.(*gormmysql.Dialector); ok && dialector.Config != nil && mysqlRequiresCheckDropSyntax0003(dialector.ServerVersion) {
		drop = "DROP CHECK"
	}
	return db.Exec("ALTER TABLE request_logs " + drop + " chk_request_log_usage_pricing_state, ADD CONSTRAINT chk_request_log_usage_pricing_state CHECK (" + usagePricing0030 + ")").Error
}

func upgradedRequestPricing0030(definition string) bool {
	normalized := strings.NewReplacer(" ", "", "\n", "", "\t", "", "`", "", `"`, "", "(", "", ")", "", "::text", "", "_utf8mb4", "", "_utf8mb3", "").Replace(strings.ToLower(definition))
	return strings.Contains(normalized, "billing_unit='request'") && strings.Contains(normalized, "billing_unit='token'") && strings.Contains(normalized, "status='success'")
}

func requestPricingConstraint0030(db *gorm.DB) (string, error) {
	var definition string
	var err error
	switch db.Dialector.Name() {
	case "sqlite":
		err = db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'request_logs'").Scan(&definition).Error
	case "mysql":
		err = db.Raw("SELECT CHECK_CLAUSE FROM information_schema.check_constraints WHERE constraint_schema = DATABASE() AND constraint_name = ?", "chk_request_log_usage_pricing_state").Scan(&definition).Error
	case "postgres":
		err = db.Raw("SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = ? AND conrelid = 'request_logs'::regclass", "chk_request_log_usage_pricing_state").Scan(&definition).Error
	default:
		return "", fmt.Errorf("unsupported request pricing driver")
	}
	if err == nil && definition == "" {
		return "", fmt.Errorf("request pricing constraint is missing")
	}
	return definition, err
}

// The migration runner isolates foreign keys before SQLite table rebuilds.
// Retain every index, trigger, column and child row from the current schema.
func rebuildRequestLogs0030(db *gorm.DB, ddl string) error {
	var foreignKeys int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error; err != nil {
		return err
	}
	if foreignKeys != 0 {
		return fmt.Errorf("request log rebuild requires migration foreign key isolation")
	}
	if strings.Count(ddl, oldUsagePricing0030) != 1 {
		return fmt.Errorf("unexpected prior request pricing constraint")
	}
	start := strings.Index(ddl, "(")
	if start < 0 {
		return fmt.Errorf("invalid request log table definition")
	}
	ddl = "CREATE TABLE request_logs__0030 " + strings.Replace(ddl[start:], oldUsagePricing0030, usagePricing0030, 1)
	var objects []string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE tbl_name = 'request_logs' AND type IN ('index','trigger') AND sql IS NOT NULL ORDER BY type, name").Scan(&objects).Error; err != nil {
		return err
	}
	columns, err := db.Migrator().ColumnTypes("request_logs")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		if strings.ContainsAny(column.Name(), "\"`\x00") {
			return fmt.Errorf("invalid request log column")
		}
		names = append(names, `"`+column.Name()+`"`)
	}
	projection := strings.Join(names, ",")
	for _, statement := range []string{ddl, "INSERT INTO request_logs__0030 (" + projection + ") SELECT " + projection + " FROM request_logs", "DROP TABLE request_logs", "ALTER TABLE request_logs__0030 RENAME TO request_logs"} {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	for _, statement := range objects {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
