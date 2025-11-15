// Package health provides AWS-specific health management implementation for runvoy.
// It reconciles resources between DynamoDB metadata and actual AWS services.
package health

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"runvoy/internal/backend/health"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsConstants "runvoy/internal/providers/aws/constants"
	awsOrchestrator "runvoy/internal/providers/aws/orchestrator"
	"runvoy/internal/providers/aws/secrets"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// Manager implements the health.HealthManager interface for AWS.
type Manager struct {
	ecsClient      awsOrchestrator.Client
	ssmClient      secrets.Client
	iamClient      awsOrchestrator.IAMClient
	imageRepo      awsOrchestrator.ImageTaskDefRepository
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
	ecsClient awsOrchestrator.Client,
	ssmClient secrets.Client,
	iamClient awsOrchestrator.IAMClient,
	imageRepo awsOrchestrator.ImageTaskDefRepository,
	secretsRepo database.SecretsRepository,
	cfg *Config,
	secretsPrefix string,
	log *slog.Logger,
) *Manager {
	return &Manager{
		ecsClient:     ecsClient,
		ssmClient:     ssmClient,
		iamClient:     iamClient,
		imageRepo:     imageRepo,
		secretsRepo:   secretsRepo,
		cfg:           cfg,
		secretsPrefix: secretsPrefix,
		logger:        log,
	}
}

