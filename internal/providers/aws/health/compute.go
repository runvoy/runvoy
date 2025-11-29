package health

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/runvoy/runvoy/internal/api"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"
	"github.com/runvoy/runvoy/internal/providers/aws/ecsdefs"
	"github.com/runvoy/runvoy/internal/providers/aws/secrets"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func (m *Manager) reconcileECSTaskDefinitions(
	ctx context.Context,
	reqLogger *slog.Logger,
) (api.ComputeHealthStatus, []api.HealthIssue, error) {
	status := api.ComputeHealthStatus{
		OrphanedResources: []string{},
	}
	issues := []api.HealthIssue{}

	images, err := m.imageRepo.ListImages(ctx)
	if err != nil {
		return status, issues, fmt.Errorf("failed to list images: %w", err)
	}
	status.TotalResources = len(images)

	seenFamilies := make(map[string]bool)

	imgIssues := m.checkImageTaskDefinitions(ctx, images, seenFamilies, reqLogger, &status)
	issues = append(issues, imgIssues...)

	orphanedIssues := m.findAndReportOrphanedTaskDefinitions(ctx, seenFamilies, reqLogger, &status)
	issues = append(issues, orphanedIssues...)

	return status, issues, nil
}

func (m *Manager) checkImageTaskDefinitions(
	ctx context.Context,
	images []api.ImageInfo,
	seenFamilies map[string]bool,
	reqLogger *slog.Logger,
	status *api.ComputeHealthStatus,
) []api.HealthIssue {
	issues := []api.HealthIssue{}

	for i := range images {
		img := &images[i]
		family := img.TaskDefinitionName
		if family == "" {
			issues = append(issues, api.HealthIssue{
				ResourceType: "ecs_task_definition",
				ResourceID:   img.ImageID,
				Severity:     "warning",
				Message:      fmt.Sprintf("Image %s has no task definition family", img.ImageID),
				Action:       "reported",
			})
			continue
		}
		seenFamilies[family] = true

		imgIssues := m.checkTaskDefinition(ctx, img, family, reqLogger, status)
		issues = append(issues, imgIssues...)
	}

	return issues
}

func (m *Manager) checkTaskDefinition(
	ctx context.Context,
	img *api.ImageInfo,
	family string,
	reqLogger *slog.Logger,
	status *api.ComputeHealthStatus,
) []api.HealthIssue {
	listOutput, listErr := m.ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: awsStd.String(family),
		Status:       ecsTypes.TaskDefinitionStatusActive,
		MaxResults:   awsStd.Int32(1),
	})
	if listErr != nil {
		return []api.HealthIssue{
			{
				ResourceType: "ecs_task_definition",
				ResourceID:   family,
				Severity:     "error",
				Message:      fmt.Sprintf("Failed to check task definition: %v", listErr),
				Action:       "reported",
			},
		}
	}

	if len(listOutput.TaskDefinitionArns) == 0 {
		return m.recreateMissingTaskDefinition(ctx, img, family, reqLogger, status)
	}

	taskDefARN := listOutput.TaskDefinitionArns[len(listOutput.TaskDefinitionArns)-1]
	return m.verifyTaskDefinitionTags(ctx, img, taskDefARN, family, reqLogger, status)
}

func (m *Manager) findAndReportOrphanedTaskDefinitions(
	ctx context.Context,
	seenFamilies map[string]bool,
	reqLogger *slog.Logger,
	status *api.ComputeHealthStatus,
) []api.HealthIssue {
	orphanedFamilies, orphanErr := m.findOrphanedTaskDefinitions(ctx, seenFamilies, reqLogger)
	if orphanErr != nil {
		reqLogger.Warn("failed to find orphaned task definitions", "error", orphanErr)
		return []api.HealthIssue{}
	}

	status.OrphanedCount = len(orphanedFamilies)
	status.OrphanedResources = orphanedFamilies

	issues := make([]api.HealthIssue, 0, len(orphanedFamilies))
	for _, family := range orphanedFamilies {
		issues = append(issues, api.HealthIssue{
			ResourceType: "ecs_task_definition",
			ResourceID:   family,
			Severity:     "warning",
			Message:      "Task definition exists in ECS but not in DynamoDB (orphaned)",
			Action:       "reported",
		})
	}

	return issues
}

func (m *Manager) recreateMissingTaskDefinition(
	ctx context.Context,
	img *api.ImageInfo,
	family string,
	reqLogger *slog.Logger,
	status *api.ComputeHealthStatus,
) []api.HealthIssue {
	reqLogger.Info("recreating missing task definition", "family", family, "image", img.Image)

	params := m.buildTaskDefParams(img)

	taskDefCfg := &ecsdefs.TaskDefinitionConfig{
		LogGroup: m.cfg.LogGroup,
	}

	taskDefARN, recreateErr := ecsdefs.RecreateTaskDefinition(
		ctx,
		m.ecsClient,
		taskDefCfg,
		family,
		img.Image,
		params.taskRoleARN,
		params.taskExecRoleARN,
		params.cpu,
		params.memory,
		params.runtimePlatform,
		params.isDefault,
		reqLogger,
	)
	if recreateErr != nil {
		return []api.HealthIssue{
			{
				ResourceType: "ecs_task_definition",
				ResourceID:   family,
				Severity:     "error",
				Message:      fmt.Sprintf("Failed to recreate task definition: %v", recreateErr),
				Action:       "requires_manual_intervention",
			},
		}
	}

	status.RecreatedCount++
	reqLogger.Info("task definition recreated", "family", family, "arn", taskDefARN)
	return []api.HealthIssue{
		{
			ResourceType: "ecs_task_definition",
			ResourceID:   family,
			Severity:     "warning",
			Message:      "Task definition was missing and has been recreated",
			Action:       "recreated",
		},
	}
}

type taskDefParams struct {
	taskRoleARN     string
	taskExecRoleARN string
	cpu             int
	memory          int
	runtimePlatform string
	isDefault       bool
}

func (m *Manager) buildTaskDefParams(img *api.ImageInfo) taskDefParams {
	taskRoleARN, taskExecRoleARN := m.buildRoleARNs(img.TaskRoleName, img.TaskExecutionRoleName)

	cpu := img.CPU
	if cpu == 0 {
		cpu = awsConstants.DefaultCPU
	}
	memory := img.Memory
	if memory == 0 {
		memory = awsConstants.DefaultMemory
	}
	runtimePlatform := img.RuntimePlatform
	if runtimePlatform == "" {
		runtimePlatform = awsConstants.DefaultRuntimePlatform
	}
	isDefault := img.IsDefault != nil && *img.IsDefault

	return taskDefParams{
		taskRoleARN:     taskRoleARN,
		taskExecRoleARN: taskExecRoleARN,
		cpu:             cpu,
		memory:          memory,
		runtimePlatform: runtimePlatform,
		isDefault:       isDefault,
	}
}

func (m *Manager) verifyTaskDefinitionTags(
	ctx context.Context,
	img *api.ImageInfo,
	taskDefARN string,
	family string,
	reqLogger *slog.Logger,
	status *api.ComputeHealthStatus,
) []api.HealthIssue {
	isDefault := img.IsDefault != nil && *img.IsDefault
	tagUpdated, tagErr := m.verifyAndUpdateTaskDefinitionTags(ctx, taskDefARN, family, img.Image, isDefault, reqLogger)
	if tagErr != nil {
		return []api.HealthIssue{
			{
				ResourceType: "ecs_task_definition",
				ResourceID:   family,
				Severity:     "warning",
				Message:      fmt.Sprintf("Failed to verify/update tags: %v", tagErr),
				Action:       "reported",
			},
		}
	}
	if tagUpdated {
		status.TagUpdatedCount++
		return []api.HealthIssue{
			{
				ResourceType: "ecs_task_definition",
				ResourceID:   family,
				Severity:     "warning",
				Message:      "Task definition tags were updated to match DynamoDB state",
				Action:       "tag_updated",
			},
		}
	}
	status.VerifiedCount++
	return []api.HealthIssue{}
}

func (m *Manager) findOrphanedTaskDefinitions(
	ctx context.Context,
	seenFamilies map[string]bool,
	_ *slog.Logger,
) ([]string, error) {
	familyPrefix := awsConstants.TaskDefinitionFamilyPrefix + "-"
	orphaned := []string{}

	nextToken := ""
	for {
		listOutput, err := m.ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			Status:     ecsTypes.TaskDefinitionStatusActive,
			NextToken:  awsStd.String(nextToken),
			MaxResults: awsStd.Int32(awsConstants.ECSTaskDefinitionMaxResults),
		})
		if err != nil {
			return orphaned, fmt.Errorf("failed to list task definitions: %w", err)
		}

		for _, taskDefARN := range listOutput.TaskDefinitionArns {
			parts := strings.Split(taskDefARN, "/")
			if len(parts) > 0 {
				familyWithRev := parts[len(parts)-1]
				familyParts := strings.Split(familyWithRev, ":")
				if len(familyParts) > 0 {
					family := familyParts[0]
					if strings.HasPrefix(family, familyPrefix) && !seenFamilies[family] {
						orphaned = append(orphaned, family)
					}
				}
			}
		}

		if listOutput.NextToken == nil {
			break
		}
		nextToken = *listOutput.NextToken
	}

	return orphaned, nil
}

func (m *Manager) verifyAndUpdateTaskDefinitionTags(
	ctx context.Context,
	taskDefARN string,
	_ string,
	image string,
	isDefault bool,
	reqLogger *slog.Logger,
) (bool, error) {
	tagsOutput, err := m.ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
		ResourceArn: awsStd.String(taskDefARN),
	})
	if err != nil {
		return false, fmt.Errorf("failed to list tags: %w", err)
	}

	var isDefaultPtr *bool
	if isDefault {
		isDefaultPtr = awsStd.Bool(true)
	}
	expectedTags := ecsdefs.BuildTaskDefinitionTags(image, isDefaultPtr)

	tagsMatch := m.compareTags(tagsOutput.Tags, expectedTags)
	if tagsMatch {
		return false, nil
	}

	err = ecsdefs.UpdateTaskDefinitionTags(ctx, m.ecsClient, taskDefARN, image, isDefault, reqLogger)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (m *Manager) compareTags(currentTags, expectedTags []ecsTypes.Tag) bool {
	currentMap := make(map[string]string)
	for _, tag := range currentTags {
		if tag.Key != nil && tag.Value != nil {
			currentMap[*tag.Key] = *tag.Value
		}
	}

	expectedMap := make(map[string]string)
	for _, tag := range expectedTags {
		if tag.Key != nil && tag.Value != nil {
			expectedMap[*tag.Key] = *tag.Value
		}
	}

	for key, expectedValue := range expectedMap {
		currentValue, exists := currentMap[key]
		if !exists || currentValue != expectedValue {
			return false
		}
	}

	standardTags := secrets.GetStandardTags()
	for _, stdTag := range standardTags {
		if currentMap[stdTag.Key] != stdTag.Value {
			return false
		}
	}

	return true
}
