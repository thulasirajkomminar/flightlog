// Package config loads and validates application configuration from the environment.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config holds application configuration.
type Config struct {
	Environment string            `env:"ENVIRONMENT"        envDefault:"development"`
	Server      ServerConfig      `envPrefix:"SERVER_"`
	Database    DatabaseConfig    `envPrefix:"DATABASE_"`
	AeroDataBox AeroDataBoxConfig `envPrefix:"AERODATABOX_"`
	Auth        AuthConfig        `envPrefix:"AUTH_"`
}

// ServerConfig holds server settings.
type ServerConfig struct {
	Port string `env:"PORT" envDefault:"8080"`
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Path string `env:"PATH" envDefault:"data/flightlog.db"`
}

// AeroDataBoxConfig holds AeroDataBox API settings.
type AeroDataBoxConfig struct {
	APIKey string `env:"API_KEY,required"`
}

// AuthConfig holds auth settings.
type AuthConfig struct {
	JWTSecret string `env:"JWT_SECRET,required"`
}

// Load parses environment variables into Config.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}

	err := env.Parse(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	return cfg, nil
}
