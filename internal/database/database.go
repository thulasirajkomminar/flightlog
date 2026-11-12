// Package database opens and migrates the SQLite database.
package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thulasirajkomminar/flightlog/internal/airport"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const (
	dataDirPerm = 0o750
	// dsnOptions enables write-ahead logging and waits on a locked database
	// instead of failing immediately.
	dsnOptions = "?_journal_mode=WAL&_busy_timeout=5000"
)

// Open creates the data directory if needed, opens the SQLite database at
// path, and brings its schema up to date.
func Open(path string) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		err := os.MkdirAll(dir, dataDirPerm)
		if err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	db, err := gorm.Open(sqlite.Open(path+dsnOptions), &gorm.Config{
		Logger: gormlogger.New(log.New(os.Stderr, "", log.LstdFlags), gormlogger.Config{
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = migrate(db)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func migrate(db *gorm.DB) error {
	err := db.AutoMigrate(&domain.Flight{}, &domain.UserFlight{}, &domain.User{}, &airport.Record{})
	if err != nil {
		return fmt.Errorf("failed to run database migration: %w", err)
	}

	// Route distances are computed from coordinates now; drop the old cache table.
	err = db.Migrator().DropTable("distance_cache")
	if err != nil {
		return fmt.Errorf("failed to drop legacy distance cache: %w", err)
	}

	return nil
}
