// Package health provides AWS-specific health management implementation for runvoy.
// It reconciles resources between DynamoDB metadata and actual AWS services.
package health

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/backend/health"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"
	"runvoy/internal/providers/aws/secrets"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// These interfaces are defined locally to avoid circular dependency with orchestrator package.
// They match the interfaces in internal/providers/aws/orchestrator.

// ECSClient defines the interface for ECS operations needed by the health manager.
type ECSClient interface {
	ListTaskDefinitions(
		ctx context.Context,
		params *ecs.ListTaskDefinitionsInput,
		optFns ...func(*ecs.Options),
	) (*ecs.ListTaskDefinitionsOutput, error)
	ListTagsForResource(
		ctx context.Context,
		params *ecs.ListTagsForResourceInput,
		optFns ...func(*ecs.Options),
	) (*ecs.ListTagsForResourceOutput, error)
	TagResource(
		ctx context.Context,
		params *ecs.TagResourceInput,
		optFns ...func(*ecs.Options),
	) (*ecs.TagResourceOutput, error)
	UntagResource(
		ctx context.Context,
		params *ecs.UntagResourceInput,
		optFns ...func(*ecs.Options),
	) (*ecs.UntagResourceOutput, error)
	RegisterTaskDefinition(
		ctx context.Context,
		params *ecs.RegisterTaskDefinitionInput,
		optFns ...func(*ecs.Options),
	) (*ecs.RegisterTaskDefinitionOutput, error)
}

// IAMClient defines the interface for IAM operations needed by the health manager.
type IAMClient interface {
	GetRole(
		ctx context.Context,
		params *iam.GetRoleInput,
		optFns ...func(*iam.Options),
	) (*iam.GetRoleOutput, error)
}

// ImageTaskDefRepository defines the interface for image-taskdef mapping operations.
type ImageTaskDefRepository interface {
	ListImages(ctx context.Context) ([]api.ImageInfo, error)
}

// TaskDefRecreator defines the interface for recreating task definitions.
// This allows the health manager to recreate task definitions without importing orchestrator.
type TaskDefRecreator interface {
	RecreateTaskDefinition(
		ctx context.Context,
		family string,
		image string,
		taskRoleARN string,
		taskExecRoleARN string,
		cpu, memory int,
		runtimePlatform string,
		isDefault bool,
		reqLogger *slog.Logger,
	) (string, error)
	BuildTaskDefinitionTags(image string, isDefault *bool) []ecsTypes.Tag
	UpdateTaskDefinitionTags(
		ctx context.Context,
		taskDefARN string,
		image string,
		isDefault bool,
		reqLogger *slog.Logger,
	) error
}

// Manager implements the health.HealthManager interface for AWS.
type Manager struct {
	ecsClient      ECSClient
	ssmClient      secrets.Client
	iamClient      IAMClient
	imageRepo      ImageTaskDefRepository
	taskDefRecreat TaskDefRecreator
	secretsRepo    database.SecretsRepository
	cfg            *Config
	logger         *slog.Logger
	secretsPrefix  string
}

// Config holds AWS-specific configuration for the health manager.
type Config struct {
	Region                 string
	AccountID              string
	DefaultTaskRoleARN     string
	DefaultTaskExecRoleARN string
}

// NewManager creates a new AWS health manager.
func NewManager(
	ecsClient ECSClient,
	ssmClient secrets.Client,
	iamClient IAMClient,
	imageRepo ImageTaskDefRepository,
	taskDefRecreat TaskDefRecreator,
	secretsRepo database.SecretsRepository,
	cfg *Config,
	secretsPrefix string,
	log *slog.Logger,
) *Manager {
	return &Manager{
		ecsClient:      ecsClient,
		ssmClient:      ssmClient,
		iamClient:      iamClient,
		imageRepo:      imageRepo,
		taskDefRecreat: taskDefRecreat,
		secretsRepo:    secretsRepo,
		cfg:            cfg,
		secretsPrefix:  secretsPrefix,
		logger:         log,
	}
}

