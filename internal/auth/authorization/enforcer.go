// Package authorization provides Casbin-based authorization enforcement for runvoy.
// It implements role-based access control (RBAC) with resource ownership.
package authorization

import (
	"bufio"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
)

// Enforcer wraps the Casbin enforcer with additional functionality.
type Enforcer struct {
	enforcer *casbin.Enforcer
	logger   *slog.Logger
}

// embeddedAdapter is a custom Casbin adapter that reads from an embedded filesystem.
type embeddedAdapter struct {
	modelFS  fs.FS
	pathBase string
}

// NewEmbeddedAdapter creates a new adapter for reading Casbin config from an embedded filesystem.
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

func (a *embeddedAdapter) SavePolicy(_ model.Model) error {
	return fmt.Errorf("SavePolicy not supported for embedded adapter")
}

func (a *embeddedAdapter) AddPolicy(_, _ string, _ []string) error {
	return fmt.Errorf("AddPolicy not supported for embedded adapter")
}

func (a *embeddedAdapter) RemovePolicy(_, _ string, _ []string) error {
	return fmt.Errorf("RemovePolicy not supported for embedded adapter")
}

func (a *embeddedAdapter) RemoveFilteredPolicy(_, _ string, _ int, _ ...string) error {
	return fmt.Errorf("RemoveFilteredPolicy not supported for embedded adapter")
}

func (a *embeddedAdapter) UpdatePolicy(_, _ string, _, _ []string) error {
	return fmt.Errorf("UpdatePolicy not supported for embedded adapter")
}

func (a *embeddedAdapter) UpdateFilteredPolicies(_, _ string, _ [][]string, _ int, _ ...string) error {
	return fmt.Errorf("UpdateFilteredPolicies not supported for embedded adapter")
}

// NewEnforcer creates a new Casbin enforcer using embedded Casbin configuration files.
// The model and policy are embedded in the binary at build time, so no filesystem access is required.
func NewEnforcer(logger *slog.Logger) (*Enforcer, error) {
	modelBytes, err := CasbinFS.ReadFile("casbin/model.conf")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded casbin model: %w", err)
	}

	m, err := model.NewModelFromString(string(modelBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse casbin model: %w", err)
	}

	adapter := NewEmbeddedAdapter(CasbinFS, "casbin/")
	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin authorization enforcer: %w", err)
	}

	logger.Debug("casbin authorization enforcer initialized")

	return &Enforcer{
		enforcer: enforcer,
		logger:   logger,
	}, nil
}

// Enforce checks if a subject (user) can perform an action on an object (resource).
// Returns true if the action is allowed, false otherwise.
//
// Example usage:
//
//	allowed, err := e.Enforce("user@example.com", "/api/secrets/secret-123", "read")
func (e *Enforcer) Enforce(subject, object, action string) (bool, error) {
	allowed, err := e.enforcer.Enforce(subject, object, action)
	if err != nil {
		e.logger.Error("casbin enforcement error", "subject", subject, "object", object, "action", action, "error", err)
		return false, fmt.Errorf("casbin enforcement failed: %w", err)
	}

	e.logger.Debug("casbin enforcement result", "subject", subject, "object", object, "action", action, "allowed", allowed)
	return allowed, nil
}

// AddRoleForUser assigns a role to a user.
// The role should be in the format "role:admin", "role:operator", etc.
//
// Example usage:
//
//	err := e.AddRoleForUser("user@example.com", "role:developer")
func (e *Enforcer) AddRoleForUser(user, role string) error {
	added, err := e.enforcer.AddGroupingPolicy(user, role)
	if err != nil {
		return fmt.Errorf("failed to add role for user: %w", err)
	}
	if !added {
		e.logger.Debug("role already exists for user", "user", user, "role", role)
		return nil
	}

	e.logger.Info("role added for user", "user", user, "role", role)
	return nil
}

// RemoveRoleForUser removes a role from a user.
//
// Example usage:
//
//	err := e.RemoveRoleForUser("user@example.com", "role:developer")
func (e *Enforcer) RemoveRoleForUser(user, role string) error {
	removed, err := e.enforcer.RemoveGroupingPolicy(user, role)
	if err != nil {
		return fmt.Errorf("failed to remove role for user: %w", err)
	}
	if !removed {
		e.logger.Debug("role did not exist for user", "user", user, "role", role)
		return nil
	}

	e.logger.Info("role removed for user", "user", user, "role", role)
	return nil
}

// AddOwnershipForResource adds ownership mapping for a resource.
// This allows the owner to access their own resources.
//
// Example usage:
//
//	err := e.AddOwnershipForResource("secret:secret-123", "user@example.com")
func (e *Enforcer) AddOwnershipForResource(resourceID, ownerEmail string) error {
	added, err := e.enforcer.AddGroupingPolicy(resourceID, ownerEmail)
	if err != nil {
		return fmt.Errorf("failed to add ownership for resource: %w", err)
	}
	if !added {
		e.logger.Debug("ownership already exists for resource", "resource", resourceID, "owner", ownerEmail)
		return nil
	}

	e.logger.Debug("ownership added for resource", "resource", resourceID, "owner", ownerEmail)
	return nil
}

// RemoveOwnershipForResource removes ownership mapping for a resource.
//
// Example usage:
//
//	err := e.RemoveOwnershipForResource("secret:secret-123", "user@example.com")
func (e *Enforcer) RemoveOwnershipForResource(resourceID, ownerEmail string) error {
	removed, err := e.enforcer.RemoveGroupingPolicy(resourceID, ownerEmail)
	if err != nil {
		return fmt.Errorf("failed to remove ownership for resource: %w", err)
	}
	if !removed {
		e.logger.Debug("ownership did not exist for resource", "resource", resourceID, "owner", ownerEmail)
		return nil
	}

	e.logger.Debug("ownership removed for resource", "resource", resourceID, "owner", ownerEmail)
	return nil
}

// LoadRolesForUsers loads role assignments for multiple users into the enforcer.
// This is typically called at startup to initialize the enforcer with current user roles.
//
// Example usage:
//
//	roles := map[string]string{
//	  "admin@example.com": "role:admin",
//	  "dev@example.com": "role:developer",
//	}
//	err := e.LoadRolesForUsers(roles)
func (e *Enforcer) LoadRolesForUsers(userRoles map[string]string) error {
	for user, role := range userRoles {
		if err := e.AddRoleForUser(user, role); err != nil {
			return fmt.Errorf("failed to load role for user %s: %w", user, err)
		}
	}

	e.logger.Info("loaded user roles", "count", len(userRoles))
	return nil
}

// LoadResourceOwnerships loads resource ownership mappings into the enforcer.
// This is typically called at startup to initialize the enforcer with current ownerships.
//
// Example usage:
//
//	ownerships := map[string]string{
//	  "secret:secret-123": "user@example.com",
//	  "execution:exec-456": "user@example.com",
//	}
//	err := e.LoadResourceOwnerships(ownerships)
func (e *Enforcer) LoadResourceOwnerships(ownerships map[string]string) error {
	for resourceID, ownerEmail := range ownerships {
		if err := e.AddOwnershipForResource(resourceID, ownerEmail); err != nil {
			return fmt.Errorf("failed to load ownership for resource %s: %w", resourceID, err)
		}
	}

	e.logger.Info("loaded resource ownerships", "count", len(ownerships))
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
