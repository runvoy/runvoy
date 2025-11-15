// Package health provides health management functionality for runvoy.
// It defines the interface for reconciling resources between DynamoDB and AWS services.
package health

import (
	"context"
	"time"
)

// HealthManager defines the interface for health checks and resource reconciliation.
// Different cloud providers can implement this interface to support their specific infrastructure.
type HealthManager interface { //nolint:revive // HealthManager is the established name for this interface
	// Reconcile checks and repairs inconsistencies between DynamoDB metadata and actual AWS resources.
	// It verifies ECS task definitions, SSM parameters (secrets), and IAM roles.
	// Returns a comprehensive health report with all issues found and actions taken.
	Reconcile(ctx context.Context) (*HealthReport, error)
}

// HealthReport contains the results of a health reconciliation run.
type HealthReport struct { //nolint:revive // HealthReport is the established name for this type
	Timestamp       time.Time
	ECSStatus       ECSHealthStatus
	SecretsStatus   SecretsHealthStatus
	IAMStatus       IAMHealthStatus
	Issues          []HealthIssue
	ReconciledCount int
	ErrorCount      int
}

// ECSHealthStatus contains the health status for ECS task definitions.
type ECSHealthStatus struct {
	TotalImages      int
	VerifiedCount    int
	RecreatedCount   int
	TagUpdatedCount  int
	OrphanedCount    int
	OrphanedFamilies []string
}

// SecretsHealthStatus contains the health status for SSM parameters (secrets).
type SecretsHealthStatus struct {
	TotalSecrets       int
	VerifiedCount      int
	TagUpdatedCount    int
	MissingCount       int
	OrphanedCount      int
	OrphanedParameters []string
}

// IAMHealthStatus contains the health status for IAM roles.
type IAMHealthStatus struct {
	DefaultRolesVerified bool
	CustomRolesVerified  int
	CustomRolesTotal     int
	MissingRoles         []string
}

// HealthIssue represents a single health issue found during reconciliation.
type HealthIssue struct { //nolint:revive // HealthIssue is the established name for this type
	ResourceType string // "ecs_task_definition", "ssm_parameter", "iam_role"
	ResourceID   string
	Severity     string // "error", "warning"
	Message      string
	Action       string // "recreated", "requires_manual_intervention", "reported", "tag_updated"
}
