package constants

import "time"

const (
	// DefaultRegion is the default GCP region used when no region is specified.
	// GCP uses us-central1 as the default region for most services.
	DefaultRegion = "us-central1"

	// ProjectPollInterval is the interval at which to poll for project status changes.
	ProjectPollInterval = 5 * time.Second

	// ProjectOperationTimeout is the maximum time to wait for a project operation to complete.
	ProjectOperationTimeout = 5 * time.Minute

	// StatusDeleteRequested is the lifecycle state of a project after deletion is requested.
	StatusDeleteRequested = "DELETE_REQUESTED"
)
