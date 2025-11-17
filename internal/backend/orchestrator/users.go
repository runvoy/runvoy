package orchestrator

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
)

// validateCreateUserRequest validates the email and role in the create user request.
func (s *Service) validateCreateUserRequest(ctx context.Context, email, role string) error {
	if email == "" {
		return apperrors.ErrBadRequest("email is required", nil)
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return apperrors.ErrBadRequest("invalid email address", err)
	}

	if role == "" {
		return apperrors.ErrBadRequest("role is required", nil)
	}

	if !authorization.IsValidRole(role) {
		validRoles := strings.Join(authorization.ValidRoles(), ", ")
		return apperrors.ErrBadRequest("invalid role, must be one of: "+validRoles, nil)
	}

	existingUser, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return err
	}

	if existingUser != nil {
		return apperrors.ErrConflict("user with this email already exists", nil)
	}

	return nil
}

// generateOrUseAPIKey generates a new API key if none is provided.
func generateOrUseAPIKey(providedKey string) (string, error) {
	if providedKey != "" {
		return providedKey, nil
	}
	apiKey, err := auth.GenerateSecretToken()
	if err != nil {
		return "", apperrors.ErrInternalError("failed to generate API key", err)
	}
	return apiKey, nil
}

// createPendingClaim creates a pending API key claim record.
func (s *Service) createPendingClaim(
	ctx context.Context, apiKey, email, createdByEmail string, expiresAt int64,
) (string, error) {
	secretToken, err := auth.GenerateSecretToken()
	if err != nil {
		return "", apperrors.ErrInternalError("failed to generate secret token", err)
	}

	pending := &api.PendingAPIKey{
		SecretToken: secretToken,
		APIKey:      apiKey,
		UserEmail:   email,
		CreatedBy:   createdByEmail,
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   expiresAt,
		Viewed:      false,
	}

	if err = s.userRepo.CreatePendingAPIKey(ctx, pending); err != nil {
		return "", apperrors.ErrDatabaseError("failed to create pending API key", err)
	}

	return secretToken, nil
}

// CreateUser creates a new user with an API key and returns a claim token.
// If no API key is provided in the request, one will be generated.
// Requires a valid role to be specified in the request.
func (s *Service) CreateUser(
	ctx context.Context, req api.CreateUserRequest, createdByEmail string,
) (*api.CreateUserResponse, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	if err := s.validateCreateUserRequest(ctx, req.Email, req.Role); err != nil {
		return nil, err
	}

	apiKey, err := generateOrUseAPIKey(req.APIKey)
	if err != nil {
		return nil, err
	}

	apiKeyHash := auth.HashAPIKey(apiKey)

	user := &api.User{
		Email:     req.Email,
		Role:      req.Role,
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
	}

	expiresAt := time.Now().Add(constants.ClaimURLExpirationMinutes * time.Minute).Unix()

	if err = s.userRepo.CreateUser(ctx, user, apiKeyHash, expiresAt); err != nil {
		return nil, apperrors.ErrDatabaseError("failed to create user", err)
	}

	if syncErr := s.syncUserRoleAfterCreate(ctx, req.Email, req.Role); syncErr != nil {
		return nil, syncErr
	}

	secretToken, err := s.createPendingClaim(ctx, apiKey, req.Email, createdByEmail, expiresAt)
	if err != nil {
		if removeErr := s.removeRoleForUserFromEnforcer(req.Email, req.Role); removeErr != nil {
			s.Logger.Error("failed to remove user role after pending claim failure", "context", map[string]string{
				"user":  req.Email,
				"role":  req.Role,
				"error": removeErr.Error(),
			})
		}
		if revokeErr := s.userRepo.RevokeUser(ctx, req.Email); revokeErr != nil {
			s.Logger.Error("failed to revoke user after pending claim failure", "context", map[string]string{
				"user":  req.Email,
				"error": revokeErr.Error(),
			})
		}
		return nil, err
	}

	return &api.CreateUserResponse{
		User:       user,
		ClaimToken: secretToken,
	}, nil
}

// ClaimAPIKey retrieves and claims a pending API key by its secret token.
func (s *Service) ClaimAPIKey(
	ctx context.Context,
	secretToken string,
	ipAddress string,
) (*api.ClaimAPIKeyResponse, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	// Retrieve pending key
	pending, err := s.userRepo.GetPendingAPIKey(ctx, secretToken)
	if err != nil {
		return nil, apperrors.ErrDatabaseError("failed to retrieve pending key", err)
	}

	if pending == nil {
		return nil, apperrors.ErrNotFound("invalid or expired token", nil)
	}

	// Check if already viewed
	if pending.Viewed {
		return nil, apperrors.ErrConflict("key has already been claimed", nil)
	}

	// Check if expired
	now := time.Now().Unix()
	if pending.ExpiresAt < now {
		return nil, apperrors.ErrNotFound("token has expired", nil)
	}

	// Mark as viewed atomically
	if markErr := s.userRepo.MarkAsViewed(ctx, secretToken, ipAddress); markErr != nil {
		return nil, markErr
	}

	// Remove expiration from user record (make user permanent)
	if removeErr := s.userRepo.RemoveExpiration(ctx, pending.UserEmail); removeErr != nil {
		// Log error but don't fail the claim - user already exists and can authenticate
		s.Logger.Error("failed to remove expiration from user record", "error", removeErr, "email", pending.UserEmail)
	}

	return &api.ClaimAPIKeyResponse{
		APIKey:    pending.APIKey,
		UserEmail: pending.UserEmail,
		Message:   "API key claimed successfully",
	}, nil
}