// Reconcile performs health checks and reconciliation for ECS task definitions, SSM parameters, and IAM roles.
func (m *Manager) Reconcile(ctx context.Context) (*health.HealthReport, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)
	reqLogger.Info("starting health reconciliation")

	report := &health.HealthReport{
		Timestamp: time.Now(),
		Issues:    []health.HealthIssue{},
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
) (health.ECSHealthStatus, []health.HealthIssue, error) {
	status := health.ECSHealthStatus{
		OrphanedFamilies: []string{},
	}
	issues := []health.HealthIssue{}

	// Get all images from DynamoDB
	images, err := m.imageRepo.ListImages(ctx)
	if err != nil {
		return status, issues, fmt.Errorf("failed to list images: %w", err)
	}
	status.TotalImages = len(images)

	// Track families we've seen
	seenFamilies := make(map[string]bool)

	// Check each image's task definition
	for _, img := range images {
		family := img.TaskDefinitionName
		if family == "" {
			issues = append(issues, health.HealthIssue{
				ResourceType: "ecs_task_definition",
				ResourceID:   img.ImageID,
				Severity:     "warning",
				Message:      fmt.Sprintf("Image %s has no task definition family", img.ImageID),
				Action:       "reported",
			})
			continue
		}
		seenFamilies[family] = true

		// Check if task definition exists
		listOutput, err := m.ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			FamilyPrefix: awsStd.String(family),
			Status:       ecsTypes.TaskDefinitionStatusActive,
			MaxResults:   awsStd.Int32(1),
		})
		if err != nil {
			issues = append(issues, health.HealthIssue{
				ResourceType: "ecs_task_definition",
				ResourceID:   family,
				Severity:     "error",
				Message:      fmt.Sprintf("Failed to check task definition: %v", err),
				Action:       "reported",
			})
			continue
		}

		if len(listOutput.TaskDefinitionArns) == 0 {
			// Task definition missing - recreate it
			reqLogger.Info("recreating missing task definition", "family", family, "image", img.Image)

			// Build role ARNs
			taskRoleARN, taskExecRoleARN := m.buildRoleARNs(
				img.TaskRoleName,
				img.TaskExecutionRoleName,
			)

			// Parse CPU and memory
			cpu := awsConstants.DefaultCPU
			if img.CPU != nil {
				cpu = *img.CPU
			}
			memory := awsConstants.DefaultMemory
			if img.Memory != nil {
				memory = *img.Memory
			}
			runtimePlatform := awsConstants.DefaultRuntimePlatform
			if img.RuntimePlatform != "" {
				runtimePlatform = img.RuntimePlatform
			}

			// Create a config for recreation
			recreationCfg := &awsOrchestrator.Config{
				Region:                 m.cfg.Region,
				DefaultTaskRoleARN:     m.cfg.DefaultTaskRoleARN,
				DefaultTaskExecRoleARN: m.cfg.DefaultTaskExecRoleARN,
			}

			// Recreate task definition
			taskDefARN, recreateErr := awsOrchestrator.RecreateTaskDefinition(
				ctx,
				m.ecsClient,
				recreationCfg,
				family,
				img.Image,
				taskRoleARN,
				taskExecRoleARN,
				cpu,
				memory,
				runtimePlatform,
				img.IsDefault,
				reqLogger,
			)
			if recreateErr != nil {
				issues = append(issues, health.HealthIssue{
					ResourceType: "ecs_task_definition",
					ResourceID:    family,
					Severity:      "error",
					Message:       fmt.Sprintf("Failed to recreate task definition: %v", recreateErr),
					Action:        "requires_manual_intervention",
				})
			} else {
				status.RecreatedCount++
				issues = append(issues, health.HealthIssue{
					ResourceType: "ecs_task_definition",
					ResourceID:   family,
					Severity:     "warning",
					Message:      fmt.Sprintf("Task definition was missing and has been recreated"),
					Action:       "recreated",
				})
				reqLogger.Info("task definition recreated", "family", family, "arn", taskDefARN)
			}
		} else {
			// Task definition exists - verify tags
			taskDefARN := listOutput.TaskDefinitionArns[len(listOutput.TaskDefinitionArns)-1]
			tagUpdated, tagErr := m.verifyAndUpdateTaskDefinitionTags(
				ctx,
				taskDefARN,
				family,
				img.Image,
				img.IsDefault,
				reqLogger,
			)
			if tagErr != nil {
				issues = append(issues, health.HealthIssue{
					ResourceType: "ecs_task_definition",
					ResourceID:   family,
					Severity:     "warning",
					Message:      fmt.Sprintf("Failed to verify/update tags: %v", tagErr),
					Action:       "reported",
				})
			} else if tagUpdated {
				status.TagUpdatedCount++
				issues = append(issues, health.HealthIssue{
					ResourceType: "ecs_task_definition",
					ResourceID:   family,
					Severity:     "warning",
					Message:      "Task definition tags were updated to match DynamoDB state",
					Action:       "tag_updated",
				})
			} else {
				status.VerifiedCount++
			}
		}
	}

	// Find orphaned task definitions (exist in ECS but not in DynamoDB)
	orphanedFamilies, orphanErr := m.findOrphanedTaskDefinitions(ctx, seenFamilies, reqLogger)
	if orphanErr != nil {
		reqLogger.Warn("failed to find orphaned task definitions", "error", orphanErr)
	} else {
		status.OrphanedCount = len(orphanedFamilies)
		status.OrphanedFamilies = orphanedFamilies
		for _, family := range orphanedFamilies {
			issues = append(issues, health.HealthIssue{
				ResourceType: "ecs_task_definition",
				ResourceID:   family,
				Severity:     "warning",
				Message:      fmt.Sprintf("Task definition exists in ECS but not in DynamoDB (orphaned)"),
				Action:       "reported",
			})
		}
	}

	return status, issues, nil
}

