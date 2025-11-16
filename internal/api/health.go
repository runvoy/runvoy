package api

// HealthReconcileResponse is returned by POST /api/v1/health/reconcile.
type HealthReconcileResponse struct {
	Status string                 `json:"status"`
	Report *HealthReconcileReport `json:"report"`
}

// HealthReconcileReport is the payload shape returned by the health reconciliation API.
type HealthReconcileReport struct {
	Timestamp       string                        `json:"timestamp"`
	ComputeStatus   HealthReconcileComputeStatus  `json:"computeStatus"`
	SecretsStatus   HealthReconcileSecretsStatus  `json:"secretsStatus"`
	IdentityStatus  HealthReconcileIdentityStatus `json:"identityStatus"`
	Issues          []HealthReconcileIssue        `json:"issues"`
	ReconciledCount int                           `json:"reconciledCount"`
	ErrorCount      int                           `json:"errorCount"`
}

// HealthReconcileComputeStatus summarizes compute reconciliation.
type HealthReconcileComputeStatus struct {
	TotalResources    int      `json:"totalResources"`
	VerifiedCount     int      `json:"verifiedCount"`
	RecreatedCount    int      `json:"recreatedCount"`
	TagUpdatedCount   int      `json:"tagUpdatedCount"`
	OrphanedCount     int      `json:"orphanedCount"`
	OrphanedResources []string `json:"orphanedResources"`
}

// HealthReconcileSecretsStatus summarizes secrets reconciliation.
type HealthReconcileSecretsStatus struct {
	TotalSecrets       int      `json:"totalSecrets"`
	VerifiedCount      int      `json:"verifiedCount"`
	TagUpdatedCount    int      `json:"tagUpdatedCount"`
	MissingCount       int      `json:"missingCount"`
	OrphanedCount      int      `json:"orphanedCount"`
	OrphanedParameters []string `json:"orphanedParameters"`
}

// HealthReconcileIdentityStatus summarizes identity (roles) reconciliation.
type HealthReconcileIdentityStatus struct {
	DefaultRolesVerified bool     `json:"defaultRolesVerified"`
	CustomRolesVerified  int      `json:"customRolesVerified"`
	CustomRolesTotal     int      `json:"customRolesTotal"`
	MissingRoles         []string `json:"missingRoles"`
}

// HealthReconcileIssue describes a single issue encountered during reconciliation.
type HealthReconcileIssue struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceID"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	Action       string `json:"action"`
}
