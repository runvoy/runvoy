// Package authorization provides Casbin-based authorization enforcement for runvoy.
// It implements role-based access control (RBAC) with resource ownership support.
package authorization

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"

	"runvoy/internal/logger"
)

// Enforcer wraps the Casbin enforcer with additional functionality.
type Enforcer struct {
	enforcer casbin.IEnforcer
	logger   *slog.Logger
}

// embeddedAdapter is a custom Casbin adapter that reads from an embedded filesystem
// and supports in-memory policy additions during runtime (without persisting back to the embedded file).
type embeddedAdapter struct {
	modelFS  fs.FS
	pathBase string
}

// NewEmbeddedAdapter creates a new adapter for reading Casbin config from an embedded filesystem.
// The adapter supports in-memory policy modifications at runtime but does not persist changes
// back to the embedded files, as they are read-only.
func NewEmbeddedAdapter(fsys fs.FS, pathBase string) persist.Adapter {
	return &embeddedAdapter{
		modelFS:  fsys,
		pathBase: pathBase,
	}
}

// LoadPolicy loads the policy from the embedded filesystem.
func (a *embeddedAdapter) LoadPolicy(m model.Model) error {
	policyPath := strings.TrimPrefix(a.pathBase, "casbin/") + "policy.csv"
	data, err := fs.ReadFile(a.modelFS, a.pathBase+policyPath)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if lineErr := persist.LoadPolicyLine(line, m); lineErr != nil {
			return fmt.Errorf("failed to load policy line: %w", lineErr)
		}
	}

	return nil
}

// SavePolicy is a no-op for the embedded adapter as it doesn't persist changes.
// Casbin calls this after policy modifications, but we don't need to persist back to the embedded files.
func (a *embeddedAdapter) SavePolicy(_ model.Model) error {
	return nil
}

// AddPolicy is a no-op for the embedded adapter. Changes are kept in Casbin's in-memory model
// but not persisted back to the embedded policy file (which is read-only).
func (a *embeddedAdapter) AddPolicy(_, _ string, _ []string) error {
	return nil
}

// RemovePolicy is a no-op for the embedded adapter. Changes are kept in Casbin's in-memory model
// but not persisted back to the embedded policy file.
func (a *embeddedAdapter) RemovePolicy(_, _ string, _ []string) error {
	return nil
}

// RemoveFilteredPolicy is a no-op for the embedded adapter. Changes are kept in Casbin's in-memory model.
func (a *embeddedAdapter) RemoveFilteredPolicy(_, _ string, _ int, _ ...string) error {
	return nil
}

// UpdatePolicy is a no-op for the embedded adapter. Changes are kept in Casbin's in-memory model.
func (a *embeddedAdapter) UpdatePolicy(_, _ string, _, _ []string) error {
	return nil
}

// UpdateFilteredPolicies is a no-op for the embedded adapter. Changes are kept in Casbin's in-memory model.
func (a *embeddedAdapter) UpdateFilteredPolicies(_, _ string, _ [][]string, _ int, _ ...string) error {
	return nil
}

// NewEnforcer creates a new Casbin enforcer using embedded Casbin configuration files.
// The model and policy are embedded in the binary at build time, so no filesystem access is required.
func NewEnforcer(log *slog.Logger) (*Enforcer, error) {
	modelBytes, err := CasbinFS.ReadFile("casbin/model.conf")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded casbin model: %w", err)
	}

	m, err := model.NewModelFromString(string(modelBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse casbin model: %w", err)
	}

	adapter := NewEmbeddedAdapter(CasbinFS, "casbin/")
	enforcer, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin authorization enforcer: %w", err)
	}

	log.Debug("initializing casbin authorization enforcer")

	return &Enforcer{
		enforcer: enforcer,
		logger:   log,
	}, nil
}

// Enforce checks if a subject (user) can perform an action on an object (resource).
// Returns true if the action is allowed, false otherwise.
//
// Example usage:
//
//	allowed, err := e.Enforce(ctx, "user@example.com", "/api/secrets/secret-123", "read")
func (e *Enforcer) Enforce(ctx context.Context, subject, object string, action Action) (bool, error) {
	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	allowed, err := e.enforcer.Enforce(subject, object, string(action))
	if err != nil {
		reqLogger.Error("casbin enforcement error", "context", map[string]any{
			"subject": subject,
			"object":  object,
			"action":  action,
			"error":   err,
		})
		return false, fmt.Errorf("casbin enforcement failed: %w", err)
	}

	reqLogger.Debug("casbin enforcement result", "context",
		map[string]any{
			"subject": subject,
			"object":  object,
			"action":  action,
			"allowed": allowed,
		})
	return allowed, nil
}

// AddRoleForUser assigns a role to a user.
// Returns an error if the role is invalid or empty.
//
// Example usage:
//
//	err := e.AddRoleForUser(ctx, "user@example.com", RoleDeveloper)
func (e *Enforcer) AddRoleForUser(ctx context.Context, user string, role Role) error {
	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	if !role.Valid() {
		return fmt.Errorf("invalid role for user %s: %s (valid roles: %s)",
			user, role, strings.Join(ValidRoles(), ", "))
	}

	formattedRole := FormatRole(role)
	added, err := e.enforcer.AddGroupingPolicy(user, formattedRole)
	if err != nil {
		return fmt.Errorf("failed to add role for user: %w", err)
	}
	if !added {
		reqLogger.Debug("role already exists for user", "user", user, "role", role)
		return nil
	}

	reqLogger.Debug("role added for user", "user", user, "role", role)
	return nil
}

