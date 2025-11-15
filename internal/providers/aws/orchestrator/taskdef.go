// Package orchestrator provides AWS-specific implementations for runvoy orchestrator.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"

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
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(image, "-")
	re2 := regexp.MustCompile(`-+`)
	sanitized = re2.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

// TaskDefinitionFamilyName returns the ECS task definition family name for a given image.
// Format: {TaskDefinitionFamilyPrefix}-{sanitized-image-name}
func TaskDefinitionFamilyName(image string) string {
	sanitized := SanitizeImageNameForTaskDef(image)
	return fmt.Sprintf("%s-%s", awsConstants.TaskDefinitionFamilyPrefix, sanitized)
}

// ExtractImageFromTaskDefFamily extracts the Docker image name from a task definition family name.
// Returns empty string if the family name doesn't match the expected format.
// NOTE: This is approximate - images should be read from container definitions or tags, not family names.
func ExtractImageFromTaskDefFamily(familyName string) string {
	prefix := awsConstants.TaskDefinitionFamilyPrefix + "-"
	if !strings.HasPrefix(familyName, prefix) {
		return ""
	}
	imagePart := strings.TrimPrefix(familyName, prefix)
	return imagePart
}

// listTaskDefinitionsByPrefix lists all active task definitions whose family name starts with the given prefix.
// It handles pagination internally and filters by checking the task definition family name (extracted from ARN).
// This is necessary because the FamilyPrefix parameter in ListTaskDefinitions doesn't work as expected
// for prefix matching - it requires exact family match rather than prefix matching.
func listTaskDefinitionsByPrefix(ctx context.Context, ecsClient Client, prefix string) ([]string, error) {
	nextToken := ""
	var taskDefArns []string

	for {
		listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			Status:    ecsTypes.TaskDefinitionStatusActive,
			NextToken: awsStd.String(nextToken),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list task definitions: %w", err)
		}

		// taskDefARN format example:
		// arn:aws:ecs:us-east-2:123456789012:task-definition/runvoy-image-alpine-latest:1
		// Extract the family name (last part after "/") and filter by prefix
		for _, taskDefARN := range listOutput.TaskDefinitionArns {
			parts := strings.Split(taskDefARN, "/")
			if len(parts) > 0 &&
				strings.HasPrefix(parts[len(parts)-1], prefix) {
				taskDefArns = append(taskDefArns, taskDefARN)
			}
		}

		if listOutput.NextToken == nil {
			break
		}
		nextToken = *listOutput.NextToken
	}

	return taskDefArns, nil
}

// GetDefaultImage returns the Docker image marked as default (via IsDefault tag).
// Returns empty string if no default image is found.
func GetDefaultImage(
	ctx context.Context,
	ecsClient Client,
	log *slog.Logger,
) (string, error) {
	familyPrefix := awsConstants.TaskDefinitionFamilyPrefix + "-"
	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, ecsClient, familyPrefix)
	if err != nil {
		return "", err
	}

	log.Debug("calling external service", "context", map[string]string{
		"operation":     "ECS.ListTagsForResource",
		"resource_arns": strings.Join(taskDefArns, ", "),
	})

	for _, taskDefARN := range taskDefArns {
		var tagsOutput *ecs.ListTagsForResourceOutput
		tagsOutput, err = ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
		})
		if err != nil {
			log.Debug("failed to list tags for task definition", "context", map[string]string{
				"arn":   taskDefARN,
				"error": err.Error(),
			})
			continue
		}

		isDefault := false
		var dockerImage string
		for _, tag := range tagsOutput.Tags {
			if tag.Key != nil && tag.Value != nil {
				if *tag.Key == awsConstants.TaskDefinitionIsDefaultTagKey &&
					*tag.Value == awsConstants.TaskDefinitionIsDefaultTagValue {
					isDefault = true
				} else if *tag.Key == awsConstants.TaskDefinitionDockerImageTagKey {
					dockerImage = *tag.Value
				}
			}
		}

		if isDefault && dockerImage != "" {
			return dockerImage, nil
		}
	}

	return "", nil
}

