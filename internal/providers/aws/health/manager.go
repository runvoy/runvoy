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
	userRepo      database.UserRepository
	executionRepo database.ExecutionRepository
	enforcer      *authorization.Enforcer
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
func (m *Manager) Reconcile(ctx context.Context) (*api.HealthReport, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, m.logger)
	reqLogger.Info("starting health reconciliation")

	report := &api.HealthReport{
		Timestamp: time.Now(),
		Issues:    []api.HealthIssue{},
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

	report.AuthorizerStatus = res.casbinStatus
	report.Issues = append(report.Issues, res.casbinIssues...)

	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			report.ErrorCount++
		}
	}

	return report, nil
}

// reconciliationResults groups the results of all reconciliation tasks.
type reconciliationResults struct {
	computeStatus  api.ComputeHealthStatus
	computeIssues  []api.HealthIssue
	secretsStatus  api.SecretsHealthStatus
	secretsIssues  []api.HealthIssue
	identityStatus api.IdentityHealthStatus
	identityIssues []api.HealthIssue
	casbinStatus   api.AuthorizerHealthStatus
	casbinIssues   []api.HealthIssue
}

// runAllReconciliations executes compute, secrets, and identity reconciliations in parallel.
func (m *Manager) runAllReconciliations(
	ctx context.Context,
	reqLogger *slog.Logger,
) (reconciliationResults, error) {
	var mu sync.Mutex
	var res reconciliationResults
	g, gCtx := errgroup.WithContext(ctx)

	m.runComputeReconciliation(gCtx, g, reqLogger, &mu, &res)
	m.runSecretsReconciliation(gCtx, g, reqLogger, &mu, &res)
	m.runIdentityReconciliation(gCtx, g, reqLogger, &mu, &res)
	m.runCasbinReconciliation(gCtx, g, reqLogger, &mu, &res)

	if err := g.Wait(); err != nil {
		return reconciliationResults{}, err
	}
	return res, nil
}

func (m *Manager) runComputeReconciliation(
	ctx context.Context,
	g *errgroup.Group,
	reqLogger *slog.Logger,
	mu *sync.Mutex,
	res *reconciliationResults,
) {
	g.Go(func() error {
		status, issues, err := m.reconcileECSTaskDefinitions(ctx, reqLogger)
		if err != nil {
			return fmt.Errorf("failed to reconcile ECS task definitions: %w", err)
		}
		mu.Lock()
		res.computeStatus = status
		res.computeIssues = issues
		mu.Unlock()
		return nil
	})
}

func (m *Manager) runSecretsReconciliation(
	ctx context.Context,
	g *errgroup.Group,
	reqLogger *slog.Logger,
	mu *sync.Mutex,
	res *reconciliationResults,
) {
	g.Go(func() error {
		status, issues, err := m.reconcileSecrets(ctx, reqLogger)
		if err != nil {
			return fmt.Errorf("failed to reconcile secrets: %w", err)
		}
		mu.Lock()
		res.secretsStatus = status
		res.secretsIssues = issues
		mu.Unlock()
		return nil
	})
}

func (m *Manager) runIdentityReconciliation(
	ctx context.Context,
	g *errgroup.Group,
	reqLogger *slog.Logger,
	mu *sync.Mutex,
	res *reconciliationResults,
) {
	g.Go(func() error {
		status, issues, err := m.reconcileIAMRoles(ctx, reqLogger)
		if err != nil {
			return fmt.Errorf("failed to reconcile IAM roles: %w", err)
		}
		mu.Lock()
		res.identityStatus = status
		res.identityIssues = issues
		mu.Unlock()
		return nil
	})
}

func (m *Manager) runCasbinReconciliation(
	ctx context.Context,
	g *errgroup.Group,
	reqLogger *slog.Logger,
	mu *sync.Mutex,
	res *reconciliationResults,
) {
	g.Go(func() error {
		status, issues, err := m.reconcileCasbin(ctx, reqLogger)
		if err != nil {
			return fmt.Errorf("failed to reconcile Casbin: %w", err)
		}
		mu.Lock()
		res.casbinStatus = status
		res.casbinIssues = issues
		mu.Unlock()
		return nil
	})
}
