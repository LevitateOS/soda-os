package store

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open Soda database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("open Soda SQL connection: %w", err)
	}
	// A single connection keeps SQLite PRAGMAs and concurrent daemon access
	// deterministic. Calls still queue safely through database/sql.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err = db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		return nil, fmt.Errorf("configure SQLite busy timeout: %w", err)
	}
	if err = db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, fmt.Errorf("enable SQLite foreign keys: %w", err)
	}
	if err := ensureSchema(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func ensureSchema(db *gorm.DB) error {
	var version int
	if err := db.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		return fmt.Errorf("read Soda schema version: %w", err)
	}
	if version != 0 && version != SchemaVersion {
		return fmt.Errorf("%w: found version %d, expected %d; this Soda OS release requires a fresh installation", ErrUnsupportedSchema, version, SchemaVersion)
	}
	if version == 0 {
		if err := initializeSchema(db, nil); err != nil {
			return err
		}
	}
	return nil
}

func initializeSchema(db *gorm.DB, beforeVersion func(*gorm.DB) error) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&Person{}, &GitIdentity{}, &SSHDeviceKey{}, &Project{}, &Membership{}, &Worktree{}, &ToolchainInstallation{}, &ProjectToolchainResolution{}, &ProvisioningJob{}, &BuiltInGitUser{}, &BuiltInGitKey{}, &BuiltInGitIdentity{}, &BuiltInGitRepository{}); err != nil {
			return fmt.Errorf("create Soda schema: %w", err)
		}
		if beforeVersion != nil {
			if err := beforeVersion(tx); err != nil {
				return err
			}
		}
		if err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)).Error; err != nil {
			return fmt.Errorf("record Soda schema version: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("initialize Soda schema: %w", err)
	}
	return nil
}
