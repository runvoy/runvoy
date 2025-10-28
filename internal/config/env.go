package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Env represents environment variable configuration for the service.
// All configuration values are loaded from environment variables at startup.
type Env struct {
	// Port is the HTTP server port. Defaults to "56212".
	Port string `env:"RUNVOY_DEV_SERVER_PORT" envDefault:"56212"`

	// APIKeysTable is the DynamoDB table name for API keys (AWS only).
	// This is required when running with AWS backend and cannot be empty.
	APIKeysTable string `env:"RUNVOY_API_KEYS_TABLE,notEmpty" envRequired:"true"`

	// LambdaInitTimeout is the timeout for the Lambda function initialization.
	LambdaInitTimeout time.Duration `env:"RUNVOY_LAMBDA_INIT_TIMEOUT" envDefault:"10s"`
}

// LoadEnv loads and validates environment variables into an Env struct.
// It returns an error if required variables are missing or invalid.
func LoadEnv() (*Env, error) {
	cfg := &Env{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Validate that APIKeysTable is not empty
	if cfg.APIKeysTable == "" {
		return nil, fmt.Errorf("APIKeysTable (RUNVOY_API_KEYS_TABLE) cannot be empty")
	}

	return cfg, nil
}

// MustLoadEnv loads environment variables and panics if there's an error.
// This is suitable for application startup where configuration errors should be fatal.
func MustLoadEnv() *Env {
	cfg, err := LoadEnv()
	if err != nil {
		panic(fmt.Sprintf("failed to load environment configuration: %v", err))
	}
	return cfg
}
