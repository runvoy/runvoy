package authorization

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"runvoy/internal/api"
)

// Mock repositories for testing

type mockUserRepository struct {
	users []*api.User
	err   error
}

func (m *mockUserRepository) CreateUser(ctx context.Context, user *api.User, apiKeyHash string, expiresAtUnix int64) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) RemoveExpiration(ctx context.Context, email string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) GetUserByAPIKeyHash(ctx context.Context, apiKeyHash string) (*api.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) UpdateLastUsed(ctx context.Context, email string) (*time.Time, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) RevokeUser(ctx context.Context, email string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) ListUsers(ctx context.Context) ([]*api.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.users, nil
}

func (m *mockUserRepository) CreatePendingAPIKey(ctx context.Context, pending *api.PendingAPIKey) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) GetPendingAPIKey(ctx context.Context, claimToken string) (*api.PendingAPIKey, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) DeletePendingAPIKey(ctx context.Context, claimToken string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) MarkAsViewed(ctx context.Context, email, viewer string) error {
	return errors.New("not implemented")
}

type mockExecutionRepository struct {
	executions []*api.Execution
	err        error
}

func (m *mockExecutionRepository) CreateExecution(ctx context.Context, execution *api.Execution) error {
	return errors.New("not implemented")
}

func (m *mockExecutionRepository) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	return nil, errors.New("not implemented")
}

func (m *mockExecutionRepository) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	return errors.New("not implemented")
}

func (m *mockExecutionRepository) ListExecutions(ctx context.Context, limit int, statuses []string) ([]*api.Execution, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.executions, nil
}

type mockSecretsRepository struct {
	secrets []*api.Secret
	err     error
}

func (m *mockSecretsRepository) CreateSecret(ctx context.Context, secret *api.Secret) error {
	return errors.New("not implemented")
}

func (m *mockSecretsRepository) GetSecret(ctx context.Context, name string, includeValue bool) (*api.Secret, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSecretsRepository) ListSecrets(ctx context.Context, includeValue bool) ([]*api.Secret, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.secrets, nil
}

func (m *mockSecretsRepository) UpdateSecret(ctx context.Context, secret *api.Secret) error {
	return errors.New("not implemented")
}