// Reconcile performs health checks and reconciliation for ECS task definitions, SSM parameters, and IAM roles.
func (m *Manager) Reconcile(ctx context.Context) (*health.Report, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)
	reqLogger.Info("starting health reconciliation")

	report := &health.Report{
		Timestamp: time.Now(),
		Issues:    []health.Issue{},
	}

	// Reconcile ECS task definitions
	ecsStatus, ecsIssues, err := m.reconcileECSTaskDefinitions(ctx, reqLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile ECS task definitions: %w", err)
	}
	report.ECSStatus = ecsStatus
	report.Issues = append(report.Issues, ecsIssues...)
	report.ReconciledCount += ecsStatus.RecreatedCount + ecsStatus.TagUpdatedCount

	// Reconcile SSM parameters (secrets)
	secretsStatus, secretsIssues, err := m.reconcileSecrets(ctx, reqLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile secrets: %w", err)
	}
	report.SecretsStatus = secretsStatus
	report.Issues = append(report.Issues, secretsIssues...)
	report.ReconciledCount += secretsStatus.TagUpdatedCount

	// Reconcile IAM roles
	iamStatus, iamIssues, err := m.reconcileIAMRoles(ctx, reqLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile IAM roles: %w", err)
	}
	report.IAMStatus = iamStatus
	report.Issues = append(report.Issues, iamIssues...)

	// Count errors
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			report.ErrorCount++
		}
	}

	reqLogger.Info("health reconciliation completed",
		"reconciled_count", report.ReconciledCount,
		"error_count", report.ErrorCount,
		"total_issues", len(report.Issues))

	return report, nil
}

// reconcileECSTaskDefinitions reconciles ECS task definitions with DynamoDB metadata.
func (m *Manager) reconcileECSTaskDefinitions(
	ctx context.Context,
	reqLogger *slog.Logger,
) (health.ECSHealthStatus, []health.Issue, error) {
	status := health.ECSHealthStatus{
		OrphanedFamilies: []string{},
	}
	issues := []health.Issue{}

	// Get all images from DynamoDB
	images, err := m.imageRepo.ListImages(ctx)
	if err != nil {
		return status, issues, fmt.Errorf("failed to list images: %w", err)
	}
	status.TotalImages = len(images)

	// Track families we've seen
	seenFamilies := make(map[string]bool)

	// Check each image's task definition
	imgIssues := m.checkImageTaskDefinitions(ctx, images, seenFamilies, reqLogger, &status)
	issues = append(issues, imgIssues...)

	// Find orphaned task definitions (exist in ECS but not in DynamoDB)
	orphanedIssues := m.findAndReportOrphanedTaskDefinitions(ctx, seenFamilies, reqLogger, &status)
	issues = append(issues, orphanedIssues...)

	return status, issues, nil
}

