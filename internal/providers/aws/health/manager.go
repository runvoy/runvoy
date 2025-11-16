// Package health provides AWS-specific health management implementation for runvoy.
// It reconciles resources between DynamoDB metadata and actual AWS services.
package health

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
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

	// Variables to hold results from parallel reconciliation tasks
	var (
		computeStatus   health.ComputeStatus
		computeIssues   []health.Issue
		secretsStatus   health.SecretsStatus
		secretsIssues   []health.Issue
		identityStatus  health.IdentityStatus
		identityIssues  []health.Issue
		mu              sync.Mutex
	)

	// Run reconciliation tasks in parallel using errgroup
	g, gCtx := errgroup.WithContext(ctx)

	// Reconcile ECS task definitions
	g.Go(func() error {
		status, issues, err := m.reconcileECSTaskDefinitions(gCtx, reqLogger)
		if err != nil {
			return fmt.Errorf("failed to reconcile ECS task definitions: %w", err)
		}
		mu.Lock()
		computeStatus = status
		computeIssues = issues
		mu.Unlock()
		return nil
	})

	// Reconcile secrets
	g.Go(func() error {
		status, issues, err := m.reconcileSecrets(gCtx, reqLogger)
		if err != nil {
			return fmt.Errorf("failed to reconcile secrets: %w", err)
		}
		mu.Lock()
		secretsStatus = status
		secretsIssues = issues
		mu.Unlock()
		return nil
	})

	// Reconcile IAM roles
	g.Go(func() error {
		status, issues, err := m.reconcileIAMRoles(gCtx, reqLogger)
		if err != nil {
			return fmt.Errorf("failed to reconcile IAM roles: %w", err)
		}
		mu.Lock()
		identityStatus = status
		identityIssues = issues
		mu.Unlock()
		return nil
	})

	// Wait for all reconciliation tasks to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Aggregate results into the report
	report.ComputeStatus = computeStatus
	report.Issues = append(report.Issues, computeIssues...)
	report.ReconciledCount += computeStatus.RecreatedCount + computeStatus.TagUpdatedCount

	report.SecretsStatus = secretsStatus
	report.Issues = append(report.Issues, secretsIssues...)
	report.ReconciledCount += secretsStatus.TagUpdatedCount

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