// RemoveRoleForUser removes a role from a user.
//
// Example usage:
//
//	err := e.RemoveRoleForUser(ctx, "user@example.com", "role:developer")
func (e *Enforcer) RemoveRoleForUser(ctx context.Context, user, role string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	removed, err := e.enforcer.RemoveGroupingPolicy(user, role)
	if err != nil {
		return fmt.Errorf("failed to remove role for user: %w", err)
	}
	if !removed {
		reqLogger.Debug("role did not exist for user", "user", user, "role", role)
		return nil
	}

	reqLogger.Info("role removed for user", "user", user, "role", role)
	return nil
}

// AddOwnershipForResource adds ownership mapping for a resource.
// This allows the owner to access their own resources.
//
// Example usage:
//
//	err := e.AddOwnershipForResource(ctx, "secret:secret-123", "user@example.com")
func (e *Enforcer) AddOwnershipForResource(ctx context.Context, resourceID, ownerEmail string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	added, err := e.enforcer.AddNamedGroupingPolicy("g2", resourceID, ownerEmail)
	if err != nil {
		return fmt.Errorf("failed to add ownership for resource: %w", err)
	}
	if !added {
		reqLogger.Debug("ownership already exists for resource", "resource", resourceID, "owner", ownerEmail)
		return nil
	}

	reqLogger.Debug("ownership added for resource", "resource", resourceID, "owner", ownerEmail)
	return nil
}

// RemoveOwnershipForResource removes ownership mapping for a resource.
func (e *Enforcer) RemoveOwnershipForResource(ctx context.Context, resourceID, ownerEmail string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	removed, err := e.enforcer.RemoveNamedGroupingPolicy("g2", resourceID, ownerEmail)
	if err != nil {
		return fmt.Errorf("failed to remove ownership for resource: %w", err)
	}
	if !removed {
		reqLogger.Debug("ownership did not exist for resource", "resource", resourceID, "owner", ownerEmail)
		return nil
	}

	reqLogger.Debug("ownership removed for resource", "resource", resourceID, "owner", ownerEmail)
	return nil
}

// RemoveAllOwnershipsForResource removes every ownership mapping for the given resource identifier.
// This is useful when deleting a resource without knowing its owner ahead of time.
func (e *Enforcer) RemoveAllOwnershipsForResource(ctx context.Context, resourceID string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	if _, err := e.enforcer.RemoveFilteredNamedGroupingPolicy("g2", 0, resourceID); err != nil {
		return fmt.Errorf("failed to remove ownerships for resource %s: %w", resourceID, err)
	}
	reqLogger.Debug("ownerships removed for resource", "resource", resourceID)
	return nil
}

// HasOwnershipForResource checks if the provided user currently owns the resource.
func (e *Enforcer) HasOwnershipForResource(resourceID, ownerEmail string) (bool, error) {
	hasOwnership, err := e.enforcer.HasNamedGroupingPolicy("g2", resourceID, ownerEmail)
	if err != nil {
		return false, fmt.Errorf("failed to check ownership for resource %s: %w", resourceID, err)
	}
	return hasOwnership, nil
}

// LoadResourceOwnerships loads resource ownership mappings into the enforcer.
func (e *Enforcer) LoadResourceOwnerships(ctx context.Context, ownerships map[string]string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	for resourceID, ownerEmail := range ownerships {
		if err := e.AddOwnershipForResource(ctx, resourceID, ownerEmail); err != nil {
			return fmt.Errorf("failed to load ownership for resource %s: %w", resourceID, err)
		}
	}

	reqLogger.Info("loaded resource ownerships", "count", len(ownerships))
	return nil
}

// LoadRolesForUsers loads role assignments for multiple users into the enforcer.
// This is typically called at startup to initialize the enforcer with current user roles.
// The roleStr values should be valid role names (admin, operator, developer, viewer).
//
// Example usage:
//
//	roles := map[string]string{
//	  "admin@example.com": "admin",
//	  "dev@example.com": "developer",
//	}
//	err := e.LoadRolesForUsers(ctx, roles)
func (e *Enforcer) LoadRolesForUsers(ctx context.Context, userRoles map[string]string) error {
	reqLogger := logger.DeriveRequestLogger(ctx, e.logger)

	for user, roleStr := range userRoles {
		role, err := NewRole(roleStr)
		if err != nil {
			return fmt.Errorf("failed to load role for user %s: %w", user, err)
		}
		if addErr := e.AddRoleForUser(ctx, user, role); addErr != nil {
			return fmt.Errorf("failed to add role for user %s to enforcer: %w", user, addErr)
		}
	}

	reqLogger.Debug("loaded user roles", "count", len(userRoles))
	return nil
}

// GetRolesForUser returns all roles assigned to a user.
func (e *Enforcer) GetRolesForUser(user string) ([]string, error) {
	roles, err := e.enforcer.GetRolesForUser(user)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user: %w", err)
	}
	return roles, nil
}

// GetAllNamedGroupingPolicies returns all grouping policies for the given policy type (e.g., "g2" for ownership).
func (e *Enforcer) GetAllNamedGroupingPolicies(ptype string) ([][]string, error) {
	policies, err := e.enforcer.GetNamedGroupingPolicy(ptype)
	if err != nil {
		return nil, fmt.Errorf("failed to get named grouping policies for %s: %w", ptype, err)
	}
	return policies, nil
}