// GetTaskDefinitionForImage looks up an existing task definition for the given Docker image.
// Returns an error if the task definition doesn't exist (does not auto-register).
func GetTaskDefinitionForImage(
	ctx context.Context,
	ecsClient Client,
	image string,
	log *slog.Logger,
) (string, error) {
	family := TaskDefinitionFamilyName(image)

	listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: awsStd.String(family),
		Status:       ecsTypes.TaskDefinitionStatusActive,
		MaxResults:   awsStd.Int32(1),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list task definitions: %w", err)
	}

	if len(listOutput.TaskDefinitionArns) > 0 {
		latestARN := listOutput.TaskDefinitionArns[len(listOutput.TaskDefinitionArns)-1]
		log.Debug("task definition found", "context", map[string]string{
			"family": family,
			"arn":    latestARN,
		})
		return latestARN, nil
	}

	return "", fmt.Errorf("task definition for image %q not found (family: %s). "+
		"Image must be registered via /api/v1/images/register",
		image, family)
}

// buildTaskDefinitionTags creates the tags to be applied to a task definition.
func buildTaskDefinitionTags(image string, isDefault *bool) []ecsTypes.Tag {
	tags := []ecsTypes.Tag{
		{
			Key:   awsStd.String(awsConstants.TaskDefinitionDockerImageTagKey),
			Value: awsStd.String(image),
		},
	}
	// Add standard tags (Application, ManagedBy)
	tags = append(tags, GetStandardECSTags()...)
	if isDefault != nil && *isDefault {
		tags = append(tags, ecsTypes.Tag{
			Key:   awsStd.String(awsConstants.TaskDefinitionIsDefaultTagKey),
			Value: awsStd.String(awsConstants.TaskDefinitionIsDefaultTagValue),
		})
	}
	return tags
}

// buildTaskDefinitionInput creates the RegisterTaskDefinitionInput for a new task definition.
// parseRuntimePlatform splits runtime_platform into OS and Architecture for ECS API.
// Format: OS/ARCH matching ECS format (e.g., "Linux/ARM64", "Linux/X86_64", "WINDOWS_SERVER_2019_CORE/X86_64").
func parseRuntimePlatform(runtimePlatform string) (osFamily, cpuArch string, err error) {
	parts := strings.Split(runtimePlatform, "/")
	if len(parts) != 2 { //nolint:mnd // Runtime platform format is OS/ARCH (2 parts)
		return "", "", fmt.Errorf("invalid runtime_platform format: expected OS/ARCH, got %s", runtimePlatform)
	}
	osFamily = parts[0]
	cpuArch = parts[1]

	// Validate known architectures
	const (
		archX86_64 = "X86_64"
		archARM64  = "ARM64"
	)
	if cpuArch != archX86_64 && cpuArch != archARM64 {
		return "", "", fmt.Errorf("unsupported architecture: %s (expected X86_64 or ARM64)", cpuArch)
	}

	return osFamily, cpuArch, nil
}

// convertOSFamilyToECSEnum converts OS family string to ECS enum.
// ECS uses uppercase enum values (LINUX, WINDOWS_SERVER_2019_CORE, etc.).
func convertOSFamilyToECSEnum(osFamily string) ecsTypes.OSFamily {
	upper := strings.ToUpper(osFamily)
	return ecsTypes.OSFamily(upper)
}

