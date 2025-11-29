package authorization

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Role is a typed string representing a user role in the authorization system.
// Valid roles: admin, operator, developer, viewer.
type Role string

// Role constants for Casbin role-based access control.
// These correspond to the roles defined in casbin/policy.csv.
const (
	// RoleAdmin has full access to all resources and operations.
	RoleAdmin Role = "admin"

	// RoleOperator can manage images, secrets, and executions but cannot manage users.
	RoleOperator Role = "operator"

	// RoleDeveloper can create and manage their own resources and execute commands.
	RoleDeveloper Role = "developer"

	// RoleViewer has read-only access to executions.
	RoleViewer Role = "viewer"
)

// Action is a typed string representing an action in the authorization system.
type Action string

// Action constants for Casbin enforcement.
// These correspond to the HTTP methods mapped to CRUD actions.
const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionKill   Action = "kill"
	ActionUse    Action = "use"
)

// NewRole creates a new Role from a string, validating it against known roles.
// Returns an error if the role string is empty or not a valid role.
func NewRole(roleStr string) (Role, error) {
	if roleStr == "" {
		return "", errors.New("role cannot be empty")
	}
	role := Role(roleStr)
	if !role.Valid() {
		return "", fmt.Errorf("invalid role: %s (valid roles: %s)",
			roleStr, strings.Join(ValidRoles(), ", "))
	}
	return role, nil
}

// Valid checks if the role is a valid known role.
func (r Role) Valid() bool {
	return slices.Contains([]Role{RoleAdmin, RoleOperator, RoleDeveloper, RoleViewer}, r)
}

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}

// FormatRole converts a role to the Casbin role format.
// Example: FormatRole(RoleAdmin) returns "role:admin".
func FormatRole(role Role) string {
	return "role:" + role.String()
}

// FormatResourceID converts a resource type and ID to the Casbin resource format.
// Example: FormatResourceID("secret", "secret-123") returns "secret:secret-123".
func FormatResourceID(resourceType, resourceID string) string {
	return resourceType + ":" + resourceID
}

// ValidRoles returns a list of all valid role names as strings.
func ValidRoles() []string {
	return []string{RoleAdmin.String(), RoleOperator.String(), RoleDeveloper.String(), RoleViewer.String()}
}

// IsValidRole checks if a role name string is valid.
func IsValidRole(roleStr string) bool {
	role := Role(roleStr)
	return role.Valid()
}
