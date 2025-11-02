// Package aws provides AWS-specific implementations for runvoy.
package aws

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"runvoy/internal/constants"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// SanitizeImageNameForTaskDef converts a Docker image name to a valid ECS task definition family name.
// ECS task definition family names must match: [a-zA-Z0-9_-]+
// Examples:
//   - "hashicorp/terraform:1.6" -> "hashicorp-terraform-1-6"
//   - "myregistry.com/my-image:latest" -> "myregistry-com-my-image-latest"
func SanitizeImageNameForTaskDef(image string) string {
	// Replace invalid characters with hyphens
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(image, "-")
	// Remove consecutive hyphens
	re2 := regexp.MustCompile(`-+`)
	sanitized = re2.ReplaceAllString(sanitized, "-")
	// Remove leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

// TaskDefinitionFamilyName returns the ECS task definition family name for a given image.
// Format: {TaskDefinitionFamilyPrefix}-{sanitized-image-name}
func TaskDefinitionFamilyName(image string) string {
	sanitized := SanitizeImageNameForTaskDef(image)
	return fmt.Sprintf("%s-%s", constants.TaskDefinitionFamilyPrefix, sanitized)
}

// ExtractImageFromTaskDefFamily extracts the Docker image name from a task definition family name.
// Returns empty string if the family name doesn't match the expected format.
func ExtractImageFromTaskDefFamily(familyName string) string {
	prefix := constants.TaskDefinitionFamilyPrefix + "-"
	if !strings.HasPrefix(familyName, prefix) {
		return ""
	}
	// This is approximate - we can't perfectly reconstruct the original image name
	// but we can extract a reasonable approximation for listing purposes
	imagePart := strings.TrimPrefix(familyName, prefix)
	// Replace hyphens back to slashes/colons where it makes sense (heuristic)
	// For now, just return the sanitized version
	return imagePart
}

// GetTaskDefinitionForImage looks up an existing task definition for the given Docker image.
// Returns an error if the task definition doesn't exist (does not auto-register).
func GetTaskDefinitionForImage(
	ctx context.Context,
	ecsClient *ecs.Client,
	image string,
	logger *slog.Logger,
) (string, error) {
	family := TaskDefinitionFamilyName(image)

	// Check if task definition already exists for this family
	listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: awsStd.String(family),
		Status:       ecsTypes.TaskDefinitionStatusActive,
		MaxResults:   awsStd.Int32(1),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list task definitions: %w", err)
	}

	// If a task definition exists, return the latest one
	if len(listOutput.TaskDefinitionArns) > 0 {
		latestARN := listOutput.TaskDefinitionArns[len(listOutput.TaskDefinitionArns)-1]
		logger.Debug("task definition found", "family", family, "arn", latestARN)
		return latestARN, nil
	}

	// Task definition doesn't exist - return error
	return "", fmt.Errorf("task definition for image %q not found (family: %s). Image must be registered via /api/v1/images/register", image, family)
}

