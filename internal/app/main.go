package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/database"
)

type Service struct {
	userRepo database.UserRepository
	Logger   *slog.Logger
}

// NewService creates a new service instance.
// If userRepo is nil, user-related operations will not be available.
// This allows the service to work without database dependencies for simple operations.
func NewService(userRepo database.UserRepository, logger *slog.Logger) *Service {
	return &Service{
		userRepo: userRepo,
		Logger:   logger,
	}
}

// CreateUser creates a new user with an API key.
// If no API key is provided in the request, one will be generated.
// The API key is only returned in the response and should be stored by the client.
func (s *Service) CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error) {
	if s.userRepo == nil {
		return nil, errors.New("user repository not configured")
	}

	// Validate email
	if req.Email == "" {
		return nil, errors.New("email is required")
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, fmt.Errorf("invalid email address: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, errors.New("user with this email already exists")
	}

	// Generate or use provided API key
	apiKey := req.APIKey
	if apiKey == "" {
		apiKey, err = generateAPIKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate API key: %w", err)
		}
	}

	// Hash the API key for storage
	apiKeyHash := hashAPIKey(apiKey)

	// Create user object
	user := &api.User{
		Email:     req.Email,
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
	}

	// Store in database
	if err := s.userRepo.CreateUser(ctx, user, apiKeyHash); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &api.CreateUserResponse{
		User:   user,
		APIKey: apiKey, // Return plain API key (only time it's available!)
	}, nil
}

// AuthenticateUser verifies an API key and returns the associated user.
// Returns nil if the API key is invalid or the user is revoked.
func (s *Service) AuthenticateUser(ctx context.Context, apiKey string) (*api.User, error) {
	if s.userRepo == nil {
		return nil, errors.New("user repository not configured")
	}

	if apiKey == "" {
		return nil, errors.New("API key is required")
	}

	apiKeyHash := hashAPIKey(apiKey)

	user, err := s.userRepo.GetUserByAPIKeyHash(ctx, apiKeyHash)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	if user == nil {
		return nil, errors.New("invalid API key")
	}

	if user.Revoked {
		return nil, errors.New("API key has been revoked")
	}

	return user, nil
}

// RevokeUser marks a user's API key as revoked.
// Returns an error if the user does not exist or revocation fails.
func (s *Service) RevokeUser(ctx context.Context, email string) error {
	if s.userRepo == nil {
		return errors.New("user repository not configured")
	}

	if email == "" {
		return errors.New("email is required")
	}

	// Check if user exists
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	// Revoke the user
	if err := s.userRepo.RevokeUser(ctx, email); err != nil {
		return fmt.Errorf("failed to revoke user: %w", err)
	}

	return nil
}

// generateAPIKey creates a cryptographically secure random API key.
// The key is base64-encoded and approximately 32 characters long.
func generateAPIKey() (string, error) {
	// Generate 24 random bytes (will be ~32 chars when base64 encoded)
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	// Encode to base64 URL-safe format (no padding)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// hashAPIKey creates a SHA-256 hash of the API key for secure storage.
// We never store plain API keys in the database.
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return base64.StdEncoding.EncodeToString(hash[:])
}
