package health

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
)

func newTestEnforcer(t *testing.T) *authorization.Enforcer {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	enforcer, err := authorization.NewEnforcer(log)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	return enforcer
}

func TestCheckSingleUserRoleMissingEnforcerRole(t *testing.T) {
	enforcer := newTestEnforcer(t)
	manager := &Manager{enforcer: enforcer}
	status := &api.AuthorizerHealthStatus{}

	user := &api.User{Email: "dev@example.com", Role: "developer"}

	issues := manager.checkSingleUserRole(user, status)

	assert.Len(t, issues, 1)
	assert.Contains(t, status.UsersWithMissingRoles, "dev@example.com")
	assert.Contains(t, issues[0].Message, "role \"developer\"")
}

func TestCheckResourceOwnershipGenericMissingOwnership(t *testing.T) {
	ctx := context.Background()
	enforcer := newTestEnforcer(t)
	manager := &Manager{enforcer: enforcer}
	status := &api.AuthorizerHealthStatus{}

	issues := manager.checkResourceOwnershipGeneric(
		"secret",
		"secret-123",
		"creator@example.com",
		[]string{"owner@example.com"},
		status,
	)

	assert.Len(t, issues, 1)
	assert.Contains(t, status.MissingOwnerships, "secret:secret-123")
	assert.Contains(t, issues[0].Message, "owner@example.com")

	// Add ownership and verify the issue disappears
	addErr := enforcer.AddOwnershipForResource(ctx, "secret:secret-123", "owner@example.com")
	assert.NoError(t, addErr)

	status = &api.AuthorizerHealthStatus{}
	issues = manager.checkResourceOwnershipGeneric(
		"secret",
		"secret-123",
		"creator@example.com",
		[]string{"owner@example.com"},
		status,
	)

	assert.Empty(t, issues)
	assert.Empty(t, status.MissingOwnerships)
}

func TestCheckPolicyOrphaned(t *testing.T) {
	status := &api.AuthorizerHealthStatus{}
	manager := &Manager{}

	maps := &resourceMaps{
		userMap:      map[string]bool{"owner@example.com": true},
		secretMap:    map[string]bool{},
		executionMap: map[string]bool{},
	}

	t.Run("missing user for ownership", func(t *testing.T) {
		issues := manager.checkPolicyOrphaned(
			[]string{"secret:api-key", "ghost@example.com"},
			maps,
			status,
		)

		assert.Len(t, issues, 1)
		assert.Contains(t, status.OrphanedOwnerships, "secret:api-key -> ghost@example.com")
		assert.Contains(t, issues[0].Message, "owner user does not exist")
	})

	t.Run("secret resource missing", func(t *testing.T) {
		status = &api.AuthorizerHealthStatus{}
		issues := manager.checkPolicyOrphaned(
			[]string{"secret:missing", "owner@example.com"},
			maps,
			status,
		)

		assert.Len(t, issues, 1)
		assert.Contains(t, status.OrphanedOwnerships, "secret:missing -> owner@example.com")
		assert.Contains(t, issues[0].Message, "secret resource does not exist")
	})
}

// mockUserRepositoryForCasbin implements database.UserRepository for testing
type mockUserRepositoryForCasbin struct {
	listUsersFunc func(ctx context.Context) ([]*api.User, error)
}

func (m *mockUserRepositoryForCasbin) ListUsers(ctx context.Context) ([]*api.User, error) {
	if m.listUsersFunc != nil {
		return m.listUsersFunc(ctx)
	}
	return []*api.User{}, nil
}

func (m *mockUserRepositoryForCasbin) CreateUser(_ context.Context, _ *api.User, _ string, _ int64) error {
	return errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) RemoveExpiration(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) GetUserByEmail(_ context.Context, _ string) (*api.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) GetUserByAPIKeyHash(_ context.Context, _ string) (*api.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) UpdateLastUsed(_ context.Context, _ string) (*time.Time, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) RevokeUser(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) CreatePendingAPIKey(_ context.Context, _ *api.PendingAPIKey) error {
	return errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) GetPendingAPIKey(_ context.Context, _ string) (*api.PendingAPIKey, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) DeletePendingAPIKey(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) MarkAsViewed(_ context.Context, _, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepositoryForCasbin) GetUsersByRequestID(_ context.Context, _ string) ([]*api.User, error) {
	return nil, errors.New("not implemented")
}

// mockSecretsRepositoryForCasbin implements database.SecretsRepository for testing
type mockSecretsRepositoryForCasbin struct {
	listSecretsFunc func(ctx context.Context, includeValue bool) ([]*api.Secret, error)
}

func (m *mockSecretsRepositoryForCasbin) ListSecrets(ctx context.Context, includeValue bool) ([]*api.Secret, error) {
	if m.listSecretsFunc != nil {
		return m.listSecretsFunc(ctx, includeValue)
	}
	return []*api.Secret{}, nil
}

func (m *mockSecretsRepositoryForCasbin) CreateSecret(_ context.Context, _ *api.Secret) error {
	return errors.New("not implemented")
}

func (m *mockSecretsRepositoryForCasbin) GetSecret(_ context.Context, _ string, _ bool) (*api.Secret, error) {
	return nil, errors.New("not implemented")
}

func (m *mockSecretsRepositoryForCasbin) UpdateSecret(_ context.Context, _ *api.Secret) error {
	return errors.New("not implemented")
}

func (m *mockSecretsRepositoryForCasbin) DeleteSecret(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (m *mockSecretsRepositoryForCasbin) GetSecretsByRequestID(_ context.Context, _ string) ([]*api.Secret, error) {
	return nil, errors.New("not implemented")
}

// mockExecutionRepositoryForCasbin implements database.ExecutionRepository for testing
type mockExecutionRepositoryForCasbin struct {
	listExecutionsFunc func(ctx context.Context, limit int, statuses []string) ([]*api.Execution, error)
}

func (m *mockExecutionRepositoryForCasbin) ListExecutions(
	ctx context.Context, limit int, statuses []string) ([]*api.Execution, error) {
	if m.listExecutionsFunc != nil {
		return m.listExecutionsFunc(ctx, limit, statuses)
	}
	return []*api.Execution{}, nil
}

func (m *mockExecutionRepositoryForCasbin) CreateExecution(_ context.Context, _ *api.Execution) error {
	return errors.New("not implemented")
}

func (m *mockExecutionRepositoryForCasbin) GetExecution(_ context.Context, _ string) (*api.Execution, error) {
	return nil, errors.New("not implemented")
}

func (m *mockExecutionRepositoryForCasbin) UpdateExecution(_ context.Context, _ *api.Execution) error {
	return errors.New("not implemented")
}

func (m *mockExecutionRepositoryForCasbin) GetExecutionsByRequestID(
	_ context.Context, _ string) ([]*api.Execution, error) {
	return nil, errors.New("not implemented")
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"single character", "a", "A"},
		{"lowercase", "hello", "Hello"},
		{"uppercase", "HELLO", "HELLO"},
		{"mixed case", "hELLo", "HELLo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := capitalizeFirst(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckSingleUserRole_EmptyEmail(t *testing.T) {
	manager := &Manager{}
	status := &api.AuthorizerHealthStatus{}

	user := &api.User{Email: "", Role: "developer"}

	issues := manager.checkSingleUserRole(user, status)

	assert.Len(t, issues, 1)
	assert.Equal(t, "user", issues[0].ResourceType)
	assert.Equal(t, "unknown", issues[0].ResourceID)
	assert.Contains(t, issues[0].Message, "empty email")
}

func TestCheckSingleUserRole_EmptyRole(t *testing.T) {
	manager := &Manager{}
	status := &api.AuthorizerHealthStatus{}

	user := &api.User{Email: "user@example.com", Role: ""}

	issues := manager.checkSingleUserRole(user, status)

	assert.Len(t, issues, 1)
	assert.Contains(t, status.UsersWithMissingRoles, "user@example.com")
	assert.Contains(t, issues[0].Message, "empty role field")
}

func TestCheckSingleUserRole_InvalidRole(t *testing.T) {
	manager := &Manager{}
	status := &api.AuthorizerHealthStatus{}

	user := &api.User{Email: "user@example.com", Role: "invalid-role"}

	issues := manager.checkSingleUserRole(user, status)

	assert.Len(t, issues, 1)
	assert.Contains(t, status.UsersWithInvalidRoles, "user@example.com")
	assert.Contains(t, issues[0].Message, "invalid role")
}

func TestCheckSingleUserRole_NilUser(t *testing.T) {
	manager := &Manager{}
	status := &api.AuthorizerHealthStatus{}

	issues := manager.checkSingleUserRole(nil, status)

	assert.Empty(t, issues)
}

func TestCheckSingleUserRole_ValidRoleInEnforcer(t *testing.T) {
	ctx := context.Background()
	enforcer := newTestEnforcer(t)
	manager := &Manager{enforcer: enforcer}
	status := &api.AuthorizerHealthStatus{}

	user := &api.User{Email: "admin@example.com", Role: "admin"}

	// Add role to enforcer
	err := enforcer.AddRoleForUser(ctx, "admin@example.com", "admin")
	require.NoError(t, err)

	issues := manager.checkSingleUserRole(user, status)

	assert.Empty(t, issues)
	assert.Empty(t, status.UsersWithMissingRoles)
}

func TestCheckUserRoles_Success(t *testing.T) {
	ctx := context.Background()
	enforcer := newTestEnforcer(t)
	userRepo := &mockUserRepositoryForCasbin{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return []*api.User{
				{Email: "admin@example.com", Role: "admin"},
				{Email: "dev@example.com", Role: "developer"},
			}, nil
		},
	}

	manager := &Manager{
		enforcer: enforcer,
		userRepo: userRepo,
	}
	status := &api.AuthorizerHealthStatus{}

	issues, err := manager.checkUserRoles(ctx, nil, status)

	assert.NoError(t, err)
	assert.Equal(t, 2, status.TotalUsersChecked)
	// Both users should have missing roles since they're not in enforcer
	assert.Len(t, issues, 2)
}

