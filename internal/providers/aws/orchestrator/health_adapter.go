// Package orchestrator provides AWS-specific implementations for runvoy orchestrator.
// This file provides an adapter for the health manager to use orchestrator functions.
package orchestrator

import (
	"context"
	"log/slog"

	awsClient "runvoy/internal/providers/aws/client"

	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// TaskDefRecreatorAdapter adapts orchestrator functions for use by the health manager.
// This breaks the circular dependency between health and orchestrator packages.
type TaskDefRecreatorAdapter struct {
	ecsClient awsClient.ECSClient
	cfg       *Config
}

// NewTaskDefRecreatorAdapter creates a new adapter for task definition recreation.
func NewTaskDefRecreatorAdapter(ecsClient awsClient.ECSClient, cfg *Config) *TaskDefRecreatorAdapter {
	return &TaskDefRecreatorAdapter{
		ecsClient: ecsClient,
		cfg:       cfg,
	}
}

// RecreateTaskDefinition recreates a task definition from stored metadata.
func (a *TaskDefRecreatorAdapter) RecreateTaskDefinition(
	ctx context.Context,
	family string,
	image string,
	taskRoleARN string,
	taskExecRoleARN string,
	cpu, memory int,
	runtimePlatform string,
	isDefault bool,
	reqLogger *slog.Logger,
) (string, error) {
	return RecreateTaskDefinition(
		ctx,
		a.ecsClient,
		a.cfg,
		family,
		image,
		taskRoleARN,
		taskExecRoleARN,
		cpu,
		memory,
		runtimePlatform,
		isDefault,
		reqLogger,
	)
}

// BuildTaskDefinitionTags builds the expected tags for a task definition.
func (a *TaskDefRecreatorAdapter) BuildTaskDefinitionTags(image string, isDefault *bool) []ecsTypes.Tag {
	return BuildTaskDefinitionTags(image, isDefault)
}

// UpdateTaskDefinitionTags updates tags on an existing task definition.
func (a *TaskDefRecreatorAdapter) UpdateTaskDefinitionTags(
	ctx context.Context,
	taskDefARN string,
	image string,
	isDefault bool,
	reqLogger *slog.Logger,
) error {
	return UpdateTaskDefinitionTags(ctx, a.ecsClient, taskDefARN, image, isDefault, reqLogger)
}
