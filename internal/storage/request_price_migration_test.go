package storage

import (
	"fmt"
	"gorm.io/gorm"
	"testing"

	"gpt-load/internal/storage/models"
)

func TestRequestPriceMigrationPreservesOldPricesAndRequestChildren(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(fmt.Sprint(interrupted), func(t *testing.T) {
			db := openInternalMigrationTestDatabase(t)
			if err := applyMigrationRegistry(db, migrations[:29]); err != nil {
				t.Fatal(err)
			}
			if err := db.Table("model_prices").Create(map[string]any{"channel_id": "openai", "model_id": "legacy", "input_price_nano_usd_per_million_tokens": int64(123), "is_manual": true, "created_at_ms": 0, "updated_at_ms": 0}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Table("request_logs").Create(map[string]any{"id": "legacy-request", "completed_at_ms": 0, "access_key_id": 1, "protocol": "openai-completions", "client_model": "legacy", "upstream_model": "legacy", "status": "success", "status_code": 200, "duration_ms": 0, "error_summary": ""}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&models.RequestLogAttempt{RequestID: "legacy-request", Sequence: 1, GroupID: 1, GroupName: "legacy", CredentialID: 1, FailureCategory: "ok", Action: "terminate"}).Error; err != nil {
				t.Fatal(err)
			}
			if interrupted {
				entry := migrations[29]
				up := entry.Up
				entry.Up = func(tx *gorm.DB) error {
					if err := up(tx); err != nil {
						return err
					}
					return fmt.Errorf("interrupt request price migration")
				}
				registry := append(append([]migration(nil), migrations[:29]...), entry)
				if applyMigrationRegistry(db, registry) == nil {
					t.Fatal("interrupted migration succeeded")
				}
			}
			if err := AutoMigrate(db); err != nil {
				t.Fatal(err)
			}
			if err := AutoMigrate(db); err != nil {
				t.Fatal(err)
			}
			var price models.ModelPrice
			if err := db.Where("model_id = ?", "legacy").Take(&price).Error; err != nil {
				t.Fatal(err)
			}
			if price.BillingUnit != "token" || price.RequestPriceNanoUSD != nil || price.InputPriceNanoUSDPerMillionTokens == nil || *price.InputPriceNanoUSDPerMillionTokens != 123 {
				t.Fatalf("legacy price = %#v", price)
			}
			var count int64
			if err := db.Model(&models.RequestLogAttempt{}).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("request children = %d, %v", count, err)
			}
			if !db.Migrator().HasIndex("request_logs", "idx_request_logs_completed_id") {
				t.Fatal("request log index lost")
			}
			if err := db.Model(&models.ModelPrice{}).Where("id = ?", price.ID).Update("request_price_nano_usd", int64(-1)).Error; err == nil {
				t.Fatal("negative request price accepted")
			}
		})
	}
}
