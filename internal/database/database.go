// Package database opens and migrates the SQLite database.
package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thulasirajkomminar/flightlog/internal/airport"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/importer"
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

// appliedMigration records a one-time migration that has run on this database.
type appliedMigration struct {
	Version   int    `gorm:"primaryKey;autoIncrement:false"`
	Name      string `gorm:"not null"`
	AppliedAt time.Time
}

// TableName overrides the GORM table name.
func (*appliedMigration) TableName() string {
	return "schema_migrations"
}

// migration is a one-time change to existing data or schema. Additive changes
// (new tables and columns) need none: AutoMigrate handles those on every start.
type migration struct {
	name string
	run  func(*gorm.DB) error
}

// migrations lists the one-time migrations in the order they must run. A
// migration's version is its position in this list, starting at 1. Append new
// ones at the end; never change, remove or reorder existing entries, because
// databases in the field have already run them.
func migrations() []migration {
	return []migration{
		{"drop the route distance cache", dropDistanceCache},
		{"drop unused flight columns", dropLegacyFlightColumns},
		{"assign imported flights to their user", assignImportedFlights},
	}
}

func migrate(db *gorm.DB) error {
	err := db.AutoMigrate(&domain.Flight{}, &domain.FlightMiss{}, &domain.UserFlight{}, &domain.User{},
		&airport.Record{}, &importer.JobRecord{}, &appliedMigration{})
	if err != nil {
		return fmt.Errorf("failed to run database migration: %w", err)
	}

	for i, m := range migrations() {
		err = applyOnce(db, i+1, m)
		if err != nil {
			return err
		}
	}

	return nil
}

// applyOnce runs a migration unless this database has already had it. The
// change and its record are written together, so a failed migration is retried
// on the next start instead of being half-applied.
func applyOnce(db *gorm.DB, version int, m migration) error {
	var done int64

	err := db.Model(&appliedMigration{}).Where("version = ?", version).Count(&done).Error
	if err != nil {
		return fmt.Errorf("failed to check migration %d: %w", version, err)
	}

	if done > 0 {
		return nil
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		err := m.run(tx)
		if err != nil {
			return err
		}

		return tx.Create(&appliedMigration{Version: version, Name: m.name, AppliedAt: time.Now()}).Error
	})
	if err != nil {
		return fmt.Errorf("migration %d (%s) failed: %w", version, m.name, err)
	}

	return nil
}

// dropDistanceCache removes the table that cached route distances; they are
// computed from coordinates now.
func dropDistanceCache(db *gorm.DB) error {
	err := db.Migrator().DropTable("distance_cache")
	if err != nil {
		return fmt.Errorf("failed to drop legacy distance cache: %w", err)
	}

	return nil
}

// assignImportedFlights makes flights that were imported before imports
// became private belong to the one user who has them. A flight that several
// users already share stays shared.
func assignImportedFlights(db *gorm.DB) error {
	err := db.Exec(`
		UPDATE flights
		SET imported_by = (SELECT MIN(user_id) FROM user_flights WHERE flight_id = flights.id)
		WHERE imported_by = ''
			AND provider NOT IN ('aerodatabox', 'seed')
			AND (SELECT COUNT(DISTINCT user_id) FROM user_flights WHERE flight_id = flights.id) = 1
	`).Error
	if err != nil {
		return fmt.Errorf("failed to assign imported flights to their users: %w", err)
	}

	return nil
}

// legacyFlightColumns are columns of the flights table that are no longer
// used: the full provider response, which was never read, and the distance in
// units that are now derived from kilometres.
func legacyFlightColumns() []string {
	return []string{"raw_data", "gcd_meter", "gcd_mile", "gcd_nm", "gcd_feet"}
}

func dropLegacyFlightColumns(db *gorm.DB) error {
	for _, column := range legacyFlightColumns() {
		if !db.Migrator().HasColumn(&domain.Flight{}, column) {
			continue
		}

		// The names come from the fixed list above, never from user input.
		err := db.Exec("ALTER TABLE flights DROP COLUMN " + column).Error
		if err != nil {
			return fmt.Errorf("failed to drop legacy %s column: %w", column, err)
		}
	}

	return nil
}
