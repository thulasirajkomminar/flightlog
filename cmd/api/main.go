// Package main is the Flightlog API entrypoint.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/thulasirajkomminar/flightlog/internal/config"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/flight"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
	"github.com/thulasirajkomminar/flightlog/internal/provider"
	"github.com/thulasirajkomminar/flightlog/internal/provider/aerodatabox"
	"github.com/thulasirajkomminar/flightlog/internal/server"
	"github.com/thulasirajkomminar/flightlog/internal/user"
	"github.com/thulasirajkomminar/flightlog/internal/web"
)

const (
	shutdownTimeout = 10 * time.Second
	dataDirPerm     = 0o750
)

// version is set at build time via ldflags.
var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	appLogger, err := initLogger(cfg)
	if err != nil {
		log.Fatalf("Failed to initialise logger: %v", err)
	}

	defer logger.Sync()

	appLogger.Info("Starting Flightlog API server")

	db, err := initDatabase(cfg, appLogger)
	if err != nil {
		appLogger.Error("Failed to initialise database", err)

		return
	}

	srv, err := buildServer(cfg, db)
	if err != nil {
		appLogger.Error("Failed to build server", err)

		return
	}

	startServer(srv, appLogger)
	awaitShutdown(srv, db, appLogger)
}

func initLogger(cfg *config.Config) (*logger.ComponentLogger, error) {
	logConfig := logger.DefaultConfig()
	logConfig.Environment = cfg.Environment

	err := logger.Init(logConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise logger: %w", err)
	}

	return logger.NewComponentLogger("main"), nil
}

func startServer(srv *http.Server, appLogger *logger.ComponentLogger) {
	go func() {
		appLogger.Info("HTTP server starting on " + srv.Addr)

		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Error("Server listen failed", err)
		}
	}()
}

func awaitShutdown(srv *http.Server, db *gorm.DB, appLogger *logger.ComponentLogger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	err := srv.Shutdown(shutdownCtx)
	if err != nil {
		appLogger.Error("Server forced to shutdown", err)
	}

	closeDatabase(db, appLogger)

	appLogger.Info("Server exited")
}

func closeDatabase(db *gorm.DB, appLogger *logger.ComponentLogger) {
	sqlDB, err := db.DB()
	if err == nil {
		err = sqlDB.Close()
		if err != nil {
			appLogger.Error("Failed to close database", err)
		}
	}
}

func initDatabase(cfg *config.Config, appLogger *logger.ComponentLogger) (*gorm.DB, error) {
	if dir := filepath.Dir(cfg.Database.Path); dir != "." {
		err := os.MkdirAll(dir, dataDirPerm)
		if err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	db, err := gorm.Open(sqlite.Open(cfg.Database.Path+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.AutoMigrate(&domain.Flight{}, &domain.UserFlight{}, &domain.User{})
	if err != nil {
		return nil, fmt.Errorf("failed to run database migration: %w", err)
	}

	appLogger.Info("Database migration completed")

	return db, nil
}

func buildServer(cfg *config.Config, db *gorm.DB) (*http.Server, error) {
	flightCacheStore := flight.NewCacheStore(db)
	userFlightStore := flight.NewUserFlightStore(db)

	aeroProvider, err := aerodatabox.NewProvider(cfg.AeroDataBox.APIKey, cfg.AeroDataBox.BaseURL, cfg.AeroDataBox.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create AeroDataBox provider: %w", err)
	}

	flightService := flight.NewService(flightCacheStore, userFlightStore, aeroProvider)
	flightHandler := flight.NewHandler(flightService)
	providers := map[string]provider.FlightProvider{
		aeroProvider.GetProviderName(): aeroProvider,
	}
	providerHandler := provider.NewHandler(providers)

	userStore := user.NewStore(db)
	userService := user.NewService(userStore)
	userHandler := user.NewHandler(userService, cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)

	deps := &server.Dependencies{
		FlightHandler:         flightHandler,
		ProviderHandler:       providerHandler,
		UserHandler:           userHandler,
		JWTSecret:             cfg.Auth.JWTSecret,
		Version:               version,
		WebFS:                 web.Frontend(),
		ScriptHashes:          web.InlineScriptHashes(),
		IPRequestsPerMinute:   cfg.RateLimit.IPRequestsPerMinute,
		UserRequestsPerMinute: cfg.RateLimit.UserRequestsPerMinute,
	}
	r := server.SetupRouter(deps)

	return &http.Server{
		Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}, nil
}
