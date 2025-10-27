package provider

import (
	"context"
	"io"

	internalConfig "mycli/internal/config"
)

// Provider defines the interface for cloud provider implementations
type Provider interface {
	// InitializeInfrastructure deploys the complete infrastructure stack
	// Returns the infrastructure outputs including API endpoint and generated key
	InitializeInfrastructure(ctx context.Context, cfg *Config) (*InfrastructureOutput, error)

	// UpdateInfrastructure updates an existing infrastructure deployment
	UpdateInfrastructure(ctx context.Context, cfg *Config) error

	// DestroyInfrastructure removes all infrastructure resources
	DestroyInfrastructure(ctx context.Context, cfg *Config) error

	// GetEndpoint retrieves the API endpoint for the infrastructure
	GetEndpoint(ctx context.Context, cfg *Config) (string, error)

	// ValidateConfig validates provider-specific configuration
	ValidateConfig(cfg *Config) error

	// GetName returns the provider name (e.g., "aws", "gcp")
	GetName() string
}

// InfrastructureBuilder builds Lambda function code for deployment
type InfrastructureBuilder interface {
	// BuildLambda builds the Lambda function code
	BuildLambda() ([]byte, error)
}

// BucketUploader handles uploading artifacts to object storage
type BucketUploader interface {
	// UploadBytes uploads data to object storage
	UploadBytes(ctx context.Context, bucket, key string, data []byte) error

	// UploadReader uploads data from a reader to object storage
	UploadReader(ctx context.Context, bucket, key string, reader io.Reader, size int64) error

	// GetBucketName retrieves the bucket name from stack outputs
	GetBucketName(ctx context.Context, stackName string) (string, error)
}

// CredentialManager handles credential storage and retrieval
type CredentialManager interface {
	// StoreCredentials stores credentials in the provider's secure storage
	StoreCredentials(ctx context.Context, apiKey string, apiKeyHash string, outputs *InfrastructureOutput) error

	// LoadCredentials retrieves credentials from the provider's secure storage
	LoadCredentials(ctx context.Context, cfg *Config) (*internalConfig.Config, error)
}

// ProviderFactory creates provider instances
type ProviderFactory struct {
	providers map[string]func() (Provider, error)
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[string]func() (Provider, error)),
	}
}

// Register registers a provider with the factory
func (f *ProviderFactory) Register(name string, constructor func() (Provider, error)) {
	f.providers[name] = constructor
}

// CreateProvider creates a provider instance by name
func (f *ProviderFactory) CreateProvider(name string) (Provider, error) {
	constructor, ok := f.providers[name]
	if !ok {
		return nil, &ValidationError{
			Message: "unknown provider",
			Field:   "name",
		}
	}
	return constructor()
}

// ListProviders returns a list of available provider names
func (f *ProviderFactory) ListProviders() []string {
	names := make([]string, 0, len(f.providers))
	for name := range f.providers {
		names = append(names, name)
	}
	return names
}
