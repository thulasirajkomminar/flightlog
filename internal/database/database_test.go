package database

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

func TestOpenRemovesLegacySchemaAndKeepsData(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "data", "flightlog.db")

	// First start creates the current schema.
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Recreate what an older version left behind.
	legacy := []string{
		"ALTER TABLE flights ADD COLUMN raw_data text",
		"CREATE TABLE distance_cache (route_key text PRIMARY KEY, km real)",
		"INSERT INTO flights (id, flight_number, flight_date, raw_data) VALUES ('f1', 'KL1001', '2026-01-01', '{}')",
	}
	for _, stmt := range legacy {
		err = db.Exec(stmt).Error
		if err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	closeDB(t, db)

	db, err = Open(path)
	if err != nil {
		t.Fatalf("Open on legacy database: %v", err)
	}

	defer closeDB(t, db)

	if db.Migrator().HasColumn(&domain.Flight{}, "raw_data") {
		t.Error("raw_data column still exists")
	}

	if db.Migrator().HasTable("distance_cache") {
		t.Error("distance_cache table still exists")
	}

	var f domain.Flight

	err = db.First(&f, "id = ?", "f1").Error
	if err != nil || f.Number != "KL1001" {
		t.Errorf("flight after migration = %+v, err = %v; want it preserved", f, err)
	}
}

func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("DB: %v", err)
	}

	err = sqlDB.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
}
