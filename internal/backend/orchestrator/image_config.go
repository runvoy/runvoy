package orchestrator

// ImageConfig provides a provider-agnostic way to configure image registration.
// It abstracts away provider-specific details like IAM roles (AWS), service accounts (GCP), etc.
type ImageConfig struct {
	// Image is the Docker image reference (e.g., "alpine:latest", "gcr.io/project/image:tag")
	Image string

	// IsDefault marks this image as the default for executions when no image is specified
	IsDefault *bool

	// Resources specifies compute resource requirements
	Resources *ResourceConfig

	// Runtime specifies platform and architecture requirements
	Runtime *RuntimeConfig

	// Permissions specifies execution permissions and roles
	Permissions *PermissionConfig

	// RegisteredBy tracks which user registered this image
	RegisteredBy string
}

// ResourceConfig abstracts CPU and memory requirements across providers.
// Different providers have different granularities and limits:
// - AWS ECS: CPU in units (256, 512, 1024, etc.), Memory in MB
// - GCP Cloud Run: CPU in millicores, Memory in MB/GB
// - Azure Container Instances: CPU cores (0.5, 1, 2, 4), Memory in GB
type ResourceConfig struct {
	// CPU in provider-specific units:
	// - AWS ECS: 256, 512, 1024, 2048, 4096 (CPU units)
	// - GCP Cloud Run: 1000, 2000, 4000 (millicores)
	// - Azure ACI: 1, 2, 4 (CPU cores)
	CPU *int

	// Memory in MB (will be converted to provider-specific units)
	Memory *int
}

// RuntimeConfig specifies platform and architecture requirements.
// Abstracts provider-specific runtime platform specifications.
type RuntimeConfig struct {
	// Platform specifies the OS and architecture (e.g., "linux/amd64", "linux/arm64")
	// All providers support these standard platform strings
	Platform *string
}

// PermissionConfig abstracts execution permissions across providers.
// Different providers handle permissions differently:
// - AWS: TaskRole (app permissions) and TaskExecutionRole (infrastructure permissions)
// - GCP: Service Account
// - Azure: Managed Identity
type PermissionConfig struct {
	// TaskRole grants permissions to the running task/container
	// AWS: IAM role name, GCP: service account email, Azure: managed identity client ID
	TaskRole *string

	// ExecutionRole grants permissions to infrastructure to start the task
	// AWS-specific: IAM role for ECS to pull images, write logs, etc.
	// Not used by GCP or Azure
	ExecutionRole *string
}
