// Package health provides health management functionality for runvoy.
// It defines the interface for reconciling resources between metadata storage and cloud provider services.
package health

import (
	"context"
	"time"
)

// Manager defines the interface for health checks and resource reconciliation.
// Different cloud providers can implement this interface to support their specific infrastructure.
type Manager interface {
	// Reconcile checks and repairs inconsistencies between metadata storage and actual cloud resources.
	// It verifies compute resources (e.g., task definitions, containers), secrets, and identity/access resources.
	// Returns a comprehensive health report with all issues found and actions taken.
	Reconcile(ctx context.Context) (*Report, error)
}

// Report contains the results of a health reconciliation run.
type Report struct {
	Timestamp       time.Time
	ComputeStatus   ComputeHealthStatus
	SecretsStatus   SecretsHealthStatus
	IdentityStatus  IdentityHealthStatus
	CasbinStatus    CasbinHealthStatus
	Issues          []Issue
	ReconciledCount int
	ErrorCount      int
}

// ComputeHealthStatus contains the health status for compute resources (e.g., containers, task definitions).
type ComputeHealthStatus struct {
	TotalResources    int
	VerifiedCount     int
	RecreatedCount    int
	TagUpdatedCount   int
	OrphanedCount     int
	OrphanedResources []string
}

// SecretsHealthStatus contains the health status for secrets/parameters.
type SecretsHealthStatus struct {
	TotalSecrets       int
	VerifiedCount      int
	TagUpdatedCount    int
	MissingCount       int
	OrphanedCount      int
	OrphanedParameters []string
}

// IdentityHealthStatus contains the health status for identity and access management resources.
type IdentityHealthStatus struct {
	DefaultRolesVerified bool
	CustomRolesVerified  int
	CustomRolesTotal     int
	MissingRoles         []string
}

// CasbinHealthStatus contains the health status for Casbin authorization data.
type CasbinHealthStatus struct {
	UsersWithInvalidRoles      []string
	UsersWithMissingRoles      []string
	ResourcesWithMissingOwners []string
	OrphanedOwnerships         []string
	MissingOwnerships          []string
	TotalUsersChecked          int
	TotalResourcesChecked      int
}

// Issue represents a single health issue found during reconciliation.
type Issue struct {
	ResourceType string // Provider-specific resource type (e.g., "ecs_task_definition", "cloud_run_service")
	ResourceID   string
	Severity     string // "error", "warning"
	Message      string
	Action       string // "recreated", "requires_manual_intervention", "reported", "tag_updated"
}
