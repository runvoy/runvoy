package constants

import "strings"

// BackendProvider represents the backend infrastructure provider.
type BackendProvider string

const (
	// AWS is the Amazon Web Services backend provider.
	AWS BackendProvider = "AWS"
	// GCP is the Google Cloud Platform backend provider.
	GCP BackendProvider = "GCP"
)

// Environment represents the execution environment (e.g., CLI, Lambda).
type Environment string

// Environment types for logger configuration.
const (
	Development Environment = "development"
	Production  Environment = "production"
	CLI         Environment = "cli"
)

// Service represents a runvoy service component.
type Service string

const (
	// OrchestratorService is the main orchestrator service.
	OrchestratorService Service = "orchestrator"
	// EventProcessorService is the event processing service.
	EventProcessorService Service = "event-processor"
)

// Providers is a list of valid backend providers.
var Providers = []BackendProvider{
	AWS, GCP,
}

// ProvidersString returns a comma-separated string of the valid backend providers.
func ProvidersString() string {
	providers := make([]string, len(Providers))
	for i, provider := range Providers {
		providers[i] = strings.ToLower(string(provider))
	}

	return strings.Join(providers, ", ")
}
