// Package config loads and validates application configuration from the environment.
package config

import (
	"fmt"
	"time"

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
	RateLimit   RateLimitConfig   `envPrefix:"RATE_LIMIT_"`
}

// ServerConfig holds server settings.
type ServerConfig struct {
	Port         string        `env:"PORT"          envDefault:"8080"`
	Host         string        `env:"HOST"          envDefault:"0.0.0.0"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT"  envDefault:"30s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"30s"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT"  envDefault:"120s"`
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Path string `env:"PATH" envDefault:"data/flightlog.db"`
}

// AeroDataBoxConfig holds AeroDataBox API settings.
type AeroDataBoxConfig struct {
	APIKey  string        `env:"API_KEY,required"`
	BaseURL string        `env:"BASE_URL"         envDefault:"https://aerodatabox.p.rapidapi.com"`
	Timeout time.Duration `env:"TIMEOUT"          envDefault:"30s"`
}

// AuthConfig holds auth settings.
type AuthConfig struct {
	JWTSecret   string        `env:"JWT_SECRET,required"`
	TokenExpiry time.Duration `env:"TOKEN_EXPIRY"        envDefault:"24h"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	IPRequestsPerMinute   int `env:"IP_REQUESTS_PER_MINUTE"   envDefault:"100"`
	UserRequestsPerMinute int `env:"USER_REQUESTS_PER_MINUTE" envDefault:"200"`
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
