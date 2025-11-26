package health

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
)

type stubUserRepo struct {
	users   []*api.User
	listErr error
}

func (s *stubUserRepo) CreateUser(context.Context, *api.User, string, int64) error {
	return errors.New("not implemented")
}
func (s *stubUserRepo) RemoveExpiration(context.Context, string) error {
	return errors.New("not implemented")
}
func (s *stubUserRepo) GetUserByEmail(context.Context, string) (*api.User, error) {
	return nil, errors.New("not implemented")
}
func (s *stubUserRepo) GetUserByAPIKeyHash(context.Context, string) (*api.User, error) {
	return nil, errors.New("not implemented")
}
func (s *stubUserRepo) UpdateLastUsed(context.Context, string) (*api.User, error) {
	return nil, errors.New("not implemented")
}
func (s *stubUserRepo) RevokeUser(context.Context, string) error {
	return errors.New("not implemented")
}
func (s *stubUserRepo) CreatePendingAPIKey(context.Context, *api.PendingAPIKey) error {
	return errors.New("not implemented")
}
func (s *stubUserRepo) GetPendingAPIKey(context.Context, string) (*api.PendingAPIKey, error) {
	return nil, errors.New("not implemented")
}
func (s *stubUserRepo) MarkAsViewed(context.Context, string, string) error {
	return errors.New("not implemented")
}
func (s *stubUserRepo) DeletePendingAPIKey(context.Context, string) error {
	return errors.New("not implemented")
}
func (s *stubUserRepo) ListUsers(context.Context) ([]*api.User, error) {
	return s.users, s.listErr
}
func (s *stubUserRepo) GetUsersByRequestID(context.Context, string) ([]*api.User, error) {
	return nil, errors.New("not implemented")
}

type stubSecretsRepo struct {
	secrets []*api.Secret
}

func (s *stubSecretsRepo) CreateSecret(context.Context, *api.Secret) error {
	return errors.New("not implemented")
}
func (s *stubSecretsRepo) GetSecret(context.Context, string, bool) (*api.Secret, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSecretsRepo) ListSecrets(context.Context, bool) ([]*api.Secret, error) {
	return s.secrets, nil
}
func (s *stubSecretsRepo) UpdateSecret(context.Context, *api.Secret) error {
	return errors.New("not implemented")
}
func (s *stubSecretsRepo) DeleteSecret(context.Context, string) error {
	return errors.New("not implemented")
}
func (s *stubSecretsRepo) GetSecretsByRequestID(context.Context, string) ([]*api.Secret, error) {
	return nil, errors.New("not implemented")
}

type stubExecutionRepo struct {
	executions []*api.Execution
}

func (s *stubExecutionRepo) CreateExecution(context.Context, *api.Execution) error {
	return errors.New("not implemented")
}
func (s *stubExecutionRepo) GetExecution(context.Context, string) (*api.Execution, error) {
	return nil, errors.New("not implemented")
}
func (s *stubExecutionRepo) UpdateExecution(context.Context, *api.Execution) error {
	return errors.New("not implemented")
}
func (s *stubExecutionRepo) ListExecutions(context.Context, int, []string) ([]*api.Execution, error) {
	return s.executions, nil
}
func (s *stubExecutionRepo) GetExecutionsByRequestID(context.Context, string) ([]*api.Execution, error) {
	return nil, errors.New("not implemented")
}

type stubImageRepo struct {
	images []api.ImageInfo
}

func (s *stubImageRepo) ListImages(context.Context) ([]api.ImageInfo, error) {
	return s.images, nil
}

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
		status := &api.AuthorizerHealthStatus{}
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