func (m *Manager) checkImageTaskDefinitions(
	ctx context.Context,
	images []api.ImageInfo,
	seenFamilies map[string]bool,
	reqLogger *slog.Logger,
	status *health.ECSHealthStatus,
) []health.Issue {
	issues := []health.Issue{}

	for i := range images {
		img := &images[i]
		family := img.TaskDefinitionName
		if family == "" {
			issues = append(issues, health.Issue{
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
	status *health.ECSHealthStatus,
) []health.Issue {
	listOutput, listErr := m.ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: awsStd.String(family),
		Status:       ecsTypes.TaskDefinitionStatusActive,
		MaxResults:   awsStd.Int32(1),
	})
	if listErr != nil {
		return []health.Issue{
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
	status *health.ECSHealthStatus,
) []health.Issue {
	orphanedFamilies, orphanErr := m.findOrphanedTaskDefinitions(ctx, seenFamilies, reqLogger)
	if orphanErr != nil {
		reqLogger.Warn("failed to find orphaned task definitions", "error", orphanErr)
		return []health.Issue{}
	}

	status.OrphanedCount = len(orphanedFamilies)
	status.OrphanedFamilies = orphanedFamilies

	issues := make([]health.Issue, 0, len(orphanedFamilies))
	for _, family := range orphanedFamilies {
		issues = append(issues, health.Issue{
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
	status *health.ECSHealthStatus,
) []health.Issue {
	reqLogger.Info("recreating missing task definition", "family", family, "image", img.Image)

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

	taskDefARN, recreateErr := m.taskDefRecreat.RecreateTaskDefinition(
		ctx, family, img.Image, taskRoleARN, taskExecRoleARN, cpu, memory, runtimePlatform, isDefault, reqLogger,
	)
	if recreateErr != nil {
		return []health.Issue{
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
	return []health.Issue{
		{
			ResourceType: "ecs_task_definition",
			ResourceID:   family,
			Severity:     "warning",
			Message:      "Task definition was missing and has been recreated",
			Action:       "recreated",
		},
	}
}

func (m *Manager) verifyTaskDefinitionTags(
	ctx context.Context,
	img *api.ImageInfo,
	taskDefARN string,
	family string,
	reqLogger *slog.Logger,
	status *health.ECSHealthStatus,
) []health.Issue {
	isDefault := img.IsDefault != nil && *img.IsDefault
	tagUpdated, tagErr := m.verifyAndUpdateTaskDefinitionTags(ctx, taskDefARN, family, img.Image, isDefault, reqLogger)
	if tagErr != nil {
		return []health.Issue{
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
		return []health.Issue{
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
	return []health.Issue{}
}

// reconcileSecrets reconciles SSM parameters (secrets) with DynamoDB metadata.
func (m *Manager) reconcileSecrets(
	ctx context.Context,
	reqLogger *slog.Logger,
) (health.SecretsHealthStatus, []health.Issue, error) {
	status := health.SecretsHealthStatus{
		OrphanedParameters: []string{},
	}
	issues := []health.Issue{}

	// Get all secrets from DynamoDB
	secretsList, err := m.secretsRepo.ListSecrets(ctx, false)
	if err != nil {
		return status, issues, fmt.Errorf("failed to list secrets: %w", err)
	}
	status.TotalSecrets = len(secretsList)

	// Track parameters we've seen
	seenParameters := make(map[string]bool)

	// Check each secret's SSM parameter
	for _, secret := range secretsList {
		parameterName := m.getParameterName(secret.Name)
		seenParameters[parameterName] = true

		secretIssues := m.checkSecretParameter(ctx, parameterName, secret.Name, reqLogger, &status)
		issues = append(issues, secretIssues...)
	}

	// Find orphaned parameters (exist in SSM but not in DynamoDB)
	orphanedParams, orphanErr := m.findOrphanedParameters(ctx, seenParameters, reqLogger)
	if orphanErr != nil {
		reqLogger.Warn("failed to find orphaned parameters", "error", orphanErr)
	} else {
		status.OrphanedCount = len(orphanedParams)
		status.OrphanedParameters = orphanedParams
		for _, param := range orphanedParams {
			issues = append(issues, health.Issue{
				ResourceType: "ssm_parameter",
				ResourceID:   param,
				Severity:     "warning",
				Message:      "Parameter exists in SSM but not in DynamoDB (orphaned)",
				Action:       "reported",
			})
		}
	}

	return status, issues, nil
}

func (m *Manager) checkSecretParameter(
	ctx context.Context,
	parameterName string,
	secretName string,
	reqLogger *slog.Logger,
	status *health.SecretsHealthStatus,
) []health.Issue {
	issues := []health.Issue{}

	// Check if parameter exists
	_, getParamErr := m.ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           awsStd.String(parameterName),
		WithDecryption: awsStd.Bool(false), // Just check existence
	})
	if getParamErr != nil {
		if isParameterNotFound(getParamErr) {
			status.MissingCount++
			issues = append(issues, health.Issue{
				ResourceType: "ssm_parameter",
				ResourceID:   secretName,
				Severity:     "error",
				Message:      "Secret parameter missing in SSM Parameter Store (cannot recreate without value)",
				Action:       "requires_manual_intervention",
			})
			reqLogger.Warn("secret parameter missing", "name", secretName, "parameter", parameterName)
		} else {
			issues = append(issues, health.Issue{
				ResourceType: "ssm_parameter",
				ResourceID:   secretName,
				Severity:     "error",
				Message:      fmt.Sprintf("Failed to check parameter: %v", getParamErr),
				Action:       "reported",
			})
		}
		return issues
	}

	// Parameter exists - verify tags
	tagUpdated, tagErr := m.verifyAndUpdateSecretTags(ctx, parameterName, secretName, reqLogger)
	if tagErr != nil {
		issues = append(issues, health.Issue{
			ResourceType: "ssm_parameter",
			ResourceID:   secretName,
			Severity:     "warning",
			Message:      fmt.Sprintf("Failed to verify/update tags: %v", tagErr),
			Action:       "reported",
		})
	} else if tagUpdated {
		status.TagUpdatedCount++
		issues = append(issues, health.Issue{
			ResourceType: "ssm_parameter",
			ResourceID:   secretName,
			Severity:     "warning",
			Message:      "Secret parameter tags were updated to match DynamoDB state",
			Action:       "tag_updated",
		})
	} else {
		status.VerifiedCount++
	}

	return issues
}

// reconcileIAMRoles reconciles IAM roles with DynamoDB metadata.
func (m *Manager) reconcileIAMRoles(
	ctx context.Context,
	_ *slog.Logger,
) (health.IAMHealthStatus, []health.Issue, error) {
	status := health.IAMHealthStatus{
		MissingRoles: []string{},
	}
	issues := []health.Issue{}

	defaultIssues := m.verifyDefaultRoles(ctx, &status)
	issues = append(issues, defaultIssues...)

	customIssues, err := m.verifyCustomRoles(ctx, &status)
	if err != nil {
		return status, issues, fmt.Errorf("failed to verify custom roles: %w", err)
	}
	issues = append(issues, customIssues...)

	return status, issues, nil
}

func (m *Manager) verifyDefaultRoles(ctx context.Context, status *health.IAMHealthStatus) []health.Issue {
	issues := []health.Issue{}

	if m.cfg.DefaultTaskRoleARN != "" {
		issues = append(issues, m.verifyRole(ctx, m.cfg.DefaultTaskRoleARN, "Default task role", status)...)
	}

	if m.cfg.DefaultTaskExecRoleARN != "" {
		issues = append(issues, m.verifyRole(ctx, m.cfg.DefaultTaskExecRoleARN, "Default task execution role", status)...)
	}

	if len(status.MissingRoles) == 0 {
		status.DefaultRolesVerified = true
	}

	return issues
}

func (m *Manager) verifyRole(
	ctx context.Context,
	roleARN string,
	roleDescription string,
	status *health.IAMHealthStatus,
) []health.Issue {
	roleName := extractRoleNameFromARN(roleARN)
	_, err := m.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: awsStd.String(roleName),
	})
	if err == nil {
		return []health.Issue{}
	}

	if strings.Contains(err.Error(), "NoSuchEntity") {
		status.MissingRoles = append(status.MissingRoles, roleARN)
		return []health.Issue{
			{
				ResourceType: "iam_role",
				ResourceID:   roleARN,
				Severity:     "error",
				Message:      fmt.Sprintf("%s missing (managed by CloudFormation)", roleDescription),
				Action:       "requires_manual_intervention",
			},
		}
	}

	return []health.Issue{
		{
			ResourceType: "iam_role",
			ResourceID:   roleARN,
			Severity:     "error",
			Message:      fmt.Sprintf("Failed to verify %s: %v", roleDescription, err),
			Action:       "reported",
		},
	}
}

func (m *Manager) verifyCustomRoles(
	ctx context.Context,
	status *health.IAMHealthStatus,
) ([]health.Issue, error) {
	images, err := m.imageRepo.ListImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	customRoles := make(map[string]bool)
	for i := range images {
		img := &images[i]
		if img.TaskRoleName != nil && *img.TaskRoleName != "" {
			customRoles[*img.TaskRoleName] = true
		}
		if img.TaskExecutionRoleName != nil && *img.TaskExecutionRoleName != "" {
			customRoles[*img.TaskExecutionRoleName] = true
		}
	}

	status.CustomRolesTotal = len(customRoles)
	issues := []health.Issue{}

	for roleName := range customRoles {
		roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", m.cfg.AccountID, roleName)
		_, getRoleErr := m.iamClient.GetRole(ctx, &iam.GetRoleInput{
			RoleName: awsStd.String(roleName),
		})
		if getRoleErr != nil {
			if strings.Contains(getRoleErr.Error(), "NoSuchEntity") {
				status.MissingRoles = append(status.MissingRoles, roleARN)
				issues = append(issues, health.Issue{
					ResourceType: "iam_role",
					ResourceID:   roleARN,
					Severity:     "error",
					Message:      "Custom IAM role missing (cannot recreate without policies)",
					Action:       "requires_manual_intervention",
				})
			} else {
				issues = append(issues, health.Issue{
					ResourceType: "iam_role",
					ResourceID:   roleARN,
					Severity:     "error",
					Message:      fmt.Sprintf("Failed to verify custom role: %v", getRoleErr),
					Action:       "reported",
				})
			}
		} else {
			status.CustomRolesVerified++
		}
	}

	return issues, nil
}

// Helper functions

func (m *Manager) buildRoleARNs(taskRoleName, taskExecutionRoleName *string) (taskRoleARN, taskExecRoleARN string) {
	taskRoleARN = ""
	taskExecRoleARN = m.cfg.DefaultTaskExecRoleARN

	if taskRoleName != nil && *taskRoleName != "" {
		taskRoleARN = fmt.Sprintf("arn:aws:iam::%s:role/%s", m.cfg.AccountID, *taskRoleName)
	} else if m.cfg.DefaultTaskRoleARN != "" {
		taskRoleARN = m.cfg.DefaultTaskRoleARN
	}

	if taskExecutionRoleName != nil && *taskExecutionRoleName != "" {
		taskExecRoleARN = fmt.Sprintf("arn:aws:iam::%s:role/%s", m.cfg.AccountID, *taskExecutionRoleName)
	}

	return taskRoleARN, taskExecRoleARN
}

func (m *Manager) verifyAndUpdateTaskDefinitionTags(
	ctx context.Context,
	taskDefARN string,
	_ string,
	image string,
	isDefault bool,
	reqLogger *slog.Logger,
) (bool, error) {
	// Get current tags
	tagsOutput, err := m.ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
		ResourceArn: awsStd.String(taskDefARN),
	})
	if err != nil {
		return false, fmt.Errorf("failed to list tags: %w", err)
	}

	// Build expected tags
	var isDefaultPtr *bool
	if isDefault {
		isDefaultPtr = awsStd.Bool(true)
	}
	expectedTags := m.taskDefRecreat.BuildTaskDefinitionTags(image, isDefaultPtr)

	// Check if tags match
	tagsMatch := m.compareTags(tagsOutput.Tags, expectedTags)
	if tagsMatch {
		return false, nil
	}

	// Tags don't match - update them
	err = m.taskDefRecreat.UpdateTaskDefinitionTags(ctx, taskDefARN, image, isDefault, reqLogger)
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

	// Check all expected tags are present and match
	for key, expectedValue := range expectedMap {
		currentValue, exists := currentMap[key]
		if !exists || currentValue != expectedValue {
			return false
		}
	}

	// Check standard tags are present
	standardTags := secrets.GetStandardTags()
	for _, stdTag := range standardTags {
		if currentMap[stdTag.Key] != stdTag.Value {
			return false
		}
	}

	return true
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
			// Extract family name from ARN
			parts := strings.Split(taskDefARN, "/")
			if len(parts) > 0 {
				familyWithRev := parts[len(parts)-1]
				// Remove revision number (e.g., "family:123" -> "family")
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

func (m *Manager) getParameterName(secretName string) string {
	return fmt.Sprintf("%s/%s", m.secretsPrefix, secretName)
}

func (m *Manager) verifyAndUpdateSecretTags(
	ctx context.Context,
	parameterName string,
	secretName string,
	reqLogger *slog.Logger,
) (bool, error) {
	// Get current tags
	tagsOutput, err := m.ssmClient.ListTagsForResource(ctx, &ssm.ListTagsForResourceInput{
		ResourceType: ssmTypes.ResourceTypeForTaggingParameter,
		ResourceId:   awsStd.String(parameterName),
	})
	if err != nil {
		return false, fmt.Errorf("failed to list tags: %w", err)
	}

	// Build expected tags
	expectedTags := secrets.GetStandardTags()
	expectedTagMap := make(map[string]string)
	for _, tag := range expectedTags {
		expectedTagMap[tag.Key] = tag.Value
	}

	// Build current tag map
	currentTagMap := make(map[string]string)
	for _, tag := range tagsOutput.TagList {
		if tag.Key != nil && tag.Value != nil {
			currentTagMap[*tag.Key] = *tag.Value
		}
	}

	// Check if tags match
	tagsMatch := true
	for key, expectedValue := range expectedTagMap {
		currentValue, exists := currentTagMap[key]
		if !exists || currentValue != expectedValue {
			tagsMatch = false
			break
		}
	}

	if tagsMatch {
		return false, nil
	}

	// Tags don't match - update them
	ssmTags := []ssmTypes.Tag{}
	for _, tag := range expectedTags {
		ssmTags = append(ssmTags, ssmTypes.Tag{
			Key:   awsStd.String(tag.Key),
			Value: awsStd.String(tag.Value),
		})
	}

	_, err = m.ssmClient.AddTagsToResource(ctx, &ssm.AddTagsToResourceInput{
		ResourceType: ssmTypes.ResourceTypeForTaggingParameter,
		ResourceId:   awsStd.String(parameterName),
		Tags:         ssmTags,
	})
	if err != nil {
		return false, fmt.Errorf("failed to update tags: %w", err)
	}

	reqLogger.Debug("updated secret parameter tags", "name", secretName, "parameter", parameterName)
	return true, nil
}

func (m *Manager) findOrphanedParameters(
	ctx context.Context,
	seenParameters map[string]bool,
	_ *slog.Logger,
) ([]string, error) {
	orphaned := []string{}

	nextToken := ""
	for {
		listOutput, err := m.ssmClient.DescribeParameters(ctx, &ssm.DescribeParametersInput{
			ParameterFilters: []ssmTypes.ParameterStringFilter{
				{
					Key:    awsStd.String("Path"),
					Option: awsStd.String("BeginsWith"),
					Values: []string{m.secretsPrefix},
				},
			},
			NextToken:  awsStd.String(nextToken),
			MaxResults: awsStd.Int32(awsConstants.SSMParameterMaxResults),
		})
		if err != nil {
			return orphaned, fmt.Errorf("failed to describe parameters: %w", err)
		}

		for i := range listOutput.Parameters {
			param := &listOutput.Parameters[i]
			if param.Name != nil {
				paramName := *param.Name
				if !seenParameters[paramName] {
					// Extract secret name from parameter path
					secretName := strings.TrimPrefix(paramName, m.secretsPrefix+"/")
					orphaned = append(orphaned, secretName)
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

func extractRoleNameFromARN(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return arn
}

func isParameterNotFound(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "ParameterNotFound") ||
		strings.Contains(errMsg, "InvalidParameter")
}