// reconcileSecrets reconciles SSM parameters (secrets) with DynamoDB metadata.
func (m *Manager) reconcileSecrets(
	ctx context.Context,
	reqLogger *slog.Logger,
) (health.SecretsHealthStatus, []health.HealthIssue, error) {
	status := health.SecretsHealthStatus{
		OrphanedParameters: []string{},
	}
	issues := []health.HealthIssue{}

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

		// Check if parameter exists
		_, err := m.ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
			Name:           awsStd.String(parameterName),
			WithDecryption: awsStd.Bool(false), // Just check existence
		})
		if err != nil {
			// Check if it's a "not found" error
			if isParameterNotFound(err) {
				status.MissingCount++
				issues = append(issues, health.HealthIssue{
					ResourceType: "ssm_parameter",
					ResourceID:   secret.Name,
					Severity:     "error",
					Message:      fmt.Sprintf("Secret parameter missing in SSM Parameter Store (cannot recreate without value)"),
					Action:       "requires_manual_intervention",
				})
				reqLogger.Warn("secret parameter missing", "name", secret.Name, "parameter", parameterName)
			} else {
				issues = append(issues, health.HealthIssue{
					ResourceType: "ssm_parameter",
					ResourceID:   secret.Name,
					Severity:     "error",
					Message:      fmt.Sprintf("Failed to check parameter: %v", err),
					Action:       "reported",
				})
			}
			continue
		}

		// Parameter exists - verify tags
		tagUpdated, tagErr := m.verifyAndUpdateSecretTags(ctx, parameterName, secret.Name, reqLogger)
		if tagErr != nil {
			issues = append(issues, health.HealthIssue{
				ResourceType: "ssm_parameter",
				ResourceID:   secret.Name,
				Severity:     "warning",
				Message:      fmt.Sprintf("Failed to verify/update tags: %v", tagErr),
				Action:       "reported",
			})
		} else if tagUpdated {
			status.TagUpdatedCount++
			issues = append(issues, health.HealthIssue{
				ResourceType: "ssm_parameter",
				ResourceID:   secret.Name,
				Severity:     "warning",
				Message:      "Secret parameter tags were updated to match DynamoDB state",
				Action:       "tag_updated",
			})
		} else {
			status.VerifiedCount++
		}
	}

	// Find orphaned parameters (exist in SSM but not in DynamoDB)
	orphanedParams, orphanErr := m.findOrphanedParameters(ctx, seenParameters, reqLogger)
	if orphanErr != nil {
		reqLogger.Warn("failed to find orphaned parameters", "error", orphanErr)
	} else {
		status.OrphanedCount = len(orphanedParams)
		status.OrphanedParameters = orphanedParams
		for _, param := range orphanedParams {
			issues = append(issues, health.HealthIssue{
				ResourceType: "ssm_parameter",
				ResourceID:   param,
				Severity:     "warning",
				Message:      fmt.Sprintf("Parameter exists in SSM but not in DynamoDB (orphaned)"),
				Action:       "reported",
			})
		}
	}

	return status, issues, nil
}

