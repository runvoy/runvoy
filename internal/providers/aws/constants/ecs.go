// Package constants provides AWS-specific constants for ECS task execution.
package constants

import "runvoy/internal/constants"

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
const SharedVolumePath = "/workspace"

// EcsStatus represents the AWS ECS Task LastStatus lifecycle values.
// These are string statuses returned by ECS DescribeTasks for Task.LastStatus.
type EcsStatus string

const (
	// EcsStatusProvisioning represents a task being provisioned
	EcsStatusProvisioning EcsStatus = "PROVISIONING"
	// EcsStatusPending represents a task pending activation
	EcsStatusPending EcsStatus = "PENDING"
	// EcsStatusActivating represents a task being activated
	EcsStatusActivating EcsStatus = "ACTIVATING"
	// EcsStatusRunning represents a task currently running
	EcsStatusRunning EcsStatus = "RUNNING"
	// EcsStatusDeactivating represents a task being deactivated
	EcsStatusDeactivating EcsStatus = "DEACTIVATING"
	// EcsStatusStopping represents a task being stopped
	EcsStatusStopping EcsStatus = "STOPPING"
	// EcsStatusDeprovisioning represents a task being deprovisioned
	EcsStatusDeprovisioning EcsStatus = "DEPROVISIONING"
	// EcsStatusStopped represents a task that has stopped
	EcsStatusStopped EcsStatus = "STOPPED"
)

// LogStreamPrefix is the prefix for all log stream names for ECS tasks
const LogStreamPrefix = "task"

// LogStreamPartsCount is the expected number of parts in a log stream name
// Format: task/{container}/{execution_id} = 3 parts
const LogStreamPartsCount = 3

// ECSTaskDefinitionMaxResults is the maximum number of results for ECS ListTaskDefinitions
const ECSTaskDefinitionMaxResults = int32(100)

// ECSEphemeralStorageSizeGiB is the ECS ephemeral storage size in GiB
const ECSEphemeralStorageSizeGiB = 21

// DefaultCPU is the default CPU units for ECS task definitions
const DefaultCPU = 256

// DefaultMemory is the default memory (in MB) for ECS task definitions
const DefaultMemory = 512

// DefaultRuntimePlatform is the default runtime platform for ECS task definitions
const DefaultRuntimePlatform = DefaultRuntimePlatformOSFamily + "/" + DefaultRuntimePlatformArchitecture

// DefaultRuntimePlatformArchitecture is the default architecture for ECS task definitions
const DefaultRuntimePlatformArchitecture = "ARM64"

// DefaultRuntimePlatformOSFamily is the default OS family for ECS task definitions
const DefaultRuntimePlatformOSFamily = "Linux"

// TaskDefinitionFamilyPrefix is the prefix for all runvoy task definition families
// Task definitions are named: {ProjectName}-image-{sanitized-image-name}
// e.g., "runvoy-image-hashicorp-terraform-1-6" for image "hashicorp/terraform:1.6"
const TaskDefinitionFamilyPrefix = constants.ProjectName + "-image"

// TaskDefinitionIsDefaultTagKey is the ECS tag key used to mark a task definition as the default image
const TaskDefinitionIsDefaultTagKey = "IsDefault"

// TaskDefinitionDockerImageTagKey is the ECS tag key used to store the Docker image name for metadata
const TaskDefinitionDockerImageTagKey = "DockerImage"

// TaskDefinitionIsDefaultTagValue is the tag value used to mark a task definition as the default image
const TaskDefinitionIsDefaultTagValue = "true"
