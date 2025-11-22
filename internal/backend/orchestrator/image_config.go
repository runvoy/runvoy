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
// - Kubernetes: CPU in millicores, Memory in Mi/Gi
type ResourceConfig struct {
	// CPU in provider-specific units. For AWS ECS: 256, 512, 1024, 2048, 4096
	CPU *int

	// Memory in MB
	Memory *int
}

// RuntimeConfig specifies platform and architecture requirements.
// Abstracts provider-specific runtime platform specifications.
type RuntimeConfig struct {
	// Platform specifies the OS and architecture (e.g., "linux/amd64", "linux/arm64")
	Platform *string

	// Architecture is deprecated in favor of Platform, but kept for backwards compatibility
	Architecture *string
}

// PermissionConfig abstracts execution permissions across providers.
// Different providers handle permissions differently:
// - AWS: TaskRole (app permissions) and TaskExecutionRole (infrastructure permissions)
// - GCP: Service Account
// - Kubernetes: ServiceAccount
type PermissionConfig struct {
	// TaskRole grants permissions to the running task/container
	// AWS: IAM role name, GCP: service account email
	TaskRole *string

	// ExecutionRole grants permissions to infrastructure to start the task
	// AWS-specific: IAM role for ECS to pull images, write logs, etc.
	ExecutionRole *string
}

// ToLegacyParams converts ImageConfig to the legacy RegisterImage parameters
// for backwards compatibility with existing code.
func (c *ImageConfig) ToLegacyParams() (
	image string,
	isDefault *bool,
	taskRoleName, taskExecutionRoleName *string,
	cpu, memory *int,
	runtimePlatform *string,
	createdBy string,
) {
	image = c.Image
	isDefault = c.IsDefault
	createdBy = c.RegisteredBy

	if c.Resources != nil {
		cpu = c.Resources.CPU
		memory = c.Resources.Memory
	}

	if c.Runtime != nil {
		runtimePlatform = c.Runtime.Platform
	}

	if c.Permissions != nil {
		taskRoleName = c.Permissions.TaskRole
		taskExecutionRoleName = c.Permissions.ExecutionRole
	}

	return
}

// FromLegacyParams creates an ImageConfig from legacy RegisterImage parameters
// for backwards compatibility.
func FromLegacyParams(
	image string,
	isDefault *bool,
	taskRoleName, taskExecutionRoleName *string,
	cpu, memory *int,
	runtimePlatform *string,
	createdBy string,
) *ImageConfig {
	config := &ImageConfig{
		Image:        image,
		IsDefault:    isDefault,
		RegisteredBy: createdBy,
	}

	if cpu != nil || memory != nil {
		config.Resources = &ResourceConfig{
			CPU:    cpu,
			Memory: memory,
		}
	}

	if runtimePlatform != nil {
		config.Runtime = &RuntimeConfig{
			Platform: runtimePlatform,
		}
	}

	if taskRoleName != nil || taskExecutionRoleName != nil {
		config.Permissions = &PermissionConfig{
			TaskRole:      taskRoleName,
			ExecutionRole: taskExecutionRoleName,
		}
	}

	return config
}