// reconcileIAMRoles reconciles IAM roles with DynamoDB metadata.
func (m *Manager) reconcileIAMRoles(
	ctx context.Context,
	reqLogger *slog.Logger,
) (health.IAMHealthStatus, []health.HealthIssue, error) {
	status := health.IAMHealthStatus{
		MissingRoles: []string{},
	}
	issues := []health.HealthIssue{}

	// Verify default roles
	if m.cfg.DefaultTaskRoleARN != "" {
		roleName := extractRoleNameFromARN(m.cfg.DefaultTaskRoleARN)
		_, err := m.iamClient.GetRole(ctx, &iam.GetRoleInput{
			RoleName: awsStd.String(roleName),
		})
		if err != nil {
			var noSuchEntity *iamTypes.NoSuchEntityException
			if err != nil && strings.Contains(err.Error(), "NoSuchEntity") {
				status.MissingRoles = append(status.MissingRoles, m.cfg.DefaultTaskRoleARN)
				issues = append(issues, health.HealthIssue{
					ResourceType: "iam_role",
					ResourceID:   m.cfg.DefaultTaskRoleARN,
					Severity:     "error",
					Message:      "Default task role missing (managed by CloudFormation)",
					Action:       "requires_manual_intervention",
				})
			} else {
				issues = append(issues, health.HealthIssue{
					ResourceType: "iam_role",
					ResourceID:   m.cfg.DefaultTaskRoleARN,
					Severity:     "error",
					Message:      fmt.Sprintf("Failed to verify default task role: %v", err),
					Action:       "reported",
				})
			}
		}
	}

	if m.cfg.DefaultTaskExecRoleARN != "" {
		roleName := extractRoleNameFromARN(m.cfg.DefaultTaskExecRoleARN)
		_, err := m.iamClient.GetRole(ctx, &iam.GetRoleInput{
			RoleName: awsStd.String(roleName),
		})
		if err != nil {
			var noSuchEntity *iamTypes.NoSuchEntityException
			if err != nil && strings.Contains(err.Error(), "NoSuchEntity") {
				status.MissingRoles = append(status.MissingRoles, m.cfg.DefaultTaskExecRoleARN)
				issues = append(issues, health.HealthIssue{
					ResourceType: "iam_role",
					ResourceID:   m.cfg.DefaultTaskExecRoleARN,
					Severity:     "error",
					Message:      "Default task execution role missing (managed by CloudFormation)",
					Action:       "requires_manual_intervention",
				})
			} else {
				issues = append(issues, health.HealthIssue{
					ResourceType: "iam_role",
					ResourceID:   m.cfg.DefaultTaskExecRoleARN,
					Severity:     "error",
					Message:      fmt.Sprintf("Failed to verify default task execution role: %v", err),
					Action:       "reported",
				})
			}
		}
	}

	if len(status.MissingRoles) == 0 {
		status.DefaultRolesVerified = true
	}

	// Verify custom roles referenced in images
	images, err := m.imageRepo.ListImages(ctx)
	if err != nil {
		return status, issues, fmt.Errorf("failed to list images: %w", err)
	}

	customRoles := make(map[string]bool)
	for _, img := range images {
		if img.TaskRoleName != nil && *img.TaskRoleName != "" {
			customRoles[*img.TaskRoleName] = true
		}
		if img.TaskExecutionRoleName != nil && *img.TaskExecutionRoleName != "" {
			customRoles[*img.TaskExecutionRoleName] = true
		}
	}

	status.CustomRolesTotal = len(customRoles)
	for roleName := range customRoles {
		roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", m.cfg.AccountID, roleName)
		_, err := m.iamClient.GetRole(ctx, &iam.GetRoleInput{
			RoleName: awsStd.String(roleName),
		})
		if err != nil {
			if err != nil && strings.Contains(err.Error(), "NoSuchEntity") {
				status.MissingRoles = append(status.MissingRoles, roleARN)
				issues = append(issues, health.HealthIssue{
					ResourceType: "iam_role",
					ResourceID:   roleARN,
					Severity:     "error",
					Message:      fmt.Sprintf("Custom IAM role missing (cannot recreate without policies)"),
					Action:       "requires_manual_intervention",
				})
			} else {
				issues = append(issues, health.HealthIssue{
					ResourceType: "iam_role",
					ResourceID:   roleARN,
					Severity:     "error",
					Message:      fmt.Sprintf("Failed to verify custom role: %v", err),
					Action:       "reported",
				})
			}
		} else {
			status.CustomRolesVerified++
		}
	}

	return status, issues, nil
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
	family string,
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
	expectedTags := awsOrchestrator.BuildTaskDefinitionTags(image, isDefaultPtr)

	// Check if tags match
	tagsMatch := m.compareTags(tagsOutput.Tags, expectedTags)
	if tagsMatch {
		return false, nil
	}

	// Tags don't match - update them
	err = awsOrchestrator.UpdateTaskDefinitionTags(ctx, m.ecsClient, taskDefARN, image, isDefault, reqLogger)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (m *Manager) compareTags(currentTags []ecsTypes.Tag, expectedTags []ecsTypes.Tag) bool {
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
	reqLogger *slog.Logger,
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
	reqLogger *slog.Logger,
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
			MaxResults: awsStd.Int32(50),
		})
		if err != nil {
			return orphaned, fmt.Errorf("failed to describe parameters: %w", err)
		}

		for _, param := range listOutput.Parameters {
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