//
//nolint:funlen // Large data structure definition
func buildTaskDefinitionInput(
	ctx context.Context,
	family, image, taskExecRoleARN, taskRoleARN, region string,
	cpu, memory, runtimePlatform string,
	cfg *Config,
) *ecs.RegisterTaskDefinitionInput {
	registerInput := &ecs.RegisterTaskDefinitionInput{
		Family:      awsStd.String(family),
		NetworkMode: ecsTypes.NetworkModeAwsvpc,
		RequiresCompatibilities: []ecsTypes.Compatibility{
			ecsTypes.CompatibilityFargate,
		},
		Cpu:              awsStd.String(cpu),
		Memory:           awsStd.String(memory),
		ExecutionRoleArn: awsStd.String(taskExecRoleARN),
		EphemeralStorage: &ecsTypes.EphemeralStorage{
			SizeInGiB: awsConstants.ECSEphemeralStorageSizeGiB,
		},
		Volumes: []ecsTypes.Volume{
			{
				Name: awsStd.String(awsConstants.SharedVolumeName),
			},
		},
		ContainerDefinitions: []ecsTypes.ContainerDefinition{
			// Sidecar container
			{
				Name:      awsStd.String(awsConstants.SidecarContainerName),
				Image:     awsStd.String("public.ecr.aws/docker/library/alpine:latest"),
				Essential: awsStd.Bool(false),
				Command: []string{
					"/bin/sh",
					"-c",
					"echo \"This task definition is a template. Command will be overridden at runtime.\"",
				},
				MountPoints: []ecsTypes.MountPoint{
					{
						ContainerPath: awsStd.String(awsConstants.SharedVolumePath),
						SourceVolume:  awsStd.String(awsConstants.SharedVolumeName),
					},
				},
				LogConfiguration: &ecsTypes.LogConfiguration{
					LogDriver: ecsTypes.LogDriverAwslogs,
					Options: map[string]string{
						"awslogs-group":         cfg.LogGroup,
						"awslogs-region":        region,
						"awslogs-stream-prefix": awsConstants.LogStreamPrefix,
					},
				},
			},

			// Runner container
			{
				Name:      awsStd.String(awsConstants.RunnerContainerName),
				Image:     awsStd.String(image),
				Essential: awsStd.Bool(true),
				DependsOn: []ecsTypes.ContainerDependency{
					{
						ContainerName: awsStd.String(awsConstants.SidecarContainerName),
						Condition:     ecsTypes.ContainerConditionSuccess,
					},
				},
				Command: []string{
					"/bin/sh",
					"-c",
					"echo \"This task definition is a template. Command will be overridden at runtime.\"",
				},
				WorkingDirectory: awsStd.String(awsConstants.SharedVolumePath),
				MountPoints: []ecsTypes.MountPoint{
					{
						ContainerPath: awsStd.String(awsConstants.SharedVolumePath),
						SourceVolume:  awsStd.String(awsConstants.SharedVolumeName),
					},
				},
				LogConfiguration: &ecsTypes.LogConfiguration{
					LogDriver: ecsTypes.LogDriverAwslogs,
					Options: map[string]string{
						"awslogs-group":         cfg.LogGroup,
						"awslogs-region":        region,
						"awslogs-stream-prefix": awsConstants.LogStreamPrefix,
					},
				},
			},
		},
	}
	if taskRoleARN != "" {
		registerInput.TaskRoleArn = awsStd.String(taskRoleARN)
	}

	osFamily, cpuArch, err := parseRuntimePlatform(runtimePlatform)
	if err != nil {
		// This should not happen if validation is done before calling this function
		// But we'll use defaults as fallback
		osFamily = awsConstants.DefaultRuntimePlatformOSFamily
		cpuArch = awsConstants.DefaultRuntimePlatformArchitecture

		reqLogger := logger.DeriveRequestLogger(ctx, slog.Default())
		reqLogger.Warn("failed to parse runtime platform, falling back to defaults", "context",
			map[string]any{
				"error":            err,
				"runtime_platform": runtimePlatform,
				"os_family":        osFamily,
				"cpu_arch":         cpuArch,
			})
	}

	osFamilyEnum := convertOSFamilyToECSEnum(osFamily)

	registerInput.RuntimePlatform = &ecsTypes.RuntimePlatform{
		OperatingSystemFamily: osFamilyEnum,
		CpuArchitecture:       ecsTypes.CPUArchitecture(cpuArch),
	}

	return registerInput
}

// checkIfImageIsDefault checks if the image being removed is marked as default.
func checkIfImageIsDefault(ctx context.Context, ecsClient Client, family string, log *slog.Logger) bool {
	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, ecsClient, family)
	if err != nil {
		log.Warn("failed to check if image is default before removal", "error", err)
		return false
	}

	for _, taskDefARN := range taskDefArns {
		tagsOutput, listErr := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
		})
		if listErr == nil && tagsOutput != nil {
			for _, tag := range tagsOutput.Tags {
				if tag.Key != nil && *tag.Key == awsConstants.TaskDefinitionIsDefaultTagKey &&
					tag.Value != nil && *tag.Value == awsConstants.TaskDefinitionIsDefaultTagValue {
					return true
				}
			}
		}
	}
	return false
}

