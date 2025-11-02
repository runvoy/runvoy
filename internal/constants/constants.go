// Package constants defines global constants used throughout runvoy.
// It includes version information, paths, and configuration keys.
package constants

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

const ApiKeyHeader = "X-API-Key"

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

// RunnerContainerName is the ECS container name used for task execution.
// Must match the container override name passed in the ECS RunTask call.
const RunnerContainerName = "runner"

// SidecarContainerName is the sidecar container name for auxiliary tasks.
// This container runs before the main runner container and handles tasks like
// .env file generation from user environment variables, git repository cloning, etc.
const SidecarContainerName = "sidecar"

// SharedVolumeName is the name of the shared volume between containers.
// Used for sharing the cloned git repository from sidecar to main container.
const SharedVolumeName = "workspace"

// SharedVolumePath is the mount path for the shared volume in both containers.
// When git repository is specified, the sidecar clones to /workspace/repo.
// The sidecar also creates .env file from user environment variables (prefixed with RUNVOY_USER_).
const SharedVolumePath = "/workspace"

// EcsStatus represents the AWS ECS Task LastStatus lifecycle values.
// These are string statuses returned by ECS DescribeTasks for Task.LastStatus.
type EcsStatus string

const (
	// ECS task lifecycle statuses
	EcsStatusProvisioning   EcsStatus = "PROVISIONING"
	EcsStatusPending        EcsStatus = "PENDING"
	EcsStatusActivating     EcsStatus = "ACTIVATING"
	EcsStatusRunning        EcsStatus = "RUNNING"
	EcsStatusDeactivating   EcsStatus = "DEACTIVATING"
	EcsStatusStopping       EcsStatus = "STOPPING"
	EcsStatusDeprovisioning EcsStatus = "DEPROVISIONING"
	EcsStatusStopped        EcsStatus = "STOPPED"
)

// ExecutionStatus represents the business-level status of a command execution.
// This is distinct from EcsStatus, which reflects the AWS ECS task lifecycle.
// Execution statuses are used throughout the API and stored in the database.
type ExecutionStatus string

const (
	// ExecutionRunning indicates the command is currently executing
	ExecutionRunning ExecutionStatus = "RUNNING"
	// ExecutionSucceeded indicates the command completed successfully
	ExecutionSucceeded ExecutionStatus = "SUCCEEDED"
	// ExecutionFailed indicates the command failed with an error
	ExecutionFailed ExecutionStatus = "FAILED"
	// ExecutionStopped indicates the command was manually terminated
	ExecutionStopped ExecutionStatus = "STOPPED"
)

// TerminalExecutionStatuses returns all statuses that represent completed executions
func TerminalExecutionStatuses() []ExecutionStatus {
	return []ExecutionStatus{
		ExecutionSucceeded,
		ExecutionFailed,
		ExecutionStopped,
	}
}

// WebviewerURL is the URL of the webviewer HTML file.
// TODO: Make this configurable in the future.
const WebviewerURL = "https://runvoy-releases.s3.us-east-2.amazonaws.com/webviewer.html"

// ClaimURLExpirationMinutes is the number of minutes after which a claim URL expires
const ClaimURLExpirationMinutes = 15

// ClaimEndpointPath is the HTTP path for claiming API keys
const ClaimEndpointPath = "/claim"

type StartTimeCtxKeyType string

// StartTimeCtxKey is the key used to store the start time in context
const StartTimeCtxKey StartTimeCtxKeyType = "startTime"