// RegisterTaskDefinitionForImage registers a new ECS task definition for the given Docker image.
// The task definition uses the same structure as before (sidecar + runner), but with the specified runner image.
func RegisterTaskDefinitionForImage(
	ctx context.Context,
	ecsClient *ecs.Client,
	cfg *Config,
	image string,
	logger *slog.Logger,
) (string, error) {
	family := TaskDefinitionFamilyName(image)

	// Check if task definition already exists for this family
	listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: awsStd.String(family),
		Status:       ecsTypes.TaskDefinitionStatusActive,
		MaxResults:   awsStd.Int32(1),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list task definitions: %w", err)
	}

	// If a task definition exists, return the latest one
	if len(listOutput.TaskDefinitionArns) > 0 {
		latestARN := listOutput.TaskDefinitionArns[len(listOutput.TaskDefinitionArns)-1]
		logger.Debug("task definition already exists", "family", family, "arn", latestARN)
		return latestARN, nil
	}

	// Get task execution role and task role from existing task definitions or use from config
	taskExecRoleARN := cfg.TaskExecRoleARN
	taskRoleARN := cfg.TaskRoleARN

	// If roles not in config, try to get from an existing task definition (any family)
	if taskExecRoleARN == "" || taskRoleARN == "" {
		allFamilies, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			MaxResults: awsStd.Int32(1),
		})
		if err == nil && len(allFamilies.TaskDefinitionArns) > 0 {
			// Get the first task definition to extract roles
			descOutput, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: awsStd.String(allFamilies.TaskDefinitionArns[len(allFamilies.TaskDefinitionArns)-1]),
			})
			if err == nil && descOutput.TaskDefinition != nil {
				if taskExecRoleARN == "" && descOutput.TaskDefinition.ExecutionRoleArn != nil {
					taskExecRoleARN = awsStd.ToString(descOutput.TaskDefinition.ExecutionRoleArn)
				}
				if taskRoleARN == "" && descOutput.TaskDefinition.TaskRoleArn != nil {
					taskRoleARN = awsStd.ToString(descOutput.TaskDefinition.TaskRoleArn)
				}
			}
		}
	}

	// Register new task definition
	registerInput := &ecs.RegisterTaskDefinitionInput{
		Family:      awsStd.String(family),
		NetworkMode: ecsTypes.NetworkModeAwsvpc,
		RequiresCompatibilities: []ecsTypes.Compatibility{
			ecsTypes.CompatibilityFargate,
		},
		Cpu:             awsStd.String("256"),
		Memory:          awsStd.String("512"),
		ExecutionRoleArn: awsStd.String(taskExecRoleARN),
		TaskRoleArn:     awsStd.String(taskRoleARN),
		EphemeralStorage: &ecsTypes.EphemeralStorage{
			SizeInGiB: 21,
		},
		Volumes: []ecsTypes.Volume{
			{
				Name: awsStd.String(constants.SharedVolumeName),
			},
		},
		ContainerDefinitions: []ecsTypes.ContainerDefinition{
			// Sidecar container
			{
				Name:      awsStd.String(constants.SidecarContainerName),
				Image:     awsStd.String("public.ecr.aws/docker/library/alpine:latest"),
				Essential: awsStd.Bool(false),
				Command: []string{
					"/bin/sh",
					"-c",
					"echo \"This task definition is a template. Command will be overridden at runtime.\"",
				},
				MountPoints: []ecsTypes.MountPoint{
					{
						ContainerPath: awsStd.String(constants.SharedVolumePath),
						SourceVolume:  awsStd.String(constants.SharedVolumeName),
					},
				},
				LogConfiguration: &ecsTypes.LogConfiguration{
					LogDriver: ecsTypes.LogDriverAwslogs,
					Options: map[string]string{
						"awslogs-group":  cfg.LogGroup,
						"awslogs-region": "us-east-1", // TODO: get from AWS config
						"awslogs-stream-prefix": "task",
					},
				},
			},
			// Main runner container
			{
				Name:      awsStd.String(constants.RunnerContainerName),
				Image:     awsStd.String(image),
				Essential: awsStd.Bool(true),
				DependsOn: []ecsTypes.ContainerDependency{
					{
						ContainerName: awsStd.String(constants.SidecarContainerName),
						Condition:     ecsTypes.ContainerConditionSuccess,
					},
				},
				Command: []string{
					"/bin/sh",
					"-c",
					"echo \"This task definition is a template. Command will be overridden at runtime.\"",
				},
				WorkingDirectory: awsStd.String("/workspace/repo"),
				MountPoints: []ecsTypes.MountPoint{
					{
						ContainerPath: awsStd.String(constants.SharedVolumePath),
						SourceVolume:  awsStd.String(constants.SharedVolumeName),
					},
				},
				LogConfiguration: &ecsTypes.LogConfiguration{
					LogDriver: ecsTypes.LogDriverAwslogs,
					Options: map[string]string{
						"awslogs-group":  cfg.LogGroup,
						"awslogs-region": "us-east-1", // TODO: get from AWS config
						"awslogs-stream-prefix": "task",
					},
				},
			},
		},
	}

	registerOutput, err := ecsClient.RegisterTaskDefinition(ctx, registerInput)
	if err != nil {
		return "", fmt.Errorf("failed to register task definition: %w", err)
	}

	taskDefARN := awsStd.ToString(registerOutput.TaskDefinition.TaskDefinitionArn)
	logger.Info("registered task definition", "family", family, "arn", taskDefARN, "image", image)
	return taskDefARN, nil
}

// ListRegisteredImages lists all registered Docker images by querying ECS task definitions.
func ListRegisteredImages(
	ctx context.Context,
	ecsClient *ecs.Client,
	logger *slog.Logger,
) ([]string, error) {
	// List all task definitions with our prefix
	listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: awsStd.String(constants.TaskDefinitionFamilyPrefix),
		Status:       ecsTypes.TaskDefinitionStatusActive,
		MaxResults:   awsStd.Int32(100), // Get up to 100 task definitions
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list task definitions: %w", err)
	}

	images := make(map[string]bool)
	for _, taskDefARN := range listOutput.TaskDefinitionArns {
		// Extract family name from ARN: arn:aws:ecs:region:account:task-definition/family:revision
		parts := strings.Split(taskDefARN, "/")
		if len(parts) < 2 {
			continue
		}
		familyWithRev := parts[len(parts)-1]
		family := strings.Split(familyWithRev, ":")[0]

		// Extract image name from family
		image := ExtractImageFromTaskDefFamily(family)
		if image != "" {
			images[image] = true
		}
	}

	result := make([]string, 0, len(images))
	for img := range images {
		result = append(result, img)
	}

	return result, nil
}

// DeregisterTaskDefinitionsForImage deregisters all task definition revisions for a given image.
func DeregisterTaskDefinitionsForImage(
	ctx context.Context,
	ecsClient *ecs.Client,
	image string,
	logger *slog.Logger,
) error {
	family := TaskDefinitionFamilyName(image)

	// List all revisions of this task definition
	listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: awsStd.String(family),
		Status:       ecsTypes.TaskDefinitionStatusActive,
		MaxResults:   awsStd.Int32(100),
	})
	if err != nil {
		return fmt.Errorf("failed to list task definitions: %w", err)
	}

	// Deregister all revisions
	for _, taskDefARN := range listOutput.TaskDefinitionArns {
		_, err := ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: awsStd.String(taskDefARN),
		})
		if err != nil {
			logger.Warn("failed to deregister task definition revision", "arn", taskDefARN, "error", err)
			// Continue with other revisions
		} else {
			logger.Info("deregistered task definition revision", "arn", taskDefARN)
		}
	}

	logger.Info("deregistered all task definition revisions", "family", family, "image", image)
	return nil
}