// AuthenticateUser verifies an API key and returns the associated user.
// Returns appropriate errors for invalid API keys, revoked keys, or server errors.
func (s *Service) AuthenticateUser(ctx context.Context, apiKey string) (*api.User, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	if apiKey == "" {
		return nil, apperrors.ErrBadRequest("API key is required", nil)
	}

	apiKeyHash := auth.HashAPIKey(apiKey)

	user, err := s.userRepo.GetUserByAPIKeyHash(ctx, apiKeyHash)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, apperrors.ErrInvalidAPIKey(nil)
	}

	if user.Revoked {
		return nil, apperrors.ErrAPIKeyRevoked(nil)
	}

	return user, nil
}

// UpdateUserLastUsed updates the user's last_used timestamp after successful authentication.
// This is a best-effort operation; callers may choose to log failures without failing the request.
func (s *Service) UpdateUserLastUsed(ctx context.Context, email string) (*time.Time, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}
	if email == "" {
		return nil, apperrors.ErrBadRequest("email is required", nil)
	}
	return s.userRepo.UpdateLastUsed(ctx, email)
}

// RevokeUser marks a user's API key as revoked.
// Returns an error if the user does not exist or revocation fails.
func (s *Service) RevokeUser(ctx context.Context, email string) error {
	if s.userRepo == nil {
		return apperrors.ErrInternalError("user repository not configured", nil)
	}

	if email == "" {
		return apperrors.ErrBadRequest("email is required", nil)
	}

	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		// Propagate database errors as-is
		return err
	}
	if user == nil {
		return apperrors.ErrNotFound("user not found", nil)
	}

	// Only remove role from enforcer if the user has a role
	if user.Role != "" {
		if removeErr := s.removeRoleForUserFromEnforcer(email, user.Role); removeErr != nil {
			return apperrors.ErrInternalError("failed to remove user role from authorization enforcer", removeErr)
		}
	}

	if revokeErr := s.userRepo.RevokeUser(ctx, email); revokeErr != nil {
		// Attempt to restore the role to avoid leaving the enforcer without the user mapping.
		if restoreErr := s.addRoleForUserToEnforcer(email, user.Role); restoreErr != nil {
			s.Logger.Error("failed to restore user role after revoke failure", "context", map[string]string{
				"user":          email,
				"restore_error": restoreErr.Error(),
				"revoke_error":  revokeErr.Error(),
				"original_role": user.Role,
			})
		}
		return revokeErr
	}

	return nil
}

// ListUsers returns all users in the system sorted by email (excluding API key hashes for security).
// Returns an error if the user repository is not configured or if the query fails.
// Sorting is delegated to the repository implementation (e.g., DynamoDB GSI).
func (s *Service) ListUsers(ctx context.Context) (*api.ListUsersResponse, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	users, err := s.userRepo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	return &api.ListUsersResponse{
		Users: users,
	}, nil
}

func (s *Service) syncUserRoleAfterCreate(ctx context.Context, email, role string) error {
	if err := s.addRoleForUserToEnforcer(email, role); err != nil {
		if revokeErr := s.userRepo.RevokeUser(ctx, email); revokeErr != nil {
			s.Logger.Error("failed to revoke user after enforcer sync failure", "context", map[string]string{
				"user":  email,
				"error": revokeErr.Error(),
			})
		}
		return apperrors.ErrInternalError("failed to synchronize user role with authorization enforcer", err)
	}

	return nil
}

func (s *Service) addRoleForUserToEnforcer(email, roleStr string) error {
	role, err := authorization.NewRole(roleStr)
	if err != nil {
		return fmt.Errorf("invalid role %q for user %s: %w", roleStr, email, err)
	}

	if addErr := s.enforcer.AddRoleForUser(email, role); addErr != nil {
		return fmt.Errorf("failed to add role %q for user %s: %w", roleStr, email, addErr)
	}

	return nil
}

func (s *Service) removeRoleForUserFromEnforcer(email, roleStr string) error {
	role, err := authorization.NewRole(roleStr)
	if err != nil {
		return fmt.Errorf("invalid role %q for user %s: %w", roleStr, email, err)
	}

	formattedRole := authorization.FormatRole(role)
	if removeErr := s.enforcer.RemoveRoleForUser(email, formattedRole); removeErr != nil {
		return fmt.Errorf("failed to remove role %q for user %s: %w", roleStr, email, removeErr)
	}

	return nil
}
