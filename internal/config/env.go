package config

import (
	"fmt"
	"log/slog"
	"os"
	"runvoy/internal/constants"
	"time"

	"github.com/caarlos0/env/v11"
)

// OrchestratorEnv represents environment variable configuration for the orchestrator service.
// All configuration values are loaded from environment variables at startup.
type OrchestratorEnv struct {
	// Port is the HTTP server port. Defaults to "56212".
	Port string `env:"RUNVOY_DEV_SERVER_PORT" envDefault:"56212"`

	// RequestTimeout is the timeout for the request.
	// Defaults to 0, which means no timeout middleware is added, allowing the
	// environment (e.g., Lambda with its own timeout) to handle timeouts.
	RequestTimeout time.Duration `env:"RUNVOY_REQUEST_TIMEOUT" envDefault:"0"`

	// APIKeysTable is the DynamoDB table name for API keys (AWS only).
	// Required by orchestrator lambda, validated at runtime in app.Initialize.
	APIKeysTable string `env:"RUNVOY_API_KEYS_TABLE,notEmpty"`

	// ExecutionsTable is the DynamoDB table name for execution records (AWS only).
	// Required by both orchestrator and event processor lambdas.
	ExecutionsTable string `env:"RUNVOY_EXECUTIONS_TABLE,notEmpty" envRequired:"true"`

	// ECSCluster is the ECS cluster name where tasks will run (AWS only).
	// Required by orchestrator lambda, validated at runtime in app.Initialize.
	ECSCluster string `env:"RUNVOY_ECS_CLUSTER,notEmpty"`

	// TaskDefinition is the ECS task definition ARN or family name (AWS only).
	// Required by orchestrator lambda, validated at runtime in app.Initialize.
	TaskDefinition string `env:"RUNVOY_TASK_DEFINITION,notEmpty"`

	// Subnet1 is the first subnet ID for ECS tasks (AWS only).
	// Required by orchestrator lambda, validated at runtime in app.Initialize.
	Subnet1 string `env:"RUNVOY_SUBNET_1,notEmpty"`

	// Subnet2 is the second subnet ID for ECS tasks (AWS only).
	// Required by orchestrator lambda, validated at runtime in app.Initialize.
	Subnet2 string `env:"RUNVOY_SUBNET_2,notEmpty"`

	// SecurityGroup is the security group ID for ECS tasks (AWS only).
	// Required by orchestrator lambda, validated at runtime in app.Initialize.
	SecurityGroup string `env:"RUNVOY_SECURITY_GROUP,notEmpty"`

	// LogGroup is the CloudWatch log group name for execution logs (AWS only).
	// Required by orchestrator lambda, validated at runtime in app.Initialize.
	LogGroup string `env:"RUNVOY_LOG_GROUP,notEmpty"`

    // LogStreamPrefix is the CloudWatch log stream prefix for task logs (AWS only), e.g. "ecs/executor".
    // Optional; when set, the server reads logs from "<prefix>/<executionID>" without discovery.
    LogStreamPrefix string `env:"RUNVOY_LOG_STREAM_PREFIX"`

	// DefaultImage is the default Docker image to use if not specified in request.
	DefaultImage string `env:"RUNVOY_DEFAULT_IMAGE" envDefault:"public.ecr.aws/docker/library/ubuntu:22.04"`

	// InitTimeout is the timeout for the environment initialization. Defaults to 10 seconds.
	InitTimeout time.Duration `env:"RUNVOY_INIT_TIMEOUT" envDefault:"10s"`

	// LogLevel is the log level for the logger. Defaults to "INFO".
	LogLevel slog.Level `env:"RUNVOY_LOG_LEVEL" envDefault:"INFO"`
}

func loadOrchestratorEnv() (*OrchestratorEnv, error) {
	cfg := &OrchestratorEnv{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables for %s: %w", constants.OrchestratorService, err)
	}

	return cfg, nil
}

// EventProcessorEnv contains environment configuration for the event processor.
type EventProcessorEnv struct {
	// ExecutionsTable is the DynamoDB table name for execution records (AWS only).
	// Required by event processor lambda.
	ExecutionsTable string `env:"RUNVOY_EXECUTIONS_TABLE,notEmpty"`

	// ECSCluster is the ECS cluster name where tasks run (AWS only).
	// Required by event processor lambda.
	ECSCluster string `env:"RUNVOY_ECS_CLUSTER,notEmpty"`

	// InitTimeout is the timeout for the environment initialization. Defaults to 10 seconds.
	InitTimeout time.Duration `env:"RUNVOY_INIT_TIMEOUT" envDefault:"10s"`

	// LogLevel is the log level for the logger. Defaults to "INFO".
	LogLevel slog.Level `env:"RUNVOY_LOG_LEVEL" envDefault:"INFO"`
}

func loadEventProcessorEnv() (*EventProcessorEnv, error) {
	cfg := &EventProcessorEnv{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables for %s: %w", constants.EventProcessorService, err)
	}

	return cfg, nil
}

// MustLoadOrchestratorEnv loads orchestrator environment variables and exits if there's an error.
// NOTICE: this is suitable for application startup where configuration errors should be fatal.
func MustLoadOrchestratorEnv() *OrchestratorEnv {
	cfg, err := loadOrchestratorEnv()
	if err != nil {
		slog.Error("failed to load orchestrator environment configuration", "error", err)
		os.Exit(1)
	}
	return cfg
}

// MustLoadEventProcessorEnv loads event processor environment variables and exits if there's an error.
// NOTICE: this is suitable for application startup where configuration errors should be fatal.
func MustLoadEventProcessorEnv() *EventProcessorEnv {
	cfg, err := loadEventProcessorEnv()
	if err != nil {
		slog.Error("failed to load event processor environment configuration", "error", err)
		os.Exit(1)
	}
	return cfg
}
