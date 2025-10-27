package provider

import (
	"time"
)

// InfrastructureOutput contains information about the deployed infrastructure
type InfrastructureOutput struct {
	APIEndpoint  string
	APIKey       string
	Region       string
	StackName    string
	APIKeysTable string
	CreatedAt    time.Time
}

// Config holds provider configuration
type Config struct {
	StackName string
	Region    string
	Force     bool
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
	Field   string
}

func (e *ValidationError) Error() string {
	return e.Message
}
