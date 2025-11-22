package api

import "time"

// HealthReconcileResponse is returned by POST /api/v1/health/reconcile.
type HealthReconcileResponse struct {
	Status string        `json:"status"`
	Report *HealthReport `json:"report"`
}

// HealthReport contains the results of a health reconciliation run.
type HealthReport struct {
	Timestamp        time.Time              `json:"timestamp"`
	ComputeStatus    ComputeHealthStatus    `json:"compute_status"`
	SecretsStatus    SecretsHealthStatus    `json:"secrets_status"`
	IdentityStatus   IdentityHealthStatus   `json:"identity_status"`
	AuthorizerStatus AuthorizerHealthStatus `json:"authorizer_status"`
	Issues           []HealthIssue          `json:"issues"`
	ReconciledCount  int                    `json:"reconciled_count"`
	ErrorCount       int                    `json:"error_count"`
}

// ComputeHealthStatus contains the health status for compute resources (e.g., containers, task definitions).
type ComputeHealthStatus struct {
	TotalResources    int      `json:"total_resources"`
	VerifiedCount     int      `json:"verified_count"`
	RecreatedCount    int      `json:"recreated_count"`
	TagUpdatedCount   int      `json:"tag_updated_count"`
	OrphanedCount     int      `json:"orphaned_count"`
	OrphanedResources []string `json:"orphaned_resources"`
}

// SecretsHealthStatus contains the health status for secrets/parameters.
type SecretsHealthStatus struct {
	TotalSecrets       int      `json:"total_secrets"`
	VerifiedCount      int      `json:"verified_count"`
	TagUpdatedCount    int      `json:"tag_updated_count"`
	MissingCount       int      `json:"missing_count"`
	OrphanedCount      int      `json:"orphaned_count"`
	OrphanedParameters []string `json:"orphaned_parameters"`
}

// IdentityHealthStatus contains the health status for identity and access management resources.
type IdentityHealthStatus struct {
	DefaultRolesVerified bool     `json:"default_roles_verified"`
	CustomRolesVerified  int      `json:"custom_roles_verified"`
	CustomRolesTotal     int      `json:"custom_roles_total"`
	MissingRoles         []string `json:"missing_roles"`
}

// AuthorizerHealthStatus contains the health status for authorization data.
type AuthorizerHealthStatus struct {
	UsersWithInvalidRoles      []string `json:"users_with_invalid_roles"`
	UsersWithMissingRoles      []string `json:"users_with_missing_roles"`
	ResourcesWithMissingOwners []string `json:"resources_with_missing_owners"`
	OrphanedOwnerships         []string `json:"orphaned_ownerships"`
	MissingOwnerships          []string `json:"missing_ownerships"`
	TotalUsersChecked          int      `json:"total_users_checked"`
	TotalResourcesChecked      int      `json:"total_resources_checked"`
}

// HealthIssue represents a single health issue found during reconciliation.
type HealthIssue struct {
	// ResourceType is provider-specific resource type (e.g., "ecs_task_definition", "cloud_run_service")
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Severity     string `json:"severity"` // "error", "warning"
	Message      string `json:"message"`
	Action       string `json:"action"` // "recreated", "requires_manual_intervention", "reported", "tag_updated"
}
