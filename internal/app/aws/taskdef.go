// Package aws provides AWS-specific implementations for runvoy.
package aws

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
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
	imagePart := strings.TrimPrefix(familyName, prefix)
	return imagePart
}

// listTaskDefinitionsByPrefix lists all active task definitions whose family name starts with the given prefix.
// It handles pagination internally and filters by checking the task definition family name (extracted from ARN).
// This is necessary because the FamilyPrefix parameter in ListTaskDefinitions doesn't work as expected
// for prefix matching - it requires exact family match rather than prefix matching.
func listTaskDefinitionsByPrefix(ctx context.Context, ecsClient *ecs.Client, prefix string) ([]string, error) {
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
	ecsClient *ecs.Client,
	logger *slog.Logger,
) (string, error) {
	familyPrefix := constants.TaskDefinitionFamilyPrefix + "-"
	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, ecsClient, familyPrefix)
	if err != nil {
		return "", err
	}

	logger.Debug("calling external service", "context", map[string]string{
		"operation":    "ECS.ListTagsForResource",
		"resourceArns": strings.Join(taskDefArns, ", "),
	})

	for _, taskDefARN := range taskDefArns {
		tagsOutput, err := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
		})
		if err != nil {
			logger.Debug("failed to list tags for task definition", "context", map[string]string{
				"arn":   taskDefARN,
				"error": err.Error(),
			})
			continue
		}

		isDefault := false
		var dockerImage string
		for _, tag := range tagsOutput.Tags {
			if tag.Key != nil && tag.Value != nil {
				if *tag.Key == constants.TaskDefinitionIsDefaultTagKey && *tag.Value == "true" {
					isDefault = true
				}
				if *tag.Key == constants.TaskDefinitionDockerImageTagKey {
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

// hasExistingDefaultImage checks if any task definition has the IsDefault tag set.
func hasExistingDefaultImage(
	ctx context.Context,
	ecsClient *ecs.Client,
	logger *slog.Logger,
) (bool, error) {
	familyPrefix := constants.TaskDefinitionFamilyPrefix + "-"
	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, ecsClient, familyPrefix)
	if err != nil {
		return false, err
	}

	logger.Debug("calling external service", "context", map[string]string{
		"operation":    "ECS.ListTagsForResource",
		"resourceArns": strings.Join(taskDefArns, ", "),
	})

	for _, taskDefARN := range taskDefArns {
		tagsOutput, err := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
		})
		if err != nil {
			logger.Debug("failed to list tags for task definition", "context", map[string]string{
				"arn":   taskDefARN,
				"error": err.Error(),
			})
			continue
		}

		for _, tag := range tagsOutput.Tags {
			if tag.Key != nil && *tag.Key == constants.TaskDefinitionIsDefaultTagKey &&
				tag.Value != nil && *tag.Value == "true" {
				return true, nil
			}
		}
	}

	return false, nil
}

