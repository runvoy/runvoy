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
	"runvoy/internal/auth/authorization"
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
	ecsClient      awsClient.ECSClient
	ssmClient      secrets.Client
	iamClient      awsClient.IAMClient
	imageRepo      ImageTaskDefRepository
	secretsRepo    database.SecretsRepository
	userRepo       database.UserRepository
	executionRepo  database.ExecutionRepository
	enforcer       *authorization.Enforcer
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
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	enforcer *authorization.Enforcer,
	cfg *Config,
	log *slog.Logger,
) *Manager {
	return &Manager{
		ecsClient:     ecsClient,
		ssmClient:     ssmClient,
		iamClient:     iamClient,
		imageRepo:     imageRepo,
		secretsRepo:   secretsRepo,
		userRepo:      userRepo,
		executionRepo: executionRepo,
		enforcer:      enforcer,
		cfg:           cfg,
		secretsPrefix: cfg.SecretsPrefix,
		logger:        log,
	}
}

// SetCasbinDependencies sets the Casbin-related dependencies for the health manager.
// This allows the enforcer to be set after initialization when it becomes available.
func (m *Manager) SetCasbinDependencies(
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	enforcer *authorization.Enforcer,
) {
	m.userRepo = userRepo
	m.executionRepo = executionRepo
	m.enforcer = enforcer
}

// Reconcile performs health checks and reconciliation for ECS task definitions, SSM parameters, and IAM roles.
func (m *Manager) Reconcile(ctx context.Context) (*health.Report, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)
	reqLogger.Info("starting health reconciliation")

	report := &health.Report{
		Timestamp: time.Now(),
		Issues:    []health.Issue{},
	}

	res, err := m.runAllReconciliations(ctx, reqLogger)
	if err != nil {
		return nil, err
	}

	report.ComputeStatus = res.computeStatus
	report.Issues = append(report.Issues, res.computeIssues...)
	report.ReconciledCount += res.computeStatus.RecreatedCount + res.computeStatus.TagUpdatedCount

	report.SecretsStatus = res.secretsStatus
	report.Issues = append(report.Issues, res.secretsIssues...)
	report.ReconciledCount += res.secretsStatus.TagUpdatedCount

	report.IdentityStatus = res.identityStatus
	report.Issues = append(report.Issues, res.identityIssues...)

	report.CasbinStatus = res.casbinStatus
	report.Issues = append(report.Issues, res.casbinIssues...)

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

// reconciliationResults groups the results of all reconciliation tasks.
type reconciliationResults struct {
	computeStatus  health.ComputeHealthStatus
	computeIssues  []health.Issue
	secretsStatus  health.SecretsHealthStatus
	secretsIssues  []health.Issue
	identityStatus health.IdentityHealthStatus
	identityIssues []health.Issue
	casbinStatus   health.CasbinHealthStatus
	casbinIssues   []health.Issue
}

// runAllReconciliations executes compute, secrets, and identity reconciliations in parallel.
func (m *Manager) runAllReconciliations(
	ctx context.Context,
	reqLogger *slog.Logger,
) (reconciliationResults, error) {
	var computeStatus health.ComputeHealthStatus
	var computeIssues []health.Issue
	var secretsStatus health.SecretsHealthStatus
	var secretsIssues []health.Issue
	var identityStatus health.IdentityHealthStatus
	var identityIssues []health.Issue
	var casbinStatus health.CasbinHealthStatus
	var casbinIssues []health.Issue
	var mu sync.Mutex
	g, gCtx := errgroup.WithContext(ctx)
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
	g.Go(func() error {
		status, issues, err := m.reconcileCasbin(gCtx, reqLogger)
		if err != nil {
			return fmt.Errorf("failed to reconcile Casbin: %w", err)
		}
		mu.Lock()
		casbinStatus = status
		casbinIssues = issues
		mu.Unlock()
		return nil
	})
	if err := g.Wait(); err != nil {
		return reconciliationResults{}, err
	}
	return reconciliationResults{
		computeStatus:  computeStatus,
		computeIssues:  computeIssues,
		secretsStatus:  secretsStatus,
		secretsIssues:  secretsIssues,
		identityStatus: identityStatus,
		identityIssues: identityIssues,
		casbinStatus:   casbinStatus,
		casbinIssues:  casbinIssues,
	}, nil
}
