// Package constants defines global constants used throughout runvoy.
// It includes version information, paths, and configuration keys.
package constants

import (
	"slices"
	"time"
)

var version = "0.0.0-development" // Updated by CI/CD pipeline at build time

// GetVersion returns the current version of runvoy.
func GetVersion() *string {
	return &version
}

// ProjectName is the name of the CLI tool and application
const ProjectName = "runvoy"

// ConfigDirName is the name of the configuration directory in the user's home directory
const ConfigDirName = ".runvoy"

// ConfigFileName is the name of the global configuration file
const ConfigFileName = "config.yaml"

// ConfigDirPath returns the full path to the global configuration directory.
func ConfigDirPath(homeDir string) string {
	return homeDir + "/" + ConfigDirName
}

// ConfigFilePath returns the full path to the global configuration file
func ConfigFilePath(homeDir string) string {
	return ConfigDirPath(homeDir) + "/" + ConfigFileName
}

// BackendProvider represents the backend infrastructure provider.
type BackendProvider string

const (
	// AWS is the Amazon Web Services backend provider.
	AWS BackendProvider = "AWS"
	// Example: GCP BackendProvider = "GCP"
)

// Environment represents the execution environment (e.g., CLI, Lambda).
type Environment string

// Environment types for logger configuration
const (
	Development Environment = "development"
	Production  Environment = "production"
	CLI         Environment = "cli"
)

// APIKeyHeader is the HTTP header name for API key authentication
//
//nolint:gosec // G101: This is a header name constant, not a hardcoded credential
const APIKeyHeader = "X-API-Key"

// ContentTypeHeader is the HTTP Content-Type header name.
const ContentTypeHeader = "Content-Type"

// ConfigCtxKeyType is the type for the config context key
type ConfigCtxKeyType string

// ConfigCtxKey is the key used to store config in context
const ConfigCtxKey ConfigCtxKeyType = "config"

// Service represents a runvoy service component.
type Service string

const (
	// OrchestratorService is the main orchestrator service.
	OrchestratorService Service = "orchestrator"
	// EventProcessorService is the event processing service.
	EventProcessorService Service = "event-processor"
)

// EcsStatus, container name, and volume constants are provider-specific.
// See internal/providers/aws/constants/ecs.go for AWS ECS-specific constants.

// ExecutionStatus represents the business-level status of a command execution.
// This is distinct from EcsStatus, which reflects the AWS ECS task lifecycle.
// Execution statuses are used throughout the API and stored in the database.
type ExecutionStatus string

const (
	// ExecutionStarting indicates the command has been accepted and is being scheduled
	ExecutionStarting ExecutionStatus = "STARTING"
	// ExecutionRunning indicates the command is currently executing
	ExecutionRunning ExecutionStatus = "RUNNING"
	// ExecutionSucceeded indicates the command completed successfully
	ExecutionSucceeded ExecutionStatus = "SUCCEEDED"
	// ExecutionFailed indicates the command failed with an error
	ExecutionFailed ExecutionStatus = "FAILED"
	// ExecutionStopped indicates the command was manually terminated
	ExecutionStopped ExecutionStatus = "STOPPED"
	// ExecutionTerminating indicates a stop request is in progress
	ExecutionTerminating ExecutionStatus = "TERMINATING"
)

// TerminalExecutionStatuses returns all statuses that represent completed executions
func TerminalExecutionStatuses() []ExecutionStatus {
	return []ExecutionStatus{
		ExecutionFailed,
		ExecutionStopped,
		ExecutionSucceeded,
		ExecutionTerminating,
	}
}

// validTransitions defines the allowed state transitions for execution statuses.
// Each key represents a source status, and the value is a slice of allowed destination statuses.
var validTransitions = map[ExecutionStatus][]ExecutionStatus{
	ExecutionStarting:    {ExecutionRunning, ExecutionFailed, ExecutionTerminating},
	ExecutionRunning:     {ExecutionSucceeded, ExecutionFailed, ExecutionStopped, ExecutionTerminating},
	ExecutionTerminating: {ExecutionStopped},
	// Terminal states (SUCCEEDED, FAILED, STOPPED) have no valid transitions
	ExecutionSucceeded: {},
	ExecutionFailed:    {},
	ExecutionStopped:   {},
}

// CanTransition checks if a status transition from 'from' to 'to' is valid.
// Returns true if the transition is allowed, false otherwise.
// If the source status is not in the validTransitions map, returns false.
func CanTransition(from, to ExecutionStatus) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(allowed, to)
}

// DefaultWebURL is the default URL of the web application HTML file.
// This can be overridden via configuration (RUNVOY_WEB_URL env var or config file).
const DefaultWebURL = "https://runvoy.site/"

// WebviewerURL is deprecated. Use config.Config.WebURL or constants.DefaultWebURL instead.
// Kept for backward compatibility.
const WebviewerURL = DefaultWebURL

// ClaimURLExpirationMinutes is the number of minutes after which a claim URL expires
const ClaimURLExpirationMinutes = 15

// ClaimEndpointPath is the HTTP path for claiming API keys
const ClaimEndpointPath = "/claim"

// TaskDefinitionFamilyPrefix is the prefix for all runvoy task definition families
// Task definitions are named: {ProjectName}-image-{sanitized-image-name}
// e.g., "runvoy-image-hashicorp-terraform-1-6" for image "hashicorp/terraform:1.6"
// This is AWS ECS-specific and should ideally be in internal/providers/aws/constants/,
// but is kept here as it's used by the core app interface for task definition naming.
const TaskDefinitionFamilyPrefix = "runvoy-image"

