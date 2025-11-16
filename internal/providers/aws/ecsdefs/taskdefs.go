// Package ecsdefs contains shared ECS task definition utilities for AWS providers.
// It is intentionally decoupled from the orchestrator package so it can be reused
// by both the orchestrator and the health manager without creating import cycles.
package ecsdefs

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"runvoy/internal/constants"
	"runvoy/internal/logger"
	awsClient "runvoy/internal/providers/aws/client"
	awsConstants "runvoy/internal/providers/aws/constants"
	"runvoy/internal/providers/aws/secrets"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// TaskDefinitionConfig contains configuration needed to build task definitions.
type TaskDefinitionConfig struct {
	LogGroup string
	Region   string
}

// BuildTaskDefinitionTags creates the tags to be applied to a task definition.
func BuildTaskDefinitionTags(image string, isDefault *bool) []ecsTypes.Tag {
	tags := []ecsTypes.Tag{
		{
			Key:   awsStd.String(awsConstants.TaskDefinitionDockerImageTagKey),
			Value: awsStd.String(image),
		},
	}

	// Add standard tags (Application, ManagedBy)
	standardTags := secrets.GetStandardTags()
	for _, tag := range standardTags {
		tags = append(tags, ecsTypes.Tag{
			Key:   awsStd.String(tag.Key),
			Value: awsStd.String(tag.Value),
		})
	}

	if isDefault != nil && *isDefault {
		tags = append(tags, ecsTypes.Tag{
			Key:   awsStd.String(awsConstants.TaskDefinitionIsDefaultTagKey),
			Value: awsStd.String(awsConstants.TaskDefinitionIsDefaultTagValue),
		})
	}

	return tags
}

// RecreateTaskDefinition recreates a task definition from stored metadata.
// This function is used by the health manager to restore missing task definitions.
func RecreateTaskDefinition(
	ctx context.Context,
	ecsClient awsClient.ECSClient,
	cfg *TaskDefinitionConfig,
	family string,
	image string,
	taskRoleARN string,
	taskExecRoleARN string,
	cpu, memory int,
	runtimePlatform string,
	isDefault bool,
	reqLogger *slog.Logger,
) (string, error) {
	cpuStr := fmt.Sprintf("%d", cpu)
	memoryStr := fmt.Sprintf("%d", memory)
	registerInput := BuildTaskDefinitionInputForConfig(
		ctx,
		family,
		image,
		taskExecRoleARN,
		taskRoleARN,
		cfg.LogGroup,
		cfg.Region,
		cpuStr,
		memoryStr,
		runtimePlatform,
	)

	logArgs := []any{
		"operation", "ECS.RegisterTaskDefinition",
		"family", family,
		"image", image,
		"task_role_arn", taskRoleARN,
		"task_exec_role_arn", taskExecRoleARN,
	}
	logArgs = append(logArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("recreating task definition", "context", logger.SliceToMap(logArgs))

	output, err := ecsClient.RegisterTaskDefinition(ctx, registerInput)
	if err != nil {
		return "", fmt.Errorf("ECS RegisterTaskDefinition failed: %w", err)
	}

	if output.TaskDefinition == nil || output.TaskDefinition.TaskDefinitionArn == nil {
		return "", fmt.Errorf("ECS returned nil task definition")
	}

	taskDefARN := *output.TaskDefinition.TaskDefinitionArn

	tagTaskDefinition(ctx, ecsClient, taskDefARN, family, image, isDefault, reqLogger)

	reqLogger.Info("task definition recreated", "context", map[string]string{
		"family":              family,
		"task_definition_arn": taskDefARN,
	})

	return taskDefARN, nil
}

func tagTaskDefinition(
	ctx context.Context,
	ecsClient awsClient.ECSClient,
	taskDefARN string,
	family string,
	image string,
	isDefault bool,
	reqLogger *slog.Logger,
) {
	var isDefaultPtr *bool
	if isDefault {
		isDefaultPtr = awsStd.Bool(true)
	}
	tags := BuildTaskDefinitionTags(image, isDefaultPtr)
	if len(tags) == 0 {
		return
	}

	tagLogArgs := []any{
		"operation", "ECS.TagResource",
		"task_definition_arn", taskDefARN,
		"family", family,
	}
	tagLogArgs = append(tagLogArgs, logger.GetDeadlineInfo(ctx)...)
	reqLogger.Debug("tagging task definition", "context", logger.SliceToMap(tagLogArgs))

	_, tagErr := ecsClient.TagResource(ctx, &ecs.TagResourceInput{
		ResourceArn: awsStd.String(taskDefARN),
		Tags:        tags,
	})
	if tagErr != nil {
		reqLogger.Warn(
			"failed to tag task definition (task definition registered successfully)",
			"arn", taskDefARN,
			"error", tagErr,
		)
	} else {
		reqLogger.Debug("task definition tagged successfully", "arn", taskDefARN)
	}
}

// UpdateTaskDefinitionTags updates tags on an existing task definition to match expected values.
func UpdateTaskDefinitionTags(
	ctx context.Context,
	ecsClient awsClient.ECSClient,
	taskDefARN string,
	image string,
	isDefault bool,
	reqLogger *slog.Logger,
) error {
	var isDefaultPtr *bool
	if isDefault {
		isDefaultPtr = awsStd.Bool(true)
	}
	expectedTags := BuildTaskDefinitionTags(image, isDefaultPtr)

	// Get current tags
	tagsOutput, err := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
		ResourceArn: awsStd.String(taskDefARN),
	})
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}

	currentTagMap := buildTagMap(tagsOutput.Tags)
	expectedTagMap := buildTagMap(expectedTags)

	tagsToAdd := findTagsToAdd(expectedTags, currentTagMap)
	keysToRemove := findTagsToRemove(currentTagMap, expectedTagMap)

	// Update tags
	if len(tagsToAdd) > 0 {
		_, err = ecsClient.TagResource(ctx, &ecs.TagResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
			Tags:        tagsToAdd,
		})
		if err != nil {
			return fmt.Errorf("failed to update tags: %w", err)
		}
		reqLogger.Debug("updated task definition tags", "arn", taskDefARN, "tags_count", len(tagsToAdd))
	}

	// Remove tags that shouldn't be there
	if len(keysToRemove) > 0 {
		_, err = ecsClient.UntagResource(ctx, &ecs.UntagResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
			TagKeys:     keysToRemove,
		})
		if err != nil {
			reqLogger.Warn("failed to remove tags", "arn", taskDefARN, "error", err)
		} else {
			reqLogger.Debug("removed task definition tags", "arn", taskDefARN, "tags_count", len(keysToRemove))
		}
	}

	return nil
}

