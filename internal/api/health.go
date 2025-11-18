package api

// HealthReconcileResponse is returned by POST /api/v1/health/reconcile.
type HealthReconcileResponse struct {
	Status string                 `json:"status"`
	Report *HealthReconcileReport `json:"report"`
}

// HealthReconcileReport is the payload shape returned by the health reconciliation API.
type HealthReconcileReport struct {
	Timestamp       string                        `json:"timestamp"`
	ComputeStatus   HealthReconcileComputeStatus  `json:"compute_status"`
	SecretsStatus   HealthReconcileSecretsStatus  `json:"secrets_status"`
	IdentityStatus  HealthReconcileIdentityStatus `json:"identity_status"`
	Issues          []HealthReconcileIssue        `json:"issues"`
	ReconciledCount int                           `json:"reconciled_count"`
	ErrorCount      int                           `json:"error_count"`
}

// HealthReconcileComputeStatus summarizes compute reconciliation.
type HealthReconcileComputeStatus struct {
	TotalResources    int      `json:"total_resources"`
	VerifiedCount     int      `json:"verified_count"`
	RecreatedCount    int      `json:"recreated_count"`
	TagUpdatedCount   int      `json:"tag_updated_count"`
	OrphanedCount     int      `json:"orphaned_count"`
	OrphanedResources []string `json:"orphaned_resources"`
}

// HealthReconcileSecretsStatus summarizes secrets reconciliation.
type HealthReconcileSecretsStatus struct {
	TotalSecrets       int      `json:"total_secrets"`
	VerifiedCount      int      `json:"verified_count"`
	TagUpdatedCount    int      `json:"tag_updated_count"`
	MissingCount       int      `json:"missing_count"`
	OrphanedCount      int      `json:"orphaned_count"`
	OrphanedParameters []string `json:"orphaned_parameters"`
}

// HealthReconcileIdentityStatus summarizes identity (roles) reconciliation.
type HealthReconcileIdentityStatus struct {
	DefaultRolesVerified bool     `json:"default_roles_verified"`
	CustomRolesVerified  int      `json:"custom_roles_verified"`
	CustomRolesTotal     int      `json:"custom_roles_total"`
	MissingRoles         []string `json:"missing_roles"`
}

// HealthReconcileIssue describes a single issue encountered during reconciliation.
type HealthReconcileIssue struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	Action       string `json:"action"`
}