// unmarkExistingDefaultImages removes the IsDefault tag from all existing task definitions
// that have it. This ensures only one image can be marked as default at a time.
func unmarkExistingDefaultImages(
	ctx context.Context,
	ecsClient *ecs.Client,
	logger *slog.Logger,
) error {
	familyPrefix := constants.TaskDefinitionFamilyPrefix + "-"
	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, ecsClient, familyPrefix)
	if err != nil {
		return err
	}

	for _, taskDefARN := range taskDefArns {
		tagsOutput, err := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
		})
		if err != nil {
			logger.Warn("failed to list tags for task definition", "context", map[string]string{
				"arn":   taskDefARN,
				"error": err.Error(),
			})
			continue
		}

		hasDefaultTag := false
		for _, tag := range tagsOutput.Tags {
			if tag.Key != nil && *tag.Key == constants.TaskDefinitionIsDefaultTagKey &&
				tag.Value != nil && *tag.Value == "true" {
				hasDefaultTag = true
				break
			}
		}

		if hasDefaultTag {
			_, err := ecsClient.UntagResource(ctx, &ecs.UntagResourceInput{
				ResourceArn: awsStd.String(taskDefARN),
				TagKeys:     []string{constants.TaskDefinitionIsDefaultTagKey},
			})
			if err != nil {
				logger.Warn("failed to remove default tag from task definition", "context", map[string]string{
					"arn":    taskDefARN,
					"error":  err.Error(),
					"tagKey": constants.TaskDefinitionIsDefaultTagKey,
				})
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
		logger.Debug("task definition found", "context", map[string]string{
			"family": family,
			"arn":    latestARN,
		})
		return latestARN, nil
	}

	return "", fmt.Errorf("task definition for image %q not found (family: %s). Image must be registered via /api/v1/images/register",
		image, family)
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
	isDefault *bool,
	region string,
	logger *slog.Logger,
) error {
	family := TaskDefinitionFamilyName(image)

	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, ecsClient, family)
	if err != nil {
		return err
	}

	var existingTaskDefARN string
	if len(taskDefArns) > 0 {
		existingTaskDefARN = taskDefArns[len(taskDefArns)-1]
		logger.Debug("task definition already exists", "context", map[string]string{
			"family": family,
			"arn":    existingTaskDefARN,
		})
	}

	if isDefault != nil && *isDefault {
		if err := unmarkExistingDefaultImages(ctx, ecsClient, logger); err != nil {
			logger.Warn("failed to unmark existing default images, proceeding anyway", "error", err)
		}
	} else {
		if existingTaskDefARN != "" {
			_, err := ecsClient.UntagResource(ctx, &ecs.UntagResourceInput{
				ResourceArn: awsStd.String(existingTaskDefARN),
				TagKeys:     []string{constants.TaskDefinitionIsDefaultTagKey},
			})
			if err != nil {
				logger.Debug("failed to remove default tag from existing task definition (may not have had it)", "arn", existingTaskDefARN, "error", err)
			} else {
				logger.Info("removed default tag from existing task definition", "arn", existingTaskDefARN)
			}
		}
	}

	// If task definition already exists, just update its tags and return
	if existingTaskDefARN != "" {
		tags := []ecsTypes.Tag{
			{
				Key:   awsStd.String(constants.TaskDefinitionDockerImageTagKey),
				Value: awsStd.String(image),
			},
			{
				Key:   awsStd.String("Application"),
				Value: awsStd.String(constants.ProjectName),
			},
		}

		if isDefault != nil && *isDefault {
			tags = append(tags, ecsTypes.Tag{
				Key:   awsStd.String(constants.TaskDefinitionIsDefaultTagKey),
				Value: awsStd.String("true"),
			})
		}

		_, tagErr := ecsClient.TagResource(ctx, &ecs.TagResourceInput{
			ResourceArn: awsStd.String(existingTaskDefARN),
			Tags:        tags,
		})
		if tagErr != nil {
			logger.Warn("failed to tag existing task definition", "context", map[string]string{
				"arn":   existingTaskDefARN,
				"error": tagErr.Error(),
			})
			return fmt.Errorf("failed to update tags on existing task definition: %w", tagErr)
		}

		logger.Info("updated tags on existing task definition", "context", map[string]string{
			"family":    family,
			"arn":       existingTaskDefARN,
			"image":     image,
			"isDefault": strconv.FormatBool(isDefault != nil && *isDefault),
		})
		return nil
	}

	// Task definition doesn't exist, create it
	taskExecRoleARN := cfg.TaskExecRoleARN
	taskRoleARN := cfg.TaskRoleARN

	if taskExecRoleARN == "" || taskRoleARN == "" {
		allFamilies, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			MaxResults: awsStd.Int32(1),
		})
		if err == nil && len(allFamilies.TaskDefinitionArns) > 0 {
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

	if taskExecRoleARN == "" {
		return fmt.Errorf("task execution role ARN is required but not found in config or existing task definitions")
	}

	tags := []ecsTypes.Tag{
		{
			Key:   awsStd.String(constants.TaskDefinitionDockerImageTagKey),
			Value: awsStd.String(image),
		},
		{
			Key:   awsStd.String("Application"),
			Value: awsStd.String("runvoy"),
		},
	}

	if isDefault != nil && *isDefault {
		tags = append(tags, ecsTypes.Tag{
			Key:   awsStd.String(constants.TaskDefinitionIsDefaultTagKey),
			Value: awsStd.String("true"),
		})
	}

	registerInput := &ecs.RegisterTaskDefinitionInput{
		Family:      awsStd.String(family),
		NetworkMode: ecsTypes.NetworkModeAwsvpc,
		RequiresCompatibilities: []ecsTypes.Compatibility{
			ecsTypes.CompatibilityFargate,
		},
		Cpu:              awsStd.String("256"),
		Memory:           awsStd.String("512"),
		ExecutionRoleArn: awsStd.String(taskExecRoleARN),
		EphemeralStorage: &ecsTypes.EphemeralStorage{
			SizeInGiB: 21,
		},
		Volumes: []ecsTypes.Volume{
			{
				Name: awsStd.String(constants.SharedVolumeName),
			},
		},
		ContainerDefinitions: []ecsTypes.ContainerDefinition{
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
						"awslogs-group":         cfg.LogGroup,
						"awslogs-region":        region,
						"awslogs-stream-prefix": "sidecar",
					},
				},
			},
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
						"awslogs-group":         cfg.LogGroup,
						"awslogs-region":        region,
						"awslogs-stream-prefix": "task",
					},
				},
			},
		},
	}

	if taskRoleARN != "" {
		registerInput.TaskRoleArn = awsStd.String(taskRoleARN)
	}

	registerOutput, err := ecsClient.RegisterTaskDefinition(ctx, registerInput)
	if err != nil {
		return fmt.Errorf("failed to register task definition: %w", err)
	}

	taskDefARN := awsStd.ToString(registerOutput.TaskDefinition.TaskDefinitionArn)

	if len(tags) > 0 {
		_, tagErr := ecsClient.TagResource(ctx, &ecs.TagResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
			Tags:        tags,
		})
		if tagErr != nil {
			logger.Warn("failed to tag task definition (task definition registered successfully)", "arn", taskDefARN, "error", tagErr)
		}
	}

	logger.Info("registered task definition", "family", family, "arn", taskDefARN, "image", image)
	return nil
}

