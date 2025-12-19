package core

import "context"

const (
	OperationTypeCreate = "CREATE"
	OperationTypeUpdate = "UPDATE"

	StatusInProgress     = "IN_PROGRESS"
	StatusNotFound       = "NOT_FOUND"
	StatusUpdateComplete = "UPDATE_COMPLETE"
	StatusCreateComplete = "CREATE_COMPLETE"
)

// DeployOptions contains all options for deploying infrastructure.
type DeployOptions struct {
	Name       string   // Project/stack name (provider-specific: GCP project ID, AWS stack name)
	Template   string   // URL, S3 URI, or local file path (AWS only)
	Version    string   // Release version
	Parameters []string // KEY=VALUE format
	Wait       bool     // Wait for completion
	Region     string   // Provider region (optional)
	OrgID      string   // Organization ID for GCP (optional)
}

// DeployResult contains the result of a deployment operation.
type DeployResult struct {
	Name          string // Project/stack name
	OperationType string // "CREATE" or "UPDATE"
	Status        string
	Outputs       map[string]string
	NoChanges     bool // True if project/stack was already up to date
}

// DestroyOptions contains all options for destroying infrastructure.
type DestroyOptions struct {
	Name   string // Project/stack name
	Wait   bool   // Wait for completion
	Region string // Provider region (optional)
}

// DestroyResult contains the result of a destroy operation.
type DestroyResult struct {
	Name     string // Project/stack name
	Status   string
	NotFound bool // True if project/stack was already deleted
}

// TemplateSource represents the resolved template source.
type TemplateSource struct {
	URL  string // For remote templates (S3/HTTPS)
	Body string // For local file templates
}

// Deployer defines the interface for infrastructure deployment.
// Different cloud providers implement this interface.
type Deployer interface {
	// Deploy deploys or updates infrastructure
	Deploy(ctx context.Context, opts *DeployOptions) (*DeployResult, error)
	// Destroy destroys infrastructure
	Destroy(ctx context.Context, opts *DestroyOptions) (*DestroyResult, error)
	// CheckExists checks if the infrastructure project/stack exists
	CheckExists(ctx context.Context, name string) (bool, error)
	// GetOutputs retrieves outputs from a deployed project/stack
	GetOutputs(ctx context.Context, name string) (map[string]string, error)
	// GetRegion returns the region being used
	GetRegion() string
}
