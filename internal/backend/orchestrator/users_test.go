package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/database"
	appErrors "runvoy/internal/errors"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var defaultWebSocketManager = &mockWebSocketManager{}

func TestValidateCreateUserRequest_Success(t *testing.T) {
	repo := &mockUserRepository{
		getUserByEmailFunc: func(_ context.Context, _ string) (*api.User, error) {
			return nil, nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	err = service.validateCreateUserRequest(context.Background(), "user@example.com", "viewer")

	assert.NoError(t, err)
}

func TestValidateCreateUserRequest_EmptyEmail(t *testing.T) {
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       &mockUserRepository{},
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	err = service.validateCreateUserRequest(context.Background(), "", "viewer")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

func TestValidateCreateUserRequest_InvalidEmail(t *testing.T) {
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       &mockUserRepository{},
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	err = service.validateCreateUserRequest(context.Background(), "not-an-email", "viewer")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email address")
}

func TestValidateCreateUserRequest_UserAlreadyExists(t *testing.T) {
	repo := &mockUserRepository{
		getUserByEmailFunc: func(_ context.Context, _ string) (*api.User, error) {
			return &api.User{Email: "user@example.com"}, nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	err = service.validateCreateUserRequest(context.Background(), "user@example.com", "viewer")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user with this email already exists")
}

func TestValidateCreateUserRequest_RepositoryError(t *testing.T) {
	repo := &mockUserRepository{
		getUserByEmailFunc: func(_ context.Context, _ string) (*api.User, error) {
			return nil, appErrors.ErrDatabaseError("test error", errors.New("db error"))
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	err = service.validateCreateUserRequest(context.Background(), "user@example.com", "viewer")

	assert.Error(t, err)
}

func TestValidateCreateUserRequest_EmptyRole(t *testing.T) {
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       &mockUserRepository{},
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	err = service.validateCreateUserRequest(context.Background(), "user@example.com", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role is required")
}

func TestValidateCreateUserRequest_InvalidRole(t *testing.T) {
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       &mockUserRepository{},
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	err = service.validateCreateUserRequest(context.Background(), "user@example.com", "superuser")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
	assert.Contains(t, err.Error(), "admin")
	assert.Contains(t, err.Error(), "operator")
	assert.Contains(t, err.Error(), "developer")
	assert.Contains(t, err.Error(), "viewer")
}

func TestGenerateOrUseAPIKey_ProvidedKey(t *testing.T) {
	key, err := generateOrUseAPIKey("my-custom-key")

	assert.NoError(t, err)
	assert.Equal(t, "my-custom-key", key)
}

func TestGenerateOrUseAPIKey_GenerateNew(t *testing.T) {
	key, err := generateOrUseAPIKey("")

	assert.NoError(t, err)
	assert.NotEmpty(t, key)
	assert.Greater(t, len(key), 10) // Generated keys should be reasonably long
}

func TestCreatePendingClaim_Success(t *testing.T) {
	repo := &mockUserRepository{
		createPendingAPIKeyFunc: func(_ context.Context, _ *api.PendingAPIKey) error {
			return nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	secretToken, err := service.createPendingClaim(
		context.Background(),
		"test-api-key",
		"user@example.com",
		"admin@example.com",
		time.Now().Add(15*time.Minute).Unix(),
	)

	assert.NoError(t, err)
	assert.NotEmpty(t, secretToken)
}

func TestCreatePendingClaim_RepositoryError(t *testing.T) {
	repo := &mockUserRepository{
		createPendingAPIKeyFunc: func(_ context.Context, _ *api.PendingAPIKey) error {
			return appErrors.ErrDatabaseError("test error", errors.New("db error"))
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	_, err = service.createPendingClaim(
		context.Background(),
		"test-api-key",
		"user@example.com",
		"admin@example.com",
		time.Now().Add(15*time.Minute).Unix(),
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create pending API key")
}

func TestCreateUser_Success(t *testing.T) {
	repo := &mockUserRepository{
		getUserByEmailFunc: func(_ context.Context, _ string) (*api.User, error) {
			return nil, nil
		},
		createUserFunc: func(_ context.Context, _ *api.User, _ string, _ int64) error {
			return nil
		},
		createPendingAPIKeyFunc: func(_ context.Context, _ *api.PendingAPIKey) error {
			return nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	req := api.CreateUserRequest{Email: "user@example.com", Role: "viewer"}
	resp, err := service.CreateUser(context.Background(), req, "admin@example.com")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "user@example.com", resp.User.Email)
	assert.Equal(t, "viewer", resp.User.Role)
	assert.NotEmpty(t, resp.ClaimToken)
}

func TestCreateUser_InvalidEmail(t *testing.T) {
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       &mockUserRepository{},
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	req := api.CreateUserRequest{Email: "not-valid", Role: "viewer"}
	_, err = service.CreateUser(context.Background(), req, "admin@example.com")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email address")
}

func TestCreateUser_CreateUserError(t *testing.T) {
	repo := &mockUserRepository{
		getUserByEmailFunc: func(_ context.Context, _ string) (*api.User, error) {
			return nil, nil
		},
		createUserFunc: func(_ context.Context, _ *api.User, _ string, _ int64) error {
			return appErrors.ErrDatabaseError("test error", errors.New("db error"))
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	req := api.CreateUserRequest{Email: "user@example.com", Role: "viewer"}
	_, err = service.CreateUser(context.Background(), req, "admin@example.com")

	assert.Error(t, err)
}

func TestCreateUser_MissingRole(t *testing.T) {
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       &mockUserRepository{},
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	req := api.CreateUserRequest{Email: "user@example.com", Role: ""}
	_, err = service.CreateUser(context.Background(), req, "admin@example.com")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role is required")
}

func TestCreateUser_InvalidRole(t *testing.T) {
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       &mockUserRepository{},
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	req := api.CreateUserRequest{Email: "user@example.com", Role: "superadmin"}
	_, err = service.CreateUser(context.Background(), req, "admin@example.com")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
}

func TestCreateUser_SyncsRoleWithEnforcer(t *testing.T) {
	repo := &mockUserRepository{
		getUserByEmailFunc: func(_ context.Context, _ string) (*api.User, error) {
			return nil, nil
		},
		createUserFunc: func(_ context.Context, _ *api.User, _ string, _ int64) error {
			return nil
		},
		createPendingAPIKeyFunc: func(_ context.Context, _ *api.PendingAPIKey) error {
			return nil
		},
	}

	service, enforcer := newTestServiceWithEnforcer(
		repo,
		&mockExecutionRepository{},
		nil,
		nil,
	)

	req := api.CreateUserRequest{Email: "new-user@example.com", Role: "viewer"}
	resp, err := service.CreateUser(context.Background(), req, "admin@example.com")

	require.NoError(t, err)
	require.NotNil(t, resp)

	roles, err := enforcer.GetRolesForUser(req.Email)
	require.NoError(t, err)

	viewerRole, err := authorization.NewRole(req.Role)
	require.NoError(t, err)
	assert.Contains(t, roles, authorization.FormatRole(viewerRole))
}

func TestCreateUser_PendingClaimFailureRollsBackEnforcer(t *testing.T) {
	revokeCalled := 0
	repo := &mockUserRepository{
		getUserByEmailFunc: func(_ context.Context, _ string) (*api.User, error) {
			return nil, nil
		},
		createUserFunc: func(_ context.Context, _ *api.User, _ string, _ int64) error {
			return nil
		},
		createPendingAPIKeyFunc: func(_ context.Context, _ *api.PendingAPIKey) error {
			return appErrors.ErrDatabaseError("pending failure", errors.New("db error"))
		},
		revokeUserFunc: func(_ context.Context, _ string) error {
			revokeCalled++
			return nil
		},
	}

	service, enforcer := newTestServiceWithEnforcer(
		repo,
		&mockExecutionRepository{},
		nil,
		nil,
	)

	req := api.CreateUserRequest{Email: "rollback@example.com", Role: "viewer"}
	_, err := service.CreateUser(context.Background(), req, "admin@example.com")

	require.Error(t, err)
	assert.Equal(t, 1, revokeCalled)

	roles, getErr := enforcer.GetRolesForUser(req.Email)
	require.NoError(t, getErr)
	assert.Empty(t, roles)
}

func TestRevokeUser_RemovesRoleFromEnforcer(t *testing.T) {
	userEmail := "viewer@example.com"
	repo := &mockUserRepository{
		getUserByEmailFunc: func(_ context.Context, _ string) (*api.User, error) {
			return &api.User{Email: userEmail, Role: "viewer"}, nil
		},
		revokeUserFunc: func(_ context.Context, _ string) error {
			return nil
		},
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return []*api.User{
				{Email: userEmail, Role: "viewer"},
			}, nil
		},
	}

	service, enforcer := newTestServiceWithEnforcer(
		repo,
		&mockExecutionRepository{},
		nil,
		nil,
	)

	err := service.RevokeUser(context.Background(), userEmail)
	require.NoError(t, err)

	roles, getErr := enforcer.GetRolesForUser(userEmail)
	require.NoError(t, getErr)
	assert.Empty(t, roles)
}

func TestRevokeUser_RestoresRoleOnFailure(t *testing.T) {
	userEmail := "viewer@example.com"
	repo := &mockUserRepository{
		getUserByEmailFunc: func(_ context.Context, _ string) (*api.User, error) {
			return &api.User{Email: userEmail, Role: "viewer"}, nil
		},
		revokeUserFunc: func(_ context.Context, _ string) error {
			return appErrors.ErrDatabaseError("revoke failed", errors.New("db error"))
		},
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return []*api.User{
				{Email: userEmail, Role: "viewer"},
			}, nil
		},
	}

	service, enforcer := newTestServiceWithEnforcer(
		repo,
		&mockExecutionRepository{},
		nil,
		nil,
	)

	err := service.RevokeUser(context.Background(), userEmail)
	require.Error(t, err)

	roles, getErr := enforcer.GetRolesForUser(userEmail)
	require.NoError(t, getErr)

	viewerRole, roleErr := authorization.NewRole("viewer")
	require.NoError(t, roleErr)
	assert.Contains(t, roles, authorization.FormatRole(viewerRole))
}

func TestClaimAPIKey_Success(t *testing.T) {
	expiredAt := time.Now().Add(15 * time.Minute).Unix()
	repo := &mockUserRepository{
		getPendingAPIKeyFunc: func(_ context.Context, _ string) (*api.PendingAPIKey, error) {
			return &api.PendingAPIKey{
				SecretToken: "token",
				APIKey:      "key",
				UserEmail:   "user@example.com",
				ExpiresAt:   expiredAt,
				Viewed:      false,
			}, nil
		},
		markAsViewedFunc: func(_ context.Context, _ string, _ string) error {
			return nil
		},
		removeExpirationFunc: func(_ context.Context, _ string) error {
			return nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	resp, err := service.ClaimAPIKey(context.Background(), "token", "192.168.1.1")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "key", resp.APIKey)
	assert.Equal(t, "user@example.com", resp.UserEmail)
}

func TestClaimAPIKey_InvalidToken(t *testing.T) {
	repo := &mockUserRepository{
		getPendingAPIKeyFunc: func(_ context.Context, _ string) (*api.PendingAPIKey, error) {
			return nil, nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	_, err = service.ClaimAPIKey(context.Background(), "invalid-token", "192.168.1.1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired token")
}

func TestClaimAPIKey_AlreadyClaimed(t *testing.T) {
	expiredAt := time.Now().Add(15 * time.Minute).Unix()
	repo := &mockUserRepository{
		getPendingAPIKeyFunc: func(_ context.Context, _ string) (*api.PendingAPIKey, error) {
			return &api.PendingAPIKey{
				SecretToken: "token",
				APIKey:      "key",
				UserEmail:   "user@example.com",
				ExpiresAt:   expiredAt,
				Viewed:      true,
			}, nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	_, err = service.ClaimAPIKey(context.Background(), "token", "192.168.1.1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already been claimed")
}

func TestClaimAPIKey_TokenExpired(t *testing.T) {
	expiredAt := time.Now().Add(-15 * time.Minute).Unix() // expired 15 minutes ago
	repo := &mockUserRepository{
		getPendingAPIKeyFunc: func(_ context.Context, _ string) (*api.PendingAPIKey, error) {
			return &api.PendingAPIKey{
				SecretToken: "token",
				APIKey:      "key",
				UserEmail:   "user@example.com",
				ExpiresAt:   expiredAt,
				Viewed:      false,
			}, nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	_, err = service.ClaimAPIKey(context.Background(), "token", "192.168.1.1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestListUsers_Success(t *testing.T) {
	repo := &mockUserRepository{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return []*api.User{
				{Email: "alice@example.com", Role: "admin"},
				{Email: "bob@example.com", Role: "developer"},
			}, nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	resp, err := service.ListUsers(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Users, 2)
	// Verify sorting by email
	assert.Equal(t, "alice@example.com", resp.Users[0].Email)
	assert.Equal(t, "bob@example.com", resp.Users[1].Email)
}

func TestListUsers_Empty(t *testing.T) {
	repo := &mockUserRepository{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			return []*api.User{}, nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	resp, err := service.ListUsers(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Users, 0)
}

func TestListUsers_RepositoryError(t *testing.T) {
	initCallCount := 0
	repo := &mockUserRepository{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			initCallCount++
			// During initialization (first call), return success to allow service creation.
			// During actual ListUsers call (second call), return error.
			if initCallCount == 1 {
				return []*api.User{}, nil
			}
			return nil, appErrors.ErrDatabaseError("test error", errors.New("db error"))
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	_, err = service.ListUsers(context.Background())

	assert.Error(t, err)
}

func TestListUsers_SortingByEmail(t *testing.T) {
	repo := &mockUserRepository{
		listUsersFunc: func(_ context.Context) ([]*api.User, error) {
			// Return users sorted by email (as the database now does)
			return []*api.User{
				{Email: "alice@example.com", Role: "admin"},
				{Email: "bob@example.com", Role: "developer"},
				{Email: "charlie@example.com", Role: "viewer"},
				{Email: "zebra@example.com", Role: "operator"},
			}, nil
		},
	}
	logger := testutil.SilentLogger()
	runner := &mockRunner{}

	repos := database.Repositories{
		User:       repo,
		Execution:  &mockExecutionRepository{},
		Connection: &mockConnectionRepository{},
		Token:      &mockTokenRepository{},
		Image:      &mockImageRepository{},
		Secrets:    &mockSecretsRepository{},
	}
	service, err := NewService(context.Background(),
		testRegion,
		&repos,
		runner, // TaskManager
		runner, // ImageRegistry
		runner, // LogManager
		runner, // ObservabilityManager
		logger,
		"",
		defaultWebSocketManager,
		&stubHealthManager{},
		newPermissiveEnforcer(),
	)
	require.NoError(t, err)

	resp, err := service.ListUsers(context.Background())

	assert.NoError(t, err)
	assert.Len(t, resp.Users, 4)
	assert.Equal(t, "alice@example.com", resp.Users[0].Email)
	assert.Equal(t, "bob@example.com", resp.Users[1].Email)
	assert.Equal(t, "charlie@example.com", resp.Users[2].Email)
	assert.Equal(t, "zebra@example.com", resp.Users[3].Email)
}
