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
		"ALTER TABLE flights ADD COLUMN gcd_meter real",
		"ALTER TABLE flights ADD COLUMN gcd_mile real",
		"ALTER TABLE flights ADD COLUMN gcd_nm real",
		"ALTER TABLE flights ADD COLUMN gcd_feet real",
		"CREATE TABLE distance_cache (route_key text PRIMARY KEY, km real)",
		"INSERT INTO flights (id, flight_number, flight_date, raw_data, gcd_km, gcd_mile) VALUES ('f1', 'KL1001', '2026-01-01', '{}', 100, 62.1371)",
	}
	execAll(t, db, legacy)

	closeDB(t, db)

	db, err = Open(path)
	if err != nil {
		t.Fatalf("Open on legacy database: %v", err)
	}

	defer closeDB(t, db)

	assertLegacySchemaGone(t, db)

	var f domain.Flight

	err = db.First(&f, "id = ?", "f1").Error
	if err != nil || f.Number != "KL1001" {
		t.Errorf("flight after migration = %+v, err = %v; want it preserved", f, err)
	}

	// Kilometres are kept and the other units are derived from them on load.
	if d := f.GreatCircleDistance; d.Km != 100 || d.Meter != 100000 || d.Mile < 62 || d.Mile > 62.2 {
		t.Errorf("distance after migration = %+v, want 100 km with derived units", d)
	}
}

func execAll(t *testing.T, db *gorm.DB, statements []string) {
	t.Helper()

	for _, stmt := range statements {
		err := db.Exec(stmt).Error
		if err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

func assertLegacySchemaGone(t *testing.T, db *gorm.DB) {
	t.Helper()

	for _, column := range legacyFlightColumns() {
		if db.Migrator().HasColumn(&domain.Flight{}, column) {
			t.Errorf("%s column still exists", column)
		}
	}

	if db.Migrator().HasTable("distance_cache") {
		t.Error("distance_cache table still exists")
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
