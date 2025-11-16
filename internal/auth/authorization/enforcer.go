// Package authorization provides Casbin-based authorization enforcement for runvoy.
// It implements role-based access control (RBAC) with resource ownership.
package authorization

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"log/slog"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

//go:embed ../casbin/model.conf
var modelConfig string

//go:embed ../casbin/policy.csv
var policyConfig string

// Enforcer wraps the Casbin enforcer with additional functionality.
type Enforcer struct {
	enforcer *casbin.Enforcer
	logger   *slog.Logger
}

// NewEnforcer creates a new Casbin enforcer with embedded model and policy configurations.
// The model and policy are embedded into the binary at compile time, making the enforcer
// portable and suitable for Lambda deployments.
func NewEnforcer(logger *slog.Logger) (*Enforcer, error) {
	// Create model from embedded configuration string
	m, err := model.NewModelFromString(modelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin model from embedded config: %w", err)
	}

	// Create enforcer with model (no file adapter - policies loaded in-memory)
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	// Load policies from embedded CSV
	if err := loadPoliciesFromCSV(e, policyConfig); err != nil {
		return nil, fmt.Errorf("failed to load policies from embedded CSV: %w", err)
	}

	logger.Debug("casbin authorization enforcer initialized with embedded configuration")

	return &Enforcer{
		enforcer: e,
		logger:   logger,
	}, nil
}

// loadPoliciesFromCSV parses the embedded policy CSV and loads policies into the enforcer.
func loadPoliciesFromCSV(e *casbin.Enforcer, csvContent string) error {
	reader := csv.NewReader(strings.NewReader(csvContent))
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to parse policy CSV: %w", err)
	}

	for i, record := range records {
		if len(record) == 0 {
			continue // Skip empty lines
		}

		// Skip comment lines
		if strings.HasPrefix(strings.TrimSpace(record[0]), "#") {
			continue
		}

		// Policy format: p, role:admin, /api/*, *, allow
		// Grouping format: g, user@example.com, role:admin
		policyType := strings.TrimSpace(record[0])

		switch policyType {
		case "p":
			// Standard policy: p, sub, obj, act, eft
			if len(record) < 5 {
				return fmt.Errorf("invalid policy at line %d: expected at least 5 fields, got %d", i+1, len(record))
			}
			params := make([]string, len(record)-1)
			for j := 1; j < len(record); j++ {
				params[j-1] = strings.TrimSpace(record[j])
			}
			if _, err := e.AddPolicy(params); err != nil {
				return fmt.Errorf("failed to add policy at line %d: %w", i+1, err)
			}

		case "g", "g2":
			// Grouping policy: g, member, role or g2, resource, owner
			if len(record) < 3 {
				return fmt.Errorf("invalid grouping policy at line %d: expected at least 3 fields, got %d", i+1, len(record))
			}
			params := make([]string, len(record)-1)
			for j := 1; j < len(record); j++ {
				params[j-1] = strings.TrimSpace(record[j])
			}
			if _, err := e.AddNamedGroupingPolicy(policyType, params); err != nil {
				return fmt.Errorf("failed to add grouping policy at line %d: %w", i+1, err)
			}

		default:
			return fmt.Errorf("unknown policy type '%s' at line %d", policyType, i+1)
		}
	}

	return nil
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
