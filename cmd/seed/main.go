// Package main is the database seed entrypoint.
package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/thulasirajkomminar/flightlog/internal/database"
	"github.com/thulasirajkomminar/flightlog/internal/seed"
)

const defaultDBPath = "data/flightlog.db"

func main() {
	_ = godotenv.Load()

	if os.Getenv("ENVIRONMENT") != "development" {
		log.Fatal("seed is only allowed in development (set ENVIRONMENT=development)")
	}

	mode := "seed"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	db, err := database.Open(defaultDBPath)
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