// deregisterAllTaskDefRevisions deregisters all active task definition revisions for a given family.
func deregisterAllTaskDefRevisions(
	ctx context.Context, ecsClient Client, family, image string, log *slog.Logger,
) error {
	nextToken := ""
	log.Debug("calling external service", "context", map[string]string{
		"operation": "ECS.ListTaskDefinitions",
		"family":    family,
		"image":     image,
		"status":    string(ecsTypes.TaskDefinitionStatusActive),
		"paginated": "true",
	})

	for {
		listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			FamilyPrefix: awsStd.String(family),
			Status:       ecsTypes.TaskDefinitionStatusActive,
			MaxResults:   awsStd.Int32(awsConstants.ECSTaskDefinitionMaxResults),
			NextToken:    awsStd.String(nextToken),
		})
		if err != nil {
			return fmt.Errorf("failed to list task definitions: %w", err)
		}

		for _, taskDefARN := range listOutput.TaskDefinitionArns {
			_, deregErr := ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: awsStd.String(taskDefARN),
			})
			if deregErr != nil {
				log.Error("failed to deregister task definition revision", "context", map[string]string{
					"family": family,
					"image":  image,
					"arn":    taskDefARN,
					"error":  deregErr.Error(),
				})
				return fmt.Errorf("failed to deregister task definition revision: %w", deregErr)
			}

			log.Info("deregistered task definition revision", "context", map[string]string{
				"family": family,
				"image":  image,
				"arn":    taskDefARN,
			})
		}

		if listOutput.NextToken == nil {
			break
		}
		nextToken = *listOutput.NextToken
	}

	log.Info("deregistered all task definition revisions", "context", map[string]string{
		"family": family,
		"image":  image,
	})
	return nil
}

// markLastRemainingImageAsDefault marks the last remaining image as default if needed.
//
//nolint:funlen // Complex AWS API orchestration
func markLastRemainingImageAsDefault(
	ctx context.Context, ecsClient Client, family string, log *slog.Logger,
) error {
	familyPrefix := awsConstants.TaskDefinitionFamilyPrefix + "-"
	remainingTaskDefs, err := listTaskDefinitionsByPrefix(ctx, ecsClient, familyPrefix)
	if err != nil {
		log.Warn("failed to list remaining task definitions after removal", "error", err)
		return nil
	}

	remainingImages := make(map[string]string)
	for _, taskDefARN := range remainingTaskDefs {
		descOutput, descErr := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: awsStd.String(taskDefARN),
		})
		if descErr != nil {
			log.Error("failed to describe task definition", "context", map[string]string{
				"family": family,
				"arn":    taskDefARN,
				"error":  descErr.Error(),
			})
			continue
		}

		if descOutput.TaskDefinition == nil {
			continue
		}

		for i := range descOutput.TaskDefinition.ContainerDefinitions {
			container := &descOutput.TaskDefinition.ContainerDefinitions[i]
			if container.Name != nil && *container.Name == awsConstants.RunnerContainerName && container.Image != nil {
				remainingImages[*container.Image] = taskDefARN
				break
			}
		}
	}

	if len(remainingImages) == 1 {
		var lastImage string
		var lastTaskDefARN string
		for img, arn := range remainingImages {
			lastImage = img
			lastTaskDefARN = arn
		}

		log.Info("only one image remaining after removing default, marking it as default",
			"image", lastImage)

		tags := []ecsTypes.Tag{
			{
				Key:   awsStd.String(awsConstants.TaskDefinitionIsDefaultTagKey),
				Value: awsStd.String(awsConstants.TaskDefinitionIsDefaultTagValue),
			},
			{
				Key:   awsStd.String(awsConstants.TaskDefinitionDockerImageTagKey),
				Value: awsStd.String(lastImage),
			},
		}

		_, tagErr := ecsClient.TagResource(ctx, &ecs.TagResourceInput{
			ResourceArn: awsStd.String(lastTaskDefARN),
			Tags:        tags,
		})
		if tagErr != nil {
			log.Warn("failed to tag last remaining image as default", "context", map[string]string{
				"image": lastImage,
				"arn":   lastTaskDefARN,
				"error": tagErr.Error(),
			})
		} else {
			log.Info("marked last remaining image as default", "context", map[string]string{
				"image": lastImage,
				"arn":   lastTaskDefARN,
			})
		}
	}
	return nil
}

// DeregisterTaskDefinitionsForImage deregisters all task definition revisions for a given image.
// If the removed image was the default and only one image remains, that image becomes the new default.
func DeregisterTaskDefinitionsForImage(
	ctx context.Context,
	ecsClient Client,
	image string,
	log *slog.Logger,
) error {
	family := TaskDefinitionFamilyName(image)

	wasDefault := checkIfImageIsDefault(ctx, ecsClient, family, log)

	if err := deregisterAllTaskDefRevisions(ctx, ecsClient, family, image, log); err != nil {
		return err
	}

	if wasDefault {
		if err := markLastRemainingImageAsDefault(ctx, ecsClient, family, log); err != nil {
			return err
		}
	}

	return nil
}
