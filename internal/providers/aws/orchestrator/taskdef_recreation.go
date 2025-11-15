// Package orchestrator provides AWS-specific implementations for runvoy orchestrator.
// This file contains reusable task definition recreation logic for health manager.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// RecreateTaskDefinition recreates a task definition from stored metadata.
// This function is used by the health manager to restore missing task definitions.
func RecreateTaskDefinition(
	ctx context.Context,
	ecsClient Client,
	cfg *Config,
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
	registerInput := buildTaskDefinitionInput(
		ctx, family, image, taskExecRoleARN, taskRoleARN, cfg.Region, cpuStr, memoryStr, runtimePlatform, cfg,
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

	var isDefaultPtr *bool
	if isDefault {
		isDefaultPtr = awsStd.Bool(true)
	}
	tags := BuildTaskDefinitionTags(image, isDefaultPtr)
	if len(tags) > 0 {
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

	reqLogger.Info("task definition recreated", "context", map[string]string{
		"family":              family,
		"task_definition_arn": taskDefARN,
	})

	return taskDefARN, nil
}

// UpdateTaskDefinitionTags updates tags on an existing task definition to match expected values.
// This function is exported for use by the health manager.
func UpdateTaskDefinitionTags(
	ctx context.Context,
	ecsClient Client,
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

	// Build map of current tags
	currentTagMap := make(map[string]string)
	for _, tag := range tagsOutput.Tags {
		if tag.Key != nil && tag.Value != nil {
			currentTagMap[*tag.Key] = *tag.Value
		}
	}

	// Build map of expected tags
	expectedTagMap := make(map[string]string)
	for _, tag := range expectedTags {
		if tag.Key != nil && tag.Value != nil {
			expectedTagMap[*tag.Key] = *tag.Value
		}
	}

	// Find tags to add/update
	tagsToAdd := []ecsTypes.Tag{}
	for _, tag := range expectedTags {
		if tag.Key != nil {
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
	}

	// Find tags to remove (standard tags that shouldn't be there)
	keysToRemove := []string{}
	for key := range currentTagMap {
		// Remove tags that are in current but not in expected (for standard tags)
		if _, exists := expectedTagMap[key]; !exists {
			// Only remove standard tags, not custom ones
			if key == awsConstants.TaskDefinitionDockerImageTagKey ||
				key == awsConstants.TaskDefinitionIsDefaultTagKey ||
				key == "Application" ||
				key == "ManagedBy" {
				keysToRemove = append(keysToRemove, key)
			}
		}
	}

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