func TestCheckUserRoles_ListUsersError(t *testing.T) {
	ctx := context.Background()
	userRepo := &mockUserRepositoryForCasbin{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return nil, errors.New("database error")
		},
	}

	manager := &Manager{userRepo: userRepo}
	status := &api.AuthorizerHealthStatus{}

	_, err := manager.checkUserRoles(ctx, nil, status)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list users")
}

func TestReconcileCasbin_Success(t *testing.T) {
	ctx := context.Background()
	enforcer := newTestEnforcer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	userRepo := &mockUserRepositoryForCasbin{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return []*api.User{
				{Email: "admin@example.com", Role: "admin"},
			}, nil
		},
	}

	secretsRepo := &mockSecretsRepositoryForCasbin{
		listSecretsFunc: func(_ context.Context, _ bool) ([]*api.Secret, error) {
			return []*api.Secret{
				{Name: "secret-1", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
			}, nil
		},
	}

	executionRepo := &mockExecutionRepositoryForCasbin{
		listExecutionsFunc: func(_ context.Context, _ int, _ []string) ([]*api.Execution, error) {
			return []*api.Execution{
				{ExecutionID: "exec-1", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
			}, nil
		},
	}

	manager := &Manager{
		enforcer:      enforcer,
		userRepo:      userRepo,
		secretsRepo:   secretsRepo,
		executionRepo: executionRepo,
	}

	status, issues, err := manager.reconcileCasbin(ctx, logger)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.NotNil(t, issues)
}

func TestReconcileCasbin_CheckUserRolesError(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	userRepo := &mockUserRepositoryForCasbin{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return nil, errors.New("database error")
		},
	}

	manager := &Manager{userRepo: userRepo}

	_, _, err := manager.reconcileCasbin(ctx, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check user roles")
}

func TestCheckResourceOwnership_Success(t *testing.T) {
	ctx := context.Background()
	enforcer := newTestEnforcer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	secretsRepo := &mockSecretsRepositoryForCasbin{
		listSecretsFunc: func(_ context.Context, _ bool) ([]*api.Secret, error) {
			return []*api.Secret{
				{Name: "secret-1", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
			}, nil
		},
	}

	executionRepo := &mockExecutionRepositoryForCasbin{
		listExecutionsFunc: func(_ context.Context, _ int, _ []string) ([]*api.Execution, error) {
			return []*api.Execution{
				{ExecutionID: "exec-1", CreatedBy: "admin@example.com", OwnedBy: []string{"admin@example.com"}},
			}, nil
		},
	}

	manager := &Manager{
		enforcer:      enforcer,
		secretsRepo:   secretsRepo,
		executionRepo: executionRepo,
	}
	status := &api.AuthorizerHealthStatus{}

	issues, err := manager.checkResourceOwnership(ctx, logger, status)

	assert.NoError(t, err)
	assert.Equal(t, 2, status.TotalResourcesChecked)
	// Resources should have missing ownerships since they're not in enforcer
	assert.NotEmpty(t, issues)
}

func TestCheckResourceOwnership_ListSecretsError(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	secretsRepo := &mockSecretsRepositoryForCasbin{
		listSecretsFunc: func(_ context.Context, _ bool) ([]*api.Secret, error) {
			return nil, errors.New("database error")
		},
	}

	manager := &Manager{secretsRepo: secretsRepo}
	status := &api.AuthorizerHealthStatus{}

	_, err := manager.checkResourceOwnership(ctx, logger, status)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list secrets")
}

func TestCheckResourceOwnership_ListExecutionsError(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	executionRepo := &mockExecutionRepositoryForCasbin{
		listExecutionsFunc: func(_ context.Context, _ int, _ []string) ([]*api.Execution, error) {
			return nil, errors.New("database error")
		},
	}

	manager := &Manager{executionRepo: executionRepo}
	status := &api.AuthorizerHealthStatus{}

	_, err := manager.checkResourceOwnership(ctx, logger, status)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list executions")
}

func TestCheckSecretOwnership_NilSecret(t *testing.T) {
	manager := &Manager{}
	status := &api.AuthorizerHealthStatus{}

	issues := manager.checkSecretOwnership(nil, status)

	assert.Empty(t, issues)
}

func TestCheckSecretOwnership_EmptyName(t *testing.T) {
	manager := &Manager{}
	status := &api.AuthorizerHealthStatus{}

	secret := &api.Secret{Name: ""}
	issues := manager.checkSecretOwnership(secret, status)

	assert.Empty(t, issues)
}

func TestCheckExecutionOwnership_NilExecution(t *testing.T) {
	manager := &Manager{}
	status := &api.AuthorizerHealthStatus{}

	issues := manager.checkExecutionOwnership(nil, status)

	assert.Empty(t, issues)
}

func TestCheckExecutionOwnership_EmptyExecutionID(t *testing.T) {
	manager := &Manager{}
	status := &api.AuthorizerHealthStatus{}

	execution := &api.Execution{ExecutionID: ""}
	issues := manager.checkExecutionOwnership(execution, status)

	assert.Empty(t, issues)
}

func TestCheckImageOwnership_EmptyImageID(t *testing.T) {
	manager := &Manager{}
	status := &api.AuthorizerHealthStatus{}

	image := &api.ImageInfo{ImageID: ""}
	issues := manager.checkImageOwnership(image, status)

	assert.Empty(t, issues)
}

func TestCheckOrphanedOwnerships_Success(t *testing.T) {
	ctx := context.Background()
	enforcer := newTestEnforcer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Add an orphaned ownership to enforcer
	err := enforcer.AddOwnershipForResource(ctx, "secret:orphan-secret", "ghost@example.com")
	require.NoError(t, err)

	userRepo := &mockUserRepositoryForCasbin{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return []*api.User{
				{Email: "admin@example.com", Role: "admin"},
			}, nil
		},
	}

	secretsRepo := &mockSecretsRepositoryForCasbin{
		listSecretsFunc: func(_ context.Context, _ bool) ([]*api.Secret, error) {
			return []*api.Secret{}, nil
		},
	}

	executionRepo := &mockExecutionRepositoryForCasbin{
		listExecutionsFunc: func(_ context.Context, _ int, _ []string) ([]*api.Execution, error) {
			return []*api.Execution{}, nil
		},
	}

	manager := &Manager{
		enforcer:      enforcer,
		userRepo:      userRepo,
		secretsRepo:   secretsRepo,
		executionRepo: executionRepo,
	}
	status := &api.AuthorizerHealthStatus{}

	issues, err := manager.checkOrphanedOwnerships(ctx, logger, status)

	assert.NoError(t, err)
	// Should detect orphaned ownership (ghost user doesn't exist)
	assert.NotEmpty(t, issues)
	assert.NotEmpty(t, status.OrphanedOwnerships)
}

func TestCheckOrphanedOwnerships_BuildResourceMapsError(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	userRepo := &mockUserRepositoryForCasbin{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return nil, errors.New("database error")
		},
	}

	manager := &Manager{userRepo: userRepo}
	status := &api.AuthorizerHealthStatus{}

	_, err := manager.checkOrphanedOwnerships(ctx, logger, status)

	assert.Error(t, err)
}

func TestBuildResourceMaps_Success(t *testing.T) {
	ctx := context.Background()

	userRepo := &mockUserRepositoryForCasbin{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return []*api.User{
				{Email: "admin@example.com"},
				{Email: ""}, // Should be skipped
				nil,         // Should be skipped
			}, nil
		},
	}

	secretsRepo := &mockSecretsRepositoryForCasbin{
		listSecretsFunc: func(_ context.Context, _ bool) ([]*api.Secret, error) {
			return []*api.Secret{
				{Name: "secret-1"},
				{Name: ""}, // Should be skipped
				nil,        // Should be skipped
			}, nil
		},
	}

	executionRepo := &mockExecutionRepositoryForCasbin{
		listExecutionsFunc: func(_ context.Context, _ int, _ []string) ([]*api.Execution, error) {
			return []*api.Execution{
				{ExecutionID: "exec-1"},
				{ExecutionID: ""}, // Should be skipped
				nil,               // Should be skipped
			}, nil
		},
	}

	manager := &Manager{
		userRepo:      userRepo,
		secretsRepo:   secretsRepo,
		executionRepo: executionRepo,
	}

	maps, err := manager.buildResourceMaps(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, maps)
	assert.True(t, maps.userMap["admin@example.com"])
	assert.True(t, maps.secretMap["secret-1"])
	assert.True(t, maps.executionMap["exec-1"])
}

