package health

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

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
