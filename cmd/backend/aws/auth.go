package aws

import (
	"context"
	"crypto/sha256"
	"fmt"

	"runvoy/internal/api"
	"runvoy/internal/services"
)

// AuthService implements AuthService using DynamoDB
type AuthService struct {
	storage services.StorageService
}

// NewAuthService creates a new auth service
func NewAuthService(storage services.StorageService) *AuthService {
	return &AuthService{
		storage: storage,
	}
}

// ValidateAPIKey validates an API key and returns the user
func (a *AuthService) ValidateAPIKey(ctx context.Context, apiKey string) (*api.User, error) {
	// Hash the API key for lookup
	hash := sha256.Sum256([]byte(apiKey))
	apiKeyHash := fmt.Sprintf("%x", hash)

	// Look up user by API key hash
	user, err := a.storage.GetUserByAPIKey(ctx, apiKeyHash)
	if err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	// Check if user is revoked
	if user.Revoked {
		return nil, fmt.Errorf("API key has been revoked")
	}

	// Update last used timestamp
	// TODO: Implement last used update
	// user.LastUsed = time.Now()
	// a.storage.UpdateUser(ctx, user)

	return user, nil
}

// GenerateAPIKey generates a new API key for a user
func (a *AuthService) GenerateAPIKey(ctx context.Context, email string) (string, error) {
	// TODO: Implement API key generation
	// This would generate a secure random API key and store it in DynamoDB
	return "", fmt.Errorf("not implemented")
}

// RevokeAPIKey revokes an API key for a user
func (a *AuthService) RevokeAPIKey(ctx context.Context, email string) error {
	// TODO: Implement API key revocation
	// This would mark the user as revoked in DynamoDB
	return fmt.Errorf("not implemented")
}