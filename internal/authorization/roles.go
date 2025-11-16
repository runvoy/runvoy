package authorization

// Role constants for Casbin role-based access control.
// These correspond to the roles defined in casbin/policy.csv.
const (
	// RoleAdmin has full access to all resources and operations.
	RoleAdmin = "admin"

	// RoleOperator can manage images, secrets, and executions but cannot manage users.
	RoleOperator = "operator"

	// RoleDeveloper can create and manage their own resources and execute commands.
	RoleDeveloper = "developer"

	// RoleViewer has read-only access to executions.
	RoleViewer = "viewer"
)

// Action constants for Casbin enforcement.
// These correspond to the HTTP methods mapped to CRUD actions.
const (
	ActionCreate  = "create"
	ActionRead    = "read"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionExecute = "execute"
	ActionKill    = "kill"
)

// FormatRole converts a role name to the Casbin role format.
// Example: FormatRole("admin") returns "role:admin"
func FormatRole(role string) string {
	return "role:" + role
}

// FormatResourceID converts a resource type and ID to the Casbin resource format.
// Example: FormatResourceID("secret", "secret-123") returns "secret:secret-123"
func FormatResourceID(resourceType, resourceID string) string {
	return resourceType + ":" + resourceID
}

// ValidRoles returns a list of all valid role names.
func ValidRoles() []string {
	return []string{RoleAdmin, RoleOperator, RoleDeveloper, RoleViewer}
}

// IsValidRole checks if a role name is valid.
func IsValidRole(role string) bool {
	for _, validRole := range ValidRoles() {
		if role == validRole {
			return true
		}
	}
	return false
}
