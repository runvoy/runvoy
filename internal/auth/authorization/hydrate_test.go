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

func (m *mockUserRepository) CreateUser(_ context.Context, _ *api.User, _ string, _ int64) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) RemoveExpiration(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) GetUserByEmail(_ context.Context, _ string) (*api.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) GetUserByAPIKeyHash(_ context.Context, _ string) (*api.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) UpdateLastUsed(_ context.Context, _ string) (*time.Time, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) RevokeUser(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) ListUsers(_ context.Context) ([]*api.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.users, nil
}

func (m *mockUserRepository) CreatePendingAPIKey(_ context.Context, _ *api.PendingAPIKey) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) GetPendingAPIKey(_ context.Context, _ string) (*api.PendingAPIKey, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) DeletePendingAPIKey(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) MarkAsViewed(_ context.Context, _, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepository) GetUsersByRequestID(_ context.Context, _ string) ([]*api.User, error) {
	return nil, errors.New("not implemented")
}

type mockExecutionRepository struct {
	executions []*api.Execution
	err        error
}

func (m *mockExecutionRepository) CreateExecution(_ context.Context, _ *api.Execution) error {
	return errors.New("not implemented")
}

func (m *mockExecutionRepository) GetExecution(_ context.Context, _ string) (*api.Execution, error) {
	return nil, errors.New("not implemented")
}

func (m *mockExecutionRepository) UpdateExecution(_ context.Context, _ *api.Execution) error {
	return errors.New("not implemented")
}

func (m *mockExecutionRepository) ListExecutions(_ context.Context, _ int, _ []string) ([]*api.Execution, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.executions, nil
}

func (m *mockExecutionRepository) GetExecutionsByRequestID(_ context.Context, _ string) ([]*api.Execution, error) {
	return nil, errors.New("not implemented")
}

type mockSecretsRepository struct {
	secrets []*api.Secret
	err     error
}

func (m *mockSecretsRepository) CreateSecret(_ context.Context, _ *api.Secret) error {
	return errors.New("not implemented")
}

func (m *mockSecretsRepository) GetSecret(_ context.Context, _ string, _ bool) (*api.Secret, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSecretsRepository) ListSecrets(_ context.Context, _ bool) ([]*api.Secret, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.secrets, nil
}

func (m *mockSecretsRepository) UpdateSecret(_ context.Context, _ *api.Secret) error {
	return errors.New("not implemented")
}

func (m *mockSecretsRepository) DeleteSecret(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (m *mockSecretsRepository) GetSecretsByRequestID(_ context.Context, _ string) ([]*api.Secret, error) {
	return nil, errors.New("not implemented")
}

type mockImageRepository struct {
	images []api.ImageInfo
	err    error
}

func (m *mockImageRepository) ListImages(_ context.Context) ([]api.ImageInfo, error) {
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
						roles, verifyErr := e.GetRolesForUser(user.Email)
						if verifyErr != nil {
							t.Fatalf("GetRolesForUser(%s) failed: %v", user.Email, verifyErr)
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

type ownershipTestConfig struct {
	name      string
	repoError error
	wantError bool
	errorMsg  string
	setup     func() (*Enforcer, error)
	verify    func(*testing.T, *Enforcer)
}

func newTestEnforcer() (*Enforcer, error) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewEnforcer(logger)
}

func runOwnershipTests(t *testing.T, tests []ownershipTestConfig, funcName string) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, loadErr := tt.setup()
			if e == nil {
				if loadErr == nil {
					t.Fatalf("setup returned nil enforcer and nil error")
				}
				t.Fatalf("NewEnforcer() failed: %v", loadErr)
			}

			if tt.wantError {
				if loadErr == nil {
					t.Errorf("%s() error = nil, want error containing %q", funcName, tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(loadErr.Error(), tt.errorMsg) {
					t.Errorf("%s() error = %v, want error containing %q", funcName, loadErr, tt.errorMsg)
				}
			} else {
				if loadErr != nil {
					t.Errorf("%s() error = %v, want nil", funcName, loadErr)
				}
				tt.verify(t, e)
			}
		})
	}
}

