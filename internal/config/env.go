package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
)

// Env represents environment variable configuration for the service.
// All configuration values are loaded from environment variables at startup.
type Env struct {
	// Port is the HTTP server port. Defaults to "56212".
	Port string `env:"RUNVOY_DEV_SERVER_PORT" envDefault:"56212"`

	// RequestTimeout is the timeout for the request.
	// Defaults to 0, which means no timeout middleware is added, allowing the
	// environment (e.g., Lambda with its own timeout) to handle timeouts.
	RequestTimeout time.Duration `env:"RUNVOY_REQUEST_TIMEOUT" envDefault:"0"`

	// APIKeysTable is the DynamoDB table name for API keys (AWS only).
	// NOTICE: this is required when running with AWS backend and cannot be empty.
	APIKeysTable string `env:"RUNVOY_API_KEYS_TABLE,notEmpty" envRequired:"true"`

	// ExecutionsTable is the DynamoDB table name for execution records (AWS only).
	ExecutionsTable string `env:"RUNVOY_EXECUTIONS_TABLE,notEmpty" envRequired:"true"`

	// ECSCluster is the ECS cluster name where tasks will run (AWS only).
	ECSCluster string `env:"RUNVOY_ECS_CLUSTER,notEmpty" envRequired:"true"`

	// TaskDefinition is the ECS task definition ARN or family name (AWS only).
	TaskDefinition string `env:"RUNVOY_TASK_DEFINITION,notEmpty" envRequired:"true"`

	// Subnet1 is the first subnet ID for ECS tasks (AWS only).
	Subnet1 string `env:"RUNVOY_SUBNET_1,notEmpty" envRequired:"true"`

	// Subnet2 is the second subnet ID for ECS tasks (AWS only).
	Subnet2 string `env:"RUNVOY_SUBNET_2,notEmpty" envRequired:"true"`

	// SecurityGroup is the security group ID for ECS tasks (AWS only).
	SecurityGroup string `env:"RUNVOY_SECURITY_GROUP,notEmpty" envRequired:"true"`

	// LogGroup is the CloudWatch log group name for execution logs (AWS only).
	LogGroup string `env:"RUNVOY_LOG_GROUP,notEmpty" envRequired:"true"`

	// DefaultImage is the default Docker image to use if not specified in request.
	DefaultImage string `env:"RUNVOY_DEFAULT_IMAGE" envDefault:"public.ecr.aws/docker/library/ubuntu:22.04"`

	// InitTimeout is the timeout for the environment initialization. Defaults to 10 seconds.
	InitTimeout time.Duration `env:"RUNVOY_INIT_TIMEOUT" envDefault:"10s"`

	// LogLevel is the log level for the logger. Defaults to "INFO".
	LogLevel slog.Level `env:"RUNVOY_LOG_LEVEL" envDefault:"INFO"`
}

// LoadEnv loads and validates environment variables into an Env struct.
// It returns an error if required variables are missing or invalid.
func LoadEnv() (*Env, error) {
	cfg := &Env{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	return cfg, nil
}

// MustLoadEnv loads environment variables and exits if there's an error.
// NOTICE: this is suitable for application startup where configuration errors should be fatal.
func MustLoadEnv() *Env {
	cfg, err := LoadEnv()
	if err != nil {
		slog.Error("failed to load environment configuration", "error", err)

		os.Exit(1)
	}

	return cfg
}
