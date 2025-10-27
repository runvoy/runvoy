package provider

import (
	"time"
)

// InfrastructureOutput contains information about the deployed infrastructure
type InfrastructureOutput struct {
	APIEndpoint  string
	APIKey       string
	Region       string
	StackPrefix  string // Stack prefix that identifies the deployment (implementation detail: provider may create multiple stacks)
	APIKeysTable string
	CreatedAt    time.Time
}

// Config holds provider configuration
type Config struct {
	StackPrefix string // Stack prefix to operate on (provider-specific)
	Region      string
	Force       bool
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
	Field   string
}

func (e *ValidationError) Error() string {
	return e.Message
}
