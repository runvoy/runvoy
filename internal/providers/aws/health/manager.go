// Package health provides AWS-specific health management implementation for runvoy.
// It reconciles resources between DynamoDB metadata and actual AWS services.
package health

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/backend/health"
	"runvoy/internal/database"
	"runvoy/internal/logger"
	awsClient "runvoy/internal/providers/aws/client"
	"runvoy/internal/providers/aws/secrets"
)

// ImageTaskDefRepository defines the interface for image-taskdef mapping operations.
type ImageTaskDefRepository interface {
	ListImages(ctx context.Context) ([]api.ImageInfo, error)
}

// Manager implements the health.Manager interface for AWS.
type Manager struct {
	ecsClient     awsClient.ECSClient
	ssmClient     secrets.Client
	iamClient     awsClient.IAMClient
	imageRepo     ImageTaskDefRepository
	secretsRepo   database.SecretsRepository
	cfg           *Config
	logger        *slog.Logger
	secretsPrefix string
}

// Config holds AWS-specific configuration for the health manager.
type Config struct {
	Region                 string
	AccountID              string
	DefaultTaskRoleARN     string
	DefaultTaskExecRoleARN string
	LogGroup               string
	SecretsPrefix          string
}

// Initialize creates a new AWS health manager.
func Initialize(
	ecsClient awsClient.ECSClient,
	ssmClient secrets.Client,
	iamClient awsClient.IAMClient,
	imageRepo ImageTaskDefRepository,
	secretsRepo database.SecretsRepository,
	cfg *Config,
	log *slog.Logger,
) *Manager {
	return &Manager{
		ecsClient:     ecsClient,
		ssmClient:     ssmClient,
		iamClient:     iamClient,
		imageRepo:     imageRepo,
		secretsRepo:   secretsRepo,
		cfg:           cfg,
		secretsPrefix: cfg.SecretsPrefix,
		logger:        log,
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

	computeStatus, computeIssues, err := m.reconcileECSTaskDefinitions(ctx, reqLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile ECS task definitions: %w", err)
	}
	report.ComputeStatus = computeStatus
	report.Issues = append(report.Issues, computeIssues...)
	report.ReconciledCount += computeStatus.RecreatedCount + computeStatus.TagUpdatedCount

	secretsStatus, secretsIssues, err := m.reconcileSecrets(ctx, reqLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile secrets: %w", err)
	}
	report.SecretsStatus = secretsStatus
	report.Issues = append(report.Issues, secretsIssues...)
	report.ReconciledCount += secretsStatus.TagUpdatedCount

	identityStatus, identityIssues, err := m.reconcileIAMRoles(ctx, reqLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile IAM roles: %w", err)
	}
	report.IdentityStatus = identityStatus
	report.Issues = append(report.Issues, identityIssues...)

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
