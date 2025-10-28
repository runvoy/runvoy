package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Env represents environment variable configuration for the service.
// All configuration values are loaded from environment variables at startup.
type Env struct {
	// Port is the HTTP server port. Defaults to "8080".
	Port string `env:"PORT" envDefault:"8080"`

	// APIKeysTable is the DynamoDB table name for API keys (AWS only).
	// This is required when running with AWS backend.
	APIKeysTable string `env:"API_KEYS_TABLE"`
}

// LoadEnv loads and validates environment variables into an Env struct.
// It returns an error if required variables are missing or invalid.
//
// Example:
//
//	cfg, err := config.LoadEnv()
//	if err != nil {
//	    log.Fatalf("Failed to load environment: %v", err)
//	}
func LoadEnv() (*Env, error) {
	cfg := &Env{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}
	return cfg, nil
}

// MustLoadEnv loads environment variables and panics if there's an error.
// This is suitable for application startup where configuration errors should be fatal.
//
// Example:
//
//	cfg := config.MustLoadEnv()
//	log.Printf("Starting server on port %s", cfg.Port)
func MustLoadEnv() *Env {
	cfg, err := LoadEnv()
	if err != nil {
		panic(fmt.Sprintf("failed to load environment configuration: %v", err))
	}
	return cfg
}