func (m *mockSecretsRepository) DeleteSecret(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

type mockImageRepository struct {
	images []api.ImageInfo
	err    error
}

func (m *mockImageRepository) ListImages(ctx context.Context) ([]api.ImageInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.images, nil
}

func TestHydrate(t *testing.T) {
	tests := []struct {
		name          string
		userRepo      *mockUserRepository
		executionRepo *mockExecutionRepository
		secretsRepo   *mockSecretsRepository
		imageRepo     ImageRepository
		wantError     bool
		errorMsg      string
	}{
		{
			name: "successful hydration with all resources",
			userRepo: &mockUserRepository{
				users: []*api.User{
					{Email: "admin@example.com", Role: "admin"},
					{Email: "dev@example.com", Role: "developer"},
				},
			},
			executionRepo: &mockExecutionRepository{
				executions: []*api.Execution{
					{ExecutionID: "exec-1", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com"}},
					{ExecutionID: "exec-2", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
				},
			},
			secretsRepo: &mockSecretsRepository{
				secrets: []*api.Secret{
					{Name: "db-password", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
					{Name: "api-key", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com"}},
				},
			},
			imageRepo: &mockImageRepository{
				images: []api.ImageInfo{
					{ImageID: "img-1", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
				},
			},
			wantError: false,
		},
		{
			name: "successful hydration with nil image repo",
			userRepo: &mockUserRepository{
				users: []*api.User{
					{Email: "admin@example.com", Role: "admin"},
				},
			},
			executionRepo: &mockExecutionRepository{
				executions: []*api.Execution{},
			},
			secretsRepo: &mockSecretsRepository{
				secrets: []*api.Secret{},
			},
			imageRepo: nil,
			wantError: false,
		},
		{
			name: "user repo error",
			userRepo: &mockUserRepository{
				err: errors.New("user repo error"),
			},
			executionRepo: &mockExecutionRepository{
				executions: []*api.Execution{},
			},
			secretsRepo: &mockSecretsRepository{
				secrets: []*api.Secret{},
			},
			imageRepo: nil,
			wantError: true,
			errorMsg:  "failed to load user roles",
		},
		{
			name: "secrets repo error",
			userRepo: &mockUserRepository{
				users: []*api.User{},
			},
			executionRepo: &mockExecutionRepository{
				executions: []*api.Execution{},
			},
			secretsRepo: &mockSecretsRepository{
				err: errors.New("secrets repo error"),
			},
			imageRepo: nil,
			wantError: true,
			errorMsg:  "failed to load resource ownerships",
		},
		{
			name: "execution repo error",
			userRepo: &mockUserRepository{
				users: []*api.User{},
			},
			executionRepo: &mockExecutionRepository{
				err: errors.New("execution repo error"),
			},
			secretsRepo: &mockSecretsRepository{
				secrets: []*api.Secret{},
			},
			imageRepo: nil,
			wantError: true,
			errorMsg:  "failed to load resource ownerships",
		},
		{
			name: "image repo error",
			userRepo: &mockUserRepository{
				users: []*api.User{},
			},
			executionRepo: &mockExecutionRepository{
				executions: []*api.Execution{},
			},
			secretsRepo: &mockSecretsRepository{
				secrets: []*api.Secret{},
			},
			imageRepo: &mockImageRepository{
				err: errors.New("image repo error"),
			},
			wantError: true,
			errorMsg:  "failed to load resource ownerships",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			e, err := NewEnforcer(logger)
			if err != nil {
				t.Fatalf("NewEnforcer() failed: %v", err)
			}

			err = e.Hydrate(context.Background(), tt.userRepo, tt.executionRepo, tt.secretsRepo, tt.imageRepo)

			if tt.wantError {
				if err == nil {
					t.Errorf("Hydrate() error = nil, want error containing %q", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Hydrate() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Hydrate() error = %v, want nil", err)
				}
			}
		})
	}
}

func TestLoadUserRoles(t *testing.T) {
	tests := []struct {
		name      string
		users     []*api.User
		repoError error
		wantError bool
		errorMsg  string
	}{
		{
			name: "load valid users",
			users: []*api.User{
				{Email: "admin@example.com", Role: "admin"},
				{Email: "dev@example.com", Role: "developer"},
				{Email: "viewer@example.com", Role: "viewer"},
			},
			wantError: false,
		},
		{
			name:      "empty user list",
			users:     []*api.User{},
			wantError: false,
		},
		{
			name:      "repo error",
			repoError: errors.New("database connection failed"),
			wantError: true,
			errorMsg:  "failed to load users",
		},
		{
			name: "user with invalid role",
			users: []*api.User{
				{Email: "user@example.com", Role: "invalid-role"},
			},
			wantError: true,
			errorMsg:  "invalid role",
		},
		{
			name: "user with empty email",
			users: []*api.User{
				{Email: "", Role: "admin"},
			},
			wantError: true,
			errorMsg:  "missing email",
		},
		{
			name: "nil user in list",
			users: []*api.User{
				nil,
			},
			wantError: true,
			errorMsg:  "user is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			e, err := NewEnforcer(logger)
			if err != nil {
				t.Fatalf("NewEnforcer() failed: %v", err)
			}

			userRepo := &mockUserRepository{
				users: tt.users,
				err:   tt.repoError,
			}

			err = e.loadUserRoles(context.Background(), userRepo)

			if tt.wantError {
				if err == nil {
					t.Errorf("loadUserRoles() error = nil, want error containing %q", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("loadUserRoles() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("loadUserRoles() error = %v, want nil", err)
				}

				// Verify roles were loaded
				for _, user := range tt.users {
					if user != nil && user.Email != "" {
						roles, err := e.GetRolesForUser(user.Email)
						if err != nil {
							t.Fatalf("GetRolesForUser(%s) failed: %v", user.Email, err)
						}
						expectedRole := "role:" + user.Role
						if !containsString(roles, expectedRole) {
							t.Errorf("GetRolesForUser(%s) = %v, want to contain %q", user.Email, roles, expectedRole)
						}
					}
				}
			}
		})
	}
}

func TestLoadSecretOwnerships(t *testing.T) {
	tests := []struct {
		name      string
		secrets   []*api.Secret
		repoError error
		wantError bool
		errorMsg  string
	}{
		{
			name: "load valid secrets",
			secrets: []*api.Secret{
				{Name: "db-password", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
				{Name: "api-key", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com", "admin@example.com"}},
			},
			wantError: false,
		},
		{
			name:      "empty secrets list",
			secrets:   []*api.Secret{},
			wantError: false,
		},
		{
			name:      "repo error",
			repoError: errors.New("secrets repo failed"),
			wantError: true,
			errorMsg:  "failed to load secrets",
		},
		{
			name: "secret with empty name",
			secrets: []*api.Secret{
				{Name: "", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
			},
			wantError: true,
			errorMsg:  "missing required fields",
		},
		{
			name: "secret with empty created by",
			secrets: []*api.Secret{
				{Name: "test", CreatedBy: "", OwnedBy: []string{"admin@example.com"}},
			},
			wantError: true,
			errorMsg:  "missing required fields",
		},
		{
			name: "nil secret in list",
			secrets: []*api.Secret{
				nil,
			},
			wantError: true,
			errorMsg:  "secret is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			e, err := NewEnforcer(logger)
			if err != nil {
				t.Fatalf("NewEnforcer() failed: %v", err)
			}

			secretsRepo := &mockSecretsRepository{
				secrets: tt.secrets,
				err:     tt.repoError,
			}

			err = e.loadSecretOwnerships(context.Background(), secretsRepo)

			if tt.wantError {
				if err == nil {
					t.Errorf("loadSecretOwnerships() error = nil, want error containing %q", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("loadSecretOwnerships() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("loadSecretOwnerships() error = %v, want nil", err)
				}

				// Verify ownerships were loaded
				for _, secret := range tt.secrets {
					if secret != nil && secret.Name != "" {
						resourceID := FormatResourceID("secret", secret.Name)
						for _, owner := range secret.OwnedBy {
							hasOwnership, err := e.HasOwnershipForResource(resourceID, owner)
							if err != nil {
								t.Fatalf("HasOwnershipForResource(%s, %s) failed: %v", resourceID, owner, err)
							}
							if !hasOwnership {
								t.Errorf("HasOwnershipForResource(%s, %s) = false, want true", resourceID, owner)
							}
						}
					}
				}
			}
		})
	}
}

func TestLoadExecutionOwnerships(t *testing.T) {
	tests := []struct {
		name       string
		executions []*api.Execution
		repoError  error
		wantError  bool
		errorMsg   string
	}{
		{
			name: "load valid executions",
			executions: []*api.Execution{
				{ExecutionID: "exec-1", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com"}},
				{ExecutionID: "exec-2", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com", "dev@example.com"}},
			},
			wantError: false,
		},
		{
			name:       "empty executions list",
			executions: []*api.Execution{},
			wantError:  false,
		},
		{
			name:      "repo error",
			repoError: errors.New("executions repo failed"),
			wantError: true,
			errorMsg:  "failed to load executions",
		},
		{
			name: "execution with empty ID",
			executions: []*api.Execution{
				{ExecutionID: "", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com"}},
			},
			wantError: true,
			errorMsg:  "missing required fields",
		},
		{
			name: "execution with empty created by",
			executions: []*api.Execution{
				{ExecutionID: "exec-1", CreatedBy: "", OwnedBy: []string{"dev@example.com"}},
			},
			wantError: true,
			errorMsg:  "missing required fields",
		},
		{
			name: "nil execution in list",
			executions: []*api.Execution{
				nil,
			},
			wantError: true,
			errorMsg:  "execution is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			e, err := NewEnforcer(logger)
			if err != nil {
				t.Fatalf("NewEnforcer() failed: %v", err)
			}

			executionRepo := &mockExecutionRepository{
				executions: tt.executions,
				err:        tt.repoError,
			}

			err = e.loadExecutionOwnerships(context.Background(), executionRepo)

			if tt.wantError {
				if err == nil {
					t.Errorf("loadExecutionOwnerships() error = nil, want error containing %q", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("loadExecutionOwnerships() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("loadExecutionOwnerships() error = %v, want nil", err)
				}

				// Verify ownerships were loaded
				for _, execution := range tt.executions {
					if execution != nil && execution.ExecutionID != "" {
						resourceID := FormatResourceID("execution", execution.ExecutionID)
						for _, owner := range execution.OwnedBy {
							hasOwnership, err := e.HasOwnershipForResource(resourceID, owner)
							if err != nil {
								t.Fatalf("HasOwnershipForResource(%s, %s) failed: %v", resourceID, owner, err)
							}
							if !hasOwnership {
								t.Errorf("HasOwnershipForResource(%s, %s) = false, want true", resourceID, owner)
							}
						}
					}
				}
			}
		})
	}
}

func TestLoadImageOwnerships(t *testing.T) {
	tests := []struct {
		name      string
		images    []api.ImageInfo
		repoError error
		wantError bool
		errorMsg  string
	}{
		{
			name: "load valid images",
			images: []api.ImageInfo{
				{ImageID: "img-1", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
				{ImageID: "img-2", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com", "admin@example.com"}},
			},
			wantError: false,
		},
		{
			name:      "empty images list",
			images:    []api.ImageInfo{},
			wantError: false,
		},
		{
			name:      "repo error",
			repoError: errors.New("images repo failed"),
			wantError: true,
			errorMsg:  "failed to load images",
		},
		{
			name: "image with empty ID",
			images: []api.ImageInfo{
				{ImageID: "", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
			},
			wantError: true,
			errorMsg:  "missing required fields",
		},
		{
			name: "image with empty created by",
			images: []api.ImageInfo{
				{ImageID: "img-1", CreatedBy: "", OwnedBy: []string{"admin@example.com"}},
			},
			wantError: true,
			errorMsg:  "missing required fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			e, err := NewEnforcer(logger)
			if err != nil {
				t.Fatalf("NewEnforcer() failed: %v", err)
			}

			imageRepo := &mockImageRepository{
				images: tt.images,
				err:    tt.repoError,
			}

			err = e.loadImageOwnerships(context.Background(), imageRepo)

			if tt.wantError {
				if err == nil {
					t.Errorf("loadImageOwnerships() error = nil, want error containing %q", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("loadImageOwnerships() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("loadImageOwnerships() error = %v, want nil", err)
				}

				// Verify ownerships were loaded
				for _, image := range tt.images {
					if image.ImageID != "" {
						resourceID := FormatResourceID("image", image.ImageID)
						for _, owner := range image.OwnedBy {
							hasOwnership, err := e.HasOwnershipForResource(resourceID, owner)
							if err != nil {
								t.Fatalf("HasOwnershipForResource(%s, %s) failed: %v", resourceID, owner, err)
							}
							if !hasOwnership {
								t.Errorf("HasOwnershipForResource(%s, %s) = false, want true", resourceID, owner)
							}
						}
					}
				}
			}
		})
	}
}

func TestLoadResourceOwnershipsFromRepos(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	e, err := NewEnforcer(logger)
	if err != nil {
		t.Fatalf("NewEnforcer() failed: %v", err)
	}

	executionRepo := &mockExecutionRepository{
		executions: []*api.Execution{
			{ExecutionID: "exec-1", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com"}},
		},
	}

	secretsRepo := &mockSecretsRepository{
		secrets: []*api.Secret{
			{Name: "db-password", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
		},
	}

	imageRepo := &mockImageRepository{
		images: []api.ImageInfo{
			{ImageID: "img-1", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
		},
	}

	err = e.loadResourceOwnerships(context.Background(), executionRepo, secretsRepo, imageRepo)
	if err != nil {
		t.Fatalf("loadResourceOwnerships() error = %v, want nil", err)
	}

	// Verify all ownerships were loaded
	testCases := []struct {
		resourceID string
		owner      string
	}{
		{"execution:exec-1", "dev@example.com"},
		{"secret:db-password", "admin@example.com"},
		{"image:img-1", "admin@example.com"},
	}

	for _, tc := range testCases {
		hasOwnership, err := e.HasOwnershipForResource(tc.resourceID, tc.owner)
		if err != nil {
			t.Fatalf("HasOwnershipForResource(%s, %s) failed: %v", tc.resourceID, tc.owner, err)
		}
		if !hasOwnership {
			t.Errorf("HasOwnershipForResource(%s, %s) = false, want true", tc.resourceID, tc.owner)
		}
	}
}