// TaskDefinitionIsDefaultTagKey is the ECS tag key used to mark a task definition as the default image
// AWS-specific tagging convention.
const TaskDefinitionIsDefaultTagKey = "IsDefault"

// TaskDefinitionDockerImageTagKey is the ECS tag key used to store the Docker image name for metadata
// AWS-specific tagging convention.
const TaskDefinitionDockerImageTagKey = "DockerImage"

// TaskDefinitionIsDefaultTagValue is the tag value used to mark a task definition as the default image
// AWS-specific tagging convention.
const TaskDefinitionIsDefaultTagValue = "true"

// StartTimeCtxKeyType is the type for start time context keys
type StartTimeCtxKeyType string

// StartTimeCtxKey is the key used to store the start time in context
const StartTimeCtxKey StartTimeCtxKeyType = "startTime"

// Time-related constants

// ServerReadTimeout is the HTTP server read timeout
const ServerReadTimeout = 15 * time.Second

// ServerWriteTimeout is the HTTP server write timeout
const ServerWriteTimeout = 15 * time.Second

// ServerIdleTimeout is the HTTP server idle timeout
const ServerIdleTimeout = 60 * time.Second

// ServerShutdownTimeout is the timeout for graceful server shutdown
const ServerShutdownTimeout = 5 * time.Second

// DefaultContextTimeout is the default timeout for context operations
const DefaultContextTimeout = 10 * time.Second

// ScriptContextTimeout is the timeout for script context operations
const ScriptContextTimeout = 10 * time.Second

// LongScriptContextTimeout is the timeout for longer script context operations
const LongScriptContextTimeout = 30 * time.Second

// TestContextTimeout is the timeout for test contexts
const TestContextTimeout = 5 * time.Second

// SpinnerTickerInterval is the interval between spinner frame updates
const SpinnerTickerInterval = 80 * time.Millisecond

// HTTP status code constants

// HTTPStatusBadRequest is the HTTP status code for bad requests (400)
const HTTPStatusBadRequest = 400

// HTTPStatusServerError is the HTTP status code for server errors (500)
const HTTPStatusServerError = 500

// File permission constants

// ConfigDirPermissions is the file system permissions for config directory (0750)
const ConfigDirPermissions = 0750

// ConfigFilePermissions is the file system permissions for config file (0600)
const ConfigFilePermissions = 0600

// Byte size constants

// APIKeyByteSize is the number of random bytes used to generate API keys
const APIKeyByteSize = 24

// SecretTokenByteSize is the number of random bytes used to generate secret tokens
const SecretTokenByteSize = 24

// RequestIDByteSize is the number of random bytes used to generate request IDs
const RequestIDByteSize = 16

// UUIDByteSize is the number of random bytes used to generate UUIDs
// 16 bytes = 128 bits, same as a UUID
const UUIDByteSize = 16

// AWS/CloudWatch constants
// AWS-specific constants have been moved to internal/providers/aws/constants/

// UI/Display constants

// HeaderSeparatorLength is the length of the header separator line
const HeaderSeparatorLength = 50

// ProgressBarWidth is the default width for progress bars
const ProgressBarWidth = 40

// BoxBorderPadding is the padding used in box borders
const BoxBorderPadding = 2

// Conversion constants

// MillisecondsPerSecond is the number of milliseconds in a second
const MillisecondsPerSecond = 1000

// PercentageMultiplier is the multiplier to convert fraction to percentage
const PercentageMultiplier = 100

// SecondsPerMinute is the number of seconds in a minute
const SecondsPerMinute = 60

// MinutesPerHour is the number of minutes in an hour
const MinutesPerHour = 60

// Slice/Array capacity constants

// ExecutionsSliceInitialCapacity is the initial capacity for executions slices
const ExecutionsSliceInitialCapacity = 64

// String split constants

// EnvVarSplitLimit is the limit for splitting environment variable strings (KEY=VALUE)
const EnvVarSplitLimit = 2

// Regex match count constants

// RegexMatchCountEnvVar is the expected number of regex matches for environment variable parsing
const RegexMatchCountEnvVar = 3

// Argument validation constants

// ExpectedArgsCreateConfigFile is the expected number of arguments for create-config-file script
const ExpectedArgsCreateConfigFile = 2

// ExpectedArgsSeedAdminUser is the expected number of arguments for seed-admin-user script
const ExpectedArgsSeedAdminUser = 3

// ExpectedArgsTruncateDynamoDBTable is the expected number of arguments for truncate-dynamodb-table script
const ExpectedArgsTruncateDynamoDBTable = 2

// MinimumArgsUpdateReadmeHelp is the minimum number of arguments for update-readme-help script
const MinimumArgsUpdateReadmeHelp = 2

//
// WebSocket constants
//

// ConnectionTTLHours is the time-to-live for connection records in the database (24 hours)
const ConnectionTTLHours = 24

// FunctionalityLogStreaming identifies connections used for streaming execution logs
const FunctionalityLogStreaming = "log_streaming"

// MaxConcurrentSends is the maximum number of concurrent sends to WebSocket connections
const MaxConcurrentSends = 10

// DefaultGitRef is the default Git reference to use if no reference is provided
const DefaultGitRef = "main"

// PlaybookDirName is the name of the playbook directory in the current working directory
const PlaybookDirName = ".runvoy"

// PlaybookFileExtensions are the valid file extensions for playbook files
var PlaybookFileExtensions = []string{".yaml", ".yml"}