// DeregisterTaskDefinitionsForImage deregisters all task definition revisions for a given image.
// If the removed image was the default and only one image remains, that image becomes the new default.
func DeregisterTaskDefinitionsForImage(
	ctx context.Context,
	ecsClient *ecs.Client,
	image string,
	logger *slog.Logger,
) error {
	family := TaskDefinitionFamilyName(image)

	// Check if the image being removed is the default
	wasDefault := false
	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, ecsClient, family)
	if err != nil {
		logger.Warn("failed to check if image is default before removal", "error", err)
	} else {
		for _, taskDefARN := range taskDefArns {
			tagsOutput, err := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
				ResourceArn: awsStd.String(taskDefARN),
			})
			if err == nil && tagsOutput != nil {
				for _, tag := range tagsOutput.Tags {
					if tag.Key != nil && *tag.Key == constants.TaskDefinitionIsDefaultTagKey &&
						tag.Value != nil && *tag.Value == "true" {
						wasDefault = true
						break
					}
				}
			}
			if wasDefault {
				break
			}
		}
	}

	nextToken := ""

	logger.Debug("calling external service", "context", map[string]string{
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
			MaxResults:   awsStd.Int32(100),
			NextToken:    awsStd.String(nextToken),
		})
		if err != nil {
			return fmt.Errorf("failed to list task definitions: %w", err)
		}

		for _, taskDefARN := range listOutput.TaskDefinitionArns {
			_, err := ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: awsStd.String(taskDefARN),
			})
			if err != nil {
				logger.Error("failed to deregister task definition revision", "context", map[string]string{
					"family": family,
					"image":  image,
					"arn":    taskDefARN,
					"error":  err.Error(),
				})
				return fmt.Errorf("failed to deregister task definition revision: %w", err)
			}

			logger.Info("deregistered task definition revision", "context", map[string]string{
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

	logger.Info("deregistered all task definition revisions", "context", map[string]string{
		"family": family,
		"image":  image,
	})

	// If the removed image was the default, check if there's exactly one image left
	// and mark it as default if needed
	if wasDefault {
		familyPrefix := constants.TaskDefinitionFamilyPrefix + "-"
		remainingTaskDefs, err := listTaskDefinitionsByPrefix(ctx, ecsClient, familyPrefix)
		if err != nil {
			logger.Warn("failed to list remaining task definitions after removal", "error", err)
			return nil
		}

		// Count unique images remaining
		remainingImages := make(map[string]string) // map[image]taskDefARN
		for _, taskDefARN := range remainingTaskDefs {
			descOutput, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: awsStd.String(taskDefARN),
			})
			if err != nil {
				logger.Error("failed to describe task definition", "context", map[string]string{
					"family": family,
					"arn":    taskDefARN,
					"error":  err.Error(),
				})
				continue
			}

			if descOutput.TaskDefinition == nil {
				continue
			}

			for _, container := range descOutput.TaskDefinition.ContainerDefinitions {
				if container.Name != nil && *container.Name == constants.RunnerContainerName && container.Image != nil {
					remainingImages[*container.Image] = taskDefARN
					break
				}
			}
		}

		// If exactly one image remains, mark it as default
		if len(remainingImages) == 1 {
			var lastImage string
			var lastTaskDefARN string
			for img, arn := range remainingImages {
				lastImage = img
				lastTaskDefARN = arn
			}

			logger.Info("only one image remaining after removing default, marking it as default",
				"image", lastImage)

			tags := []ecsTypes.Tag{
				{
					Key:   awsStd.String(constants.TaskDefinitionIsDefaultTagKey),
					Value: awsStd.String("true"),
				},
				{
					Key:   awsStd.String(constants.TaskDefinitionDockerImageTagKey),
					Value: awsStd.String(lastImage),
				},
			}

			_, tagErr := ecsClient.TagResource(ctx, &ecs.TagResourceInput{
				ResourceArn: awsStd.String(lastTaskDefARN),
				Tags:        tags,
			})
			if tagErr != nil {
				logger.Warn("failed to tag last remaining image as default", "context", map[string]string{
					"image": lastImage,
					"arn":   lastTaskDefARN,
					"error": tagErr.Error(),
				})
			} else {
				logger.Info("marked last remaining image as default", "context", map[string]string{
					"image": lastImage,
					"arn":   lastTaskDefARN,
				})
			}
		}
	}

	return nil
}
