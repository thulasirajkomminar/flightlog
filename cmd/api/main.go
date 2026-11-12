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
	"syscall"
	"time"

	"gorm.io/gorm"

	"github.com/thulasirajkomminar/flightlog/internal/airport"
	"github.com/thulasirajkomminar/flightlog/internal/config"
	"github.com/thulasirajkomminar/flightlog/internal/database"
	"github.com/thulasirajkomminar/flightlog/internal/flight"
	"github.com/thulasirajkomminar/flightlog/internal/importer"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
	"github.com/thulasirajkomminar/flightlog/internal/provider/aerodatabox"
	"github.com/thulasirajkomminar/flightlog/internal/server"
	"github.com/thulasirajkomminar/flightlog/internal/user"
	"github.com/thulasirajkomminar/flightlog/internal/web"
)

const (
	shutdownTimeout    = 10 * time.Second
	serverReadTimeout  = 30 * time.Second
	serverWriteTimeout = 60 * time.Second
	serverIdleTimeout  = 120 * time.Second
)

var errUsage = errors.New("usage")

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

	if isResetPassword(os.Args) {
		err = runResetPassword(cfg, os.Args)
		if err != nil {
			appLogger.Error("Password reset failed", err)
		}

		return
	}

	appLogger.Info("Starting Flightlog API server")

	warnAboutConfig(cfg, appLogger)

	db, err := initDatabase(cfg, appLogger)
	if err != nil {
		appLogger.Error("Failed to initialise database", err)

		return
	}

	// Cancelled on shutdown to stop background work such as the status refresher.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := buildServer(ctx, cfg, db)
	if err != nil {
		appLogger.Error("Failed to build server", err)

		return
	}

	startServer(srv, appLogger)
	awaitShutdown(srv, db, appLogger)
}

// warnAboutConfig logs settings that work but are unsafe.
func warnAboutConfig(cfg *config.Config, appLogger *logger.ComponentLogger) {
	if cfg.Auth.WeakSecret() {
		appLogger.Warn("AUTH_JWT_SECRET is short enough to be guessed; anyone who guesses it can sign in as any user. " +
			"Generate a strong one with: openssl rand -base64 32")
	}
}

func initLogger(cfg *config.Config) (*logger.ComponentLogger, error) {
	err := logger.Init(logger.Config{
		Level:       cfg.LogLevel,
		Format:      cfg.LogFormat,
		Environment: cfg.Environment,
	})
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
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise database: %w", err)
	}

	appLogger.Info("Database migration completed")

	return db, nil
}

// pingDatabase returns a health probe for the database.
func pingDatabase(db *gorm.DB) func(context.Context) error {
	return func(ctx context.Context) error {
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("database handle: %w", err)
		}

		err = sqlDB.PingContext(ctx)
		if err != nil {
			return fmt.Errorf("ping database: %w", err)
		}

		return nil
	}
}

func buildServer(ctx context.Context, cfg *config.Config, db *gorm.DB) (*http.Server, error) {
	flightCacheStore := flight.NewCacheStore(db)
	userFlightStore := flight.NewUserFlightStore(db)

	aeroProvider, err := aerodatabox.NewProvider(cfg.AeroDataBox.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AeroDataBox provider: %w", err)
	}

	airportStore := airport.NewStore(db)
	cachedLookup := airport.NewCachedLookup(airportStore, aeroProvider)

	flightService := flight.NewService(flightCacheStore, userFlightStore, aeroProvider, cachedLookup).
		WithLiveCoverage(aeroProvider)
	flightHandler := flight.NewHandler(flightService)

	// The loop always runs for housekeeping; status checks are optional.
	flightService.WithStatusRefresh(cfg.Flights.StatusRefresh)

	go flightService.RunRefresher(ctx)

	importService := importer.NewService(flightService, flightService, flightCacheStore, cachedLookup).
		WithJobRepository(importer.NewJobStore(db))
	// Carry on with imports that a restart interrupted.
	importService.ResumeInterrupted(ctx)
	importHandler := importer.NewHandler(importService)

	userStore := user.NewStore(db)
	userService := user.NewService(userStore)
	userHandler := user.NewHandler(userService, cfg.Auth.JWTSecret, cfg.Auth.RegistrationEnabled)

	deps := &server.Dependencies{
		FlightHandler:  flightHandler,
		ImportHandler:  importHandler,
		UserHandler:    userHandler,
		Sessions:       userService,
		Ping:           pingDatabase(db),
		JWTSecret:      cfg.Auth.JWTSecret,
		Version:        version,
		WebFS:          web.Frontend(),
		ScriptHashes:   web.InlineScriptHashes(),
		TrustedProxies: cfg.Server.TrustedProxies,
	}
	r := server.SetupRouter(deps)

	return &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
	}, nil
}