func buildTagMap(tags []ecsTypes.Tag) map[string]string {
	tagMap := make(map[string]string)
	for _, tag := range tags {
		if tag.Key != nil && tag.Value != nil {
			tagMap[*tag.Key] = *tag.Value
		}
	}
	return tagMap
}

func findTagsToAdd(expectedTags []ecsTypes.Tag, currentTagMap map[string]string) []ecsTypes.Tag {
	tagsToAdd := []ecsTypes.Tag{}
	for _, tag := range expectedTags {
		if tag.Key == nil {
			continue
		}
		key := *tag.Key
		currentValue, exists := currentTagMap[key]
		expectedValue := ""
		if tag.Value != nil {
			expectedValue = *tag.Value
		}
		if !exists || currentValue != expectedValue {
			tagsToAdd = append(tagsToAdd, tag)
		}
	}
	return tagsToAdd
}

func findTagsToRemove(currentTagMap, expectedTagMap map[string]string) []string {
	keysToRemove := []string{}
	for key := range currentTagMap {
		if _, exists := expectedTagMap[key]; exists {
			continue
		}
		if isStandardTag(key) {
			keysToRemove = append(keysToRemove, key)
		}
	}
	return keysToRemove
}

func isStandardTag(key string) bool {
	return key == awsConstants.TaskDefinitionDockerImageTagKey ||
		key == awsConstants.TaskDefinitionIsDefaultTagKey ||
		key == constants.ResourceApplicationTagKey ||
		key == constants.ResourceManagedByTagKey
}

// parseRuntimePlatform splits runtime_platform into OS and Architecture for ECS API.
// Format: OS/ARCH matching ECS format (e.g., "Linux/ARM64", "Linux/X86_64").
// Returns error if runtime platform is not supported.
func parseRuntimePlatform(runtimePlatform string) (osFamily, cpuArch string, err error) {
	if !slices.Contains(awsConstants.SupportedRuntimePlatforms(), runtimePlatform) {
		return "", "", fmt.Errorf("unsupported runtime platform: %s (supported: %s)",
			runtimePlatform, strings.Join(awsConstants.SupportedRuntimePlatforms(), ", "))
	}
	parts := strings.Split(runtimePlatform, "/")
	return parts[0], parts[1], nil
}

// convertOSFamilyToECSEnum converts OS family string to ECS enum.
func convertOSFamilyToECSEnum(osFamily string) ecsTypes.OSFamily {
	upper := strings.ToUpper(osFamily)
	return ecsTypes.OSFamily(upper)
}

// BuildTaskDefinitionInputForConfig creates the RegisterTaskDefinitionInput for a new task definition.
//
//nolint:funlen // Large data structure definition
func BuildTaskDefinitionInputForConfig(
	ctx context.Context,
	family, image, taskExecRoleARN, taskRoleARN, logGroup, region string,
	cpu, memory, runtimePlatform string,
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
						"awslogs-group":         logGroup,
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
						"awslogs-group":         logGroup,
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
		// This should not happen if validation is done before calling this function.
		// Fall back to defaults.
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