//nolint:dupl // Test functions are similar but test different types (secrets vs executions)
func TestLoadSecretOwnerships(t *testing.T) {
	runOwnershipTests(t, []ownershipTestConfig{
		{
			name:      "load valid secrets",
			repoError: nil,
			wantError: false,
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				secretsRepo := &mockSecretsRepository{
					secrets: []*api.Secret{
						{Name: "db-password", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
						{Name: "api-key", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com", "admin@example.com"}},
					},
				}
				loadErr := e.loadSecretOwnerships(context.Background(), secretsRepo)
				return e, loadErr
			},
			verify: func(t *testing.T, e *Enforcer) {
				secrets := []*api.Secret{
					{Name: "db-password", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
					{Name: "api-key", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com", "admin@example.com"}},
				}
				verifySecretOwnerships(t, e, secrets)
			},
		},
		{
			name:      "empty secrets list",
			repoError: nil,
			wantError: false,
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				secretsRepo := &mockSecretsRepository{secrets: []*api.Secret{}}
				loadErr := e.loadSecretOwnerships(context.Background(), secretsRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
		{
			name:      "repo error",
			repoError: errors.New("secrets repo failed"),
			wantError: true,
			errorMsg:  "failed to load secrets",
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				secretsRepo := &mockSecretsRepository{err: errors.New("secrets repo failed")}
				loadErr := e.loadSecretOwnerships(context.Background(), secretsRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
		{
			name:      "secret with empty name",
			repoError: nil,
			wantError: true,
			errorMsg:  "missing required fields",
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				secretsRepo := &mockSecretsRepository{
					secrets: []*api.Secret{
						{Name: "", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
					},
				}
				loadErr := e.loadSecretOwnerships(context.Background(), secretsRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
		{
			name:      "secret with empty created by",
			repoError: nil,
			wantError: true,
			errorMsg:  "missing required fields",
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				secretsRepo := &mockSecretsRepository{
					secrets: []*api.Secret{
						{Name: "test", CreatedBy: "", OwnedBy: []string{"admin@example.com"}},
					},
				}
				loadErr := e.loadSecretOwnerships(context.Background(), secretsRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
		{
			name:      "nil secret in list",
			repoError: nil,
			wantError: true,
			errorMsg:  "secret is nil",
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				secretsRepo := &mockSecretsRepository{secrets: []*api.Secret{nil}}
				loadErr := e.loadSecretOwnerships(context.Background(), secretsRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
	}, "loadSecretOwnerships")
}

func verifySecretOwnerships(t *testing.T, e *Enforcer, secrets []*api.Secret) {
	for _, secret := range secrets {
		if secret != nil && secret.Name != "" {
			resourceID := FormatResourceID("secret", secret.Name)
			for _, owner := range secret.OwnedBy {
				hasOwner, verifyErr := e.HasOwnershipForResource(resourceID, owner)
				if verifyErr != nil {
					t.Fatalf("HasOwnershipForResource(%s, %s) failed: %v", resourceID, owner, verifyErr)
				}
				if !hasOwner {
					t.Errorf("HasOwnershipForResource(%s, %s) = false, want true", resourceID, owner)
				}
			}
		}
	}
}

//nolint:dupl // Test functions are similar but test different types (secrets vs executions)
func TestLoadExecutionOwnerships(t *testing.T) {
	runOwnershipTests(t, []ownershipTestConfig{
		{
			name:      "load valid executions",
			repoError: nil,
			wantError: false,
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				executionRepo := &mockExecutionRepository{
					executions: []*api.Execution{
						{
							ExecutionID: "exec-1",
							CreatedBy:   "dev@example.com",
							OwnedBy:     []string{"dev@example.com"},
						},
						{
							ExecutionID: "exec-2",
							CreatedBy:   "admin@example.com",
							OwnedBy:     []string{"admin@example.com", "dev@example.com"},
						},
					},
				}
				loadErr := e.loadExecutionOwnerships(context.Background(), executionRepo)
				return e, loadErr
			},
			verify: func(t *testing.T, e *Enforcer) {
				executions := []*api.Execution{
					{
						ExecutionID: "exec-1",
						CreatedBy:   "dev@example.com",
						OwnedBy:     []string{"dev@example.com"},
					},
					{
						ExecutionID: "exec-2",
						CreatedBy:   "admin@example.com",
						OwnedBy:     []string{"admin@example.com", "dev@example.com"},
					},
				}
				verifyExecutionOwnerships(t, e, executions)
			},
		},
		{
			name:      "empty executions list",
			repoError: nil,
			wantError: false,
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				executionRepo := &mockExecutionRepository{executions: []*api.Execution{}}
				loadErr := e.loadExecutionOwnerships(context.Background(), executionRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
		{
			name:      "repo error",
			repoError: errors.New("executions repo failed"),
			wantError: true,
			errorMsg:  "failed to load executions",
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				executionRepo := &mockExecutionRepository{err: errors.New("executions repo failed")}
				loadErr := e.loadExecutionOwnerships(context.Background(), executionRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
		{
			name:      "execution with empty ID",
			repoError: nil,
			wantError: true,
			errorMsg:  "missing required fields",
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				executionRepo := &mockExecutionRepository{
					executions: []*api.Execution{
						{ExecutionID: "", CreatedBy: "dev@example.com", OwnedBy: []string{"dev@example.com"}},
					},
				}
				loadErr := e.loadExecutionOwnerships(context.Background(), executionRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
		{
			name:      "execution with empty created by",
			repoError: nil,
			wantError: true,
			errorMsg:  "missing required fields",
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				executionRepo := &mockExecutionRepository{
					executions: []*api.Execution{
						{ExecutionID: "exec-1", CreatedBy: "", OwnedBy: []string{"dev@example.com"}},
					},
				}
				loadErr := e.loadExecutionOwnerships(context.Background(), executionRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
		{
			name:      "nil execution in list",
			repoError: nil,
			wantError: true,
			errorMsg:  "execution is nil",
			setup: func() (*Enforcer, error) {
				e, err := newTestEnforcer()
				if err != nil {
					return nil, err
				}
				executionRepo := &mockExecutionRepository{executions: []*api.Execution{nil}}
				loadErr := e.loadExecutionOwnerships(context.Background(), executionRepo)
				return e, loadErr
			},
			verify: func(*testing.T, *Enforcer) {},
		},
	}, "loadExecutionOwnerships")
}

func verifyExecutionOwnerships(t *testing.T, e *Enforcer, executions []*api.Execution) {
	for _, execution := range executions {
		if execution != nil && execution.ExecutionID != "" {
			resourceID := FormatResourceID("execution", execution.ExecutionID)
			for _, owner := range execution.OwnedBy {
				hasOwner, verifyErr := e.HasOwnershipForResource(resourceID, owner)
				if verifyErr != nil {
					t.Fatalf("HasOwnershipForResource(%s, %s) failed: %v", resourceID, owner, verifyErr)
				}
				if !hasOwner {
					t.Errorf("HasOwnershipForResource(%s, %s) = false, want true", resourceID, owner)
				}
			}
		}
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
							hasOwnership, verifyErr := e.HasOwnershipForResource(resourceID, owner)
							if verifyErr != nil {
								t.Fatalf("HasOwnershipForResource(%s, %s) failed: %v", resourceID, owner, verifyErr)
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
		hasOwnership, verifyErr := e.HasOwnershipForResource(tc.resourceID, tc.owner)
		if verifyErr != nil {
			t.Fatalf("HasOwnershipForResource(%s, %s) failed: %v", tc.resourceID, tc.owner, verifyErr)
		}
		if !hasOwnership {
			t.Errorf("HasOwnershipForResource(%s, %s) = false, want true", tc.resourceID, tc.owner)
		}
	}
}
