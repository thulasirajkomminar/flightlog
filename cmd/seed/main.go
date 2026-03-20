// Package main is the database seed entrypoint.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/thulasirajkomminar/flightlog/internal/airport"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/seed"
)

const dataDirPerm = 0o750

func main() {
	_ = godotenv.Load()

	mode := "seed"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "data/flightlog.db"
	}

	dbPath = filepath.Clean(dbPath)

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	switch mode {
	case "seed":
		seed.Run(db)
	case "reseed":
		seed.Reset(db)
	default:
		log.Fatal("unknown mode: use 'seed' or 'reseed'")
	}
}

func openDB(dbPath string) (*gorm.DB, error) {
	if dir := filepath.Dir(dbPath); dir != "." {
		err := os.MkdirAll(dir, dataDirPerm)
		if err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.AutoMigrate(&domain.Flight{}, &domain.UserFlight{}, &domain.User{}, &airport.Record{}, &airport.DistanceRecord{})
	if err != nil {
		return nil, fmt.Errorf("failed to run migration: %w", err)
	}

	return db, nil
}
