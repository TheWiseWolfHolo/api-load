package db

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type schemaMigration struct {
	MigrationID string    `gorm:"column:migration_id;type:varchar(128);primaryKey"`
	AppliedAt   time.Time `gorm:"column:applied_at;not null"`
}

func (schemaMigration) TableName() string {
	return "api_load_schema_migrations"
}

type databaseMigration struct {
	id  string
	run func(*gorm.DB) error
}

var databaseMigrations = []databaseMigration{
	{id: "v1_0_22_drop_retries_column", run: V1_0_22_DropRetriesColumn},
	{id: "v1_1_0_add_key_hash_column", run: V1_1_0_AddKeyHashColumn},
	{id: "v1_2_0_split_credential_enablement", run: V1_2_0_SplitCredentialEnablement},
	{id: "v1_3_0_share_pool_credentials_across_endpoints", run: V1_3_0_SharePoolCredentialsAcrossEndpoints},
}

func MigrateDatabase(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("create schema migration ledger: %w", err)
	}

	for _, migration := range databaseMigrations {
		applied, err := isMigrationApplied(db, migration.id)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", migration.id, err)
		}
		if applied {
			logrus.WithField("migration", migration.id).Debug("Database migration already applied")
			continue
		}

		logrus.WithField("migration", migration.id).Info("Applying database migration")
		if err := migration.run(db); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.id, err)
		}

		record := schemaMigration{
			MigrationID: migration.id,
			AppliedAt:   time.Now().UTC(),
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error; err != nil {
			return fmt.Errorf("record migration %s: %w", migration.id, err)
		}
		logrus.WithField("migration", migration.id).Info("Database migration applied")
	}

	return nil
}

func isMigrationApplied(db *gorm.DB, migrationID string) (bool, error) {
	var count int64
	if err := db.Model(&schemaMigration{}).
		Where("migration_id = ?", migrationID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// HandleLegacyIndexes removes old indexes from previous versions to prevent migration errors
func HandleLegacyIndexes(db *gorm.DB) {
	if db.Dialector.Name() == "mysql" {
		var indexCount int64
		db.Raw(`
				SELECT COUNT(*)
				FROM information_schema.STATISTICS
				WHERE TABLE_SCHEMA = DATABASE()
				AND TABLE_NAME = 'api_keys'
				AND INDEX_NAME = 'idx_group_key'
			`).Count(&indexCount)

		if indexCount > 0 {
			db.Exec("ALTER TABLE api_keys DROP INDEX idx_group_key")
		}
		var idxApiKeysGroupKeyCount int64
		db.Raw(`
				SELECT COUNT(*)
				FROM information_schema.STATISTICS
				WHERE TABLE_SCHEMA = DATABASE()
				AND TABLE_NAME = 'api_keys'
				AND INDEX_NAME = 'idx_api_keys_group_id_key_value'
			`).Count(&idxApiKeysGroupKeyCount)

		if idxApiKeysGroupKeyCount > 0 {
			db.Exec("ALTER TABLE api_keys DROP INDEX idx_api_keys_group_id_key_value")
		}
	} else {
		db.Exec("DROP INDEX IF EXISTS idx_group_key")
		db.Exec("DROP INDEX IF EXISTS idx_api_keys_group_id_key_value")
	}
}
