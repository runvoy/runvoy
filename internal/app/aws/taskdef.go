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
// NOTE: This is approximate - images should be read from container definitions or tags, not family names.
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

// unmarkExistingDefaultImages removes the runvoy.default tag from all existing task definitions
// that have it. This ensures only one image can be marked as default at a time.
func unmarkExistingDefaultImages(
	ctx context.Context,
	ecsClient *ecs.Client,
	logger *slog.Logger,
) error {
	// List all task definitions with our prefix
	listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: awsStd.String(constants.TaskDefinitionFamilyPrefix),
		Status:       ecsTypes.TaskDefinitionStatusActive,
		MaxResults:   awsStd.Int32(100),
	})
	if err != nil {
		return fmt.Errorf("failed to list task definitions: %w", err)
	}

	// Check each task definition for the default tag
	for _, taskDefARN := range listOutput.TaskDefinitionArns {
		tagsOutput, err := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
		})
		if err != nil {
			logger.Warn("failed to list tags for task definition", "arn", taskDefARN, "error", err)
			continue
		}

		// Check if this task definition is marked as default
		hasDefaultTag := false
		for _, tag := range tagsOutput.Tags {
			if tag.Key != nil && *tag.Key == "runvoy.default" && tag.Value != nil && *tag.Value == "true" {
				hasDefaultTag = true
				break
			}
		}

		// Remove the default tag if present
		if hasDefaultTag {
			_, err := ecsClient.UntagResource(ctx, &ecs.UntagResourceInput{
				ResourceArn: awsStd.String(taskDefARN),
				TagKeys:     []string{"runvoy.default"},
			})
			if err != nil {
				logger.Warn("failed to remove default tag from task definition", "arn", taskDefARN, "error", err)
				// Continue with other task definitions
			} else {
				logger.Info("removed default tag from existing task definition", "arn", taskDefARN)
			}
		}
	}

	return nil
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
// The Docker image is stored in a task definition tag for reliable retrieval.
// If isDefault is true, the image will be tagged as the default image.
func RegisterTaskDefinitionForImage(
	ctx context.Context,
	ecsClient *ecs.Client,
	cfg *Config,
	image string,
	region string,
	logger *slog.Logger,
) (string, error) {
	return RegisterTaskDefinitionForImageWithDefault(ctx, ecsClient, cfg, image, false, region, logger)
}

// RegisterTaskDefinitionForImageWithDefault registers a task definition with explicit default flag.
func RegisterTaskDefinitionForImageWithDefault(
	ctx context.Context,
	ecsClient *ecs.Client,
	cfg *Config,
	image string,
	isDefault bool,
	region string,
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

	// If marking as default, first unmark any existing default images to enforce single default
	if isDefault || (cfg.DefaultImage != "" && image == cfg.DefaultImage) {
		if err := unmarkExistingDefaultImages(ctx, ecsClient, logger); err != nil {
			logger.Warn("failed to unmark existing default images, proceeding anyway", "error", err)
			// Continue - we'll still mark this one as default
		}
	}

	// Build tags for the task definition
	// Store the actual Docker image in a tag for reliable retrieval
	tags := []ecsTypes.Tag{
		{
			Key:   awsStd.String("runvoy.image"),
			Value: awsStd.String(image),
		},
		{
			Key:   awsStd.String("Application"),
			Value: awsStd.String("runvoy"),
		},
	}

	// Mark as default if explicitly requested or if it matches the config default
	if isDefault || (cfg.DefaultImage != "" && image == cfg.DefaultImage) {
		tags = append(tags, ecsTypes.Tag{
			Key:   awsStd.String("runvoy.default"),
			Value: awsStd.String("true"),
		})
	}

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
						"awslogs-group":        cfg.LogGroup,
						"awslogs-region":       region,
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
						"awslogs-group":        cfg.LogGroup,
						"awslogs-region":       region,
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

	// Tag the task definition resource with image and default information
	// Tags are applied to the resource ARN, not included in RegisterTaskDefinitionInput
	if len(tags) > 0 {
		_, tagErr := ecsClient.TagResource(ctx, &ecs.TagResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
			Tags:        tags,
		})
		if tagErr != nil {
			// Log warning but don't fail - tags are metadata, task definition is registered
			logger.Warn("failed to tag task definition (task definition registered successfully)", "arn", taskDefARN, "error", tagErr)
		}
	}

	logger.Info("registered task definition", "family", family, "arn", taskDefARN, "image", image)
	return taskDefARN, nil
}

// ListRegisteredImages lists all registered Docker images by querying ECS task definitions.
// Images are extracted from task definition tags or container definitions.
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
		// Describe the task definition to get container definitions
		descOutput, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: awsStd.String(taskDefARN),
		})
		if err != nil {
			logger.Warn("failed to describe task definition", "arn", taskDefARN, "error", err)
			continue
		}

		if descOutput.TaskDefinition == nil {
			continue
		}

		// Extract image from container definition (runner container) - this is the source of truth
		image := ""
		for _, container := range descOutput.TaskDefinition.ContainerDefinitions {
			if container.Name != nil && *container.Name == constants.RunnerContainerName && container.Image != nil {
				image = *container.Image
				break
			}
		}

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