func TestCheckExecutionOrphaned_OrphanedExecution(t *testing.T) {
	status := &api.AuthorizerHealthStatus{}
	manager := &Manager{}

	maps := &resourceMaps{
		userMap:      map[string]bool{"owner@example.com": true},
		secretMap:    map[string]bool{},
		executionMap: map[string]bool{}, // execution doesn't exist
	}

	issues := manager.checkExecutionOrphaned(
		"execution:missing-exec",
		"missing-exec",
		"owner@example.com",
		maps,
		status,
	)

	assert.Len(t, issues, 1)
	assert.Contains(t, status.OrphanedOwnerships, "execution:missing-exec -> owner@example.com")
	assert.Contains(t, issues[0].Message, "execution resource does not exist")
}

func TestCheckExecutionOrphaned_ValidExecution(t *testing.T) {
	status := &api.AuthorizerHealthStatus{}
	manager := &Manager{}

	maps := &resourceMaps{
		userMap:      map[string]bool{"owner@example.com": true},
		secretMap:    map[string]bool{},
		executionMap: map[string]bool{"valid-exec": true},
	}

	issues := manager.checkExecutionOrphaned(
		"execution:valid-exec",
		"valid-exec",
		"owner@example.com",
		maps,
		status,
	)

	assert.Empty(t, issues)
}
