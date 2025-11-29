package authorization

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRole tests the NewRole constructor with various inputs
func TestNewRole(t *testing.T) {
	tests := []struct {
		name       string
		roleStr    string
		expectErr  bool
		expectRole Role
		errMsg     string
	}{
		{
			name:       "valid admin role",
			roleStr:    "admin",
			expectErr:  false,
			expectRole: RoleAdmin,
		},
		{
			name:       "valid operator role",
			roleStr:    "operator",
			expectErr:  false,
			expectRole: RoleOperator,
		},
		{
			name:       "valid developer role",
			roleStr:    "developer",
			expectErr:  false,
			expectRole: RoleDeveloper,
		},
		{
			name:       "valid viewer role",
			roleStr:    "viewer",
			expectErr:  false,
			expectRole: RoleViewer,
		},
		{
			name:       "empty string",
			roleStr:    "",
			expectErr:  true,
			expectRole: "",
			errMsg:     "role cannot be empty",
		},
		{
			name:       "invalid role",
			roleStr:    "superuser",
			expectErr:  true,
			expectRole: "",
			errMsg:     "invalid role: superuser",
		},
		{
			name:       "invalid role with lowercase check",
			roleStr:    "ADMIN",
			expectErr:  true,
			expectRole: "",
			errMsg:     "invalid role: ADMIN",
		},
		{
			name:       "role with whitespace",
			roleStr:    "admin ",
			expectErr:  true,
			expectRole: "",
			errMsg:     "invalid role: admin ",
		},
		{
			name:       "typo in role",
			roleStr:    "admmin",
			expectErr:  true,
			expectRole: "",
			errMsg:     "invalid role: admmin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, err := NewRole(tt.roleStr)

			if tt.expectErr {
				require.Error(t, err, "expected error but got none")
				assert.Contains(t, err.Error(), tt.errMsg, "error message should match")
				assert.Equal(t, tt.expectRole, role, "returned role should be empty on error")
			} else {
				require.NoError(t, err, "expected no error")
				assert.Equal(t, tt.expectRole, role, "role should match expected value")
			}
		})
	}
}

// TestRoleValid tests the Valid method on Role values
func TestRoleValid(t *testing.T) {
	tests := []struct {
		name        string
		role        Role
		expectValid bool
	}{
		{
			name:        "RoleAdmin is valid",
			role:        RoleAdmin,
			expectValid: true,
		},
		{
			name:        "RoleOperator is valid",
			role:        RoleOperator,
			expectValid: true,
		},
		{
			name:        "RoleDeveloper is valid",
			role:        RoleDeveloper,
			expectValid: true,
		},
		{
			name:        "RoleViewer is valid",
			role:        RoleViewer,
			expectValid: true,
		},
		{
			name:        "invalid role string",
			role:        Role("invalid"),
			expectValid: false,
		},
		{
			name:        "empty role",
			role:        Role(""),
			expectValid: false,
		},
		{
			name:        "uppercase role is invalid",
			role:        Role("ADMIN"),
			expectValid: false,
		},
		{
			name:        "role with typo",
			role:        Role("aadmin"),
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.role.Valid()
			assert.Equal(t, tt.expectValid, valid, "Valid() result should match expected")
		})
	}
}

// TestRoleString tests the String method on Role values
func TestRoleString(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		expected string
	}{
		{
			name:     "RoleAdmin",
			role:     RoleAdmin,
			expected: "admin",
		},
		{
			name:     "RoleOperator",
			role:     RoleOperator,
			expected: "operator",
		},
		{
			name:     "RoleDeveloper",
			role:     RoleDeveloper,
			expected: "developer",
		},
		{
			name:     "RoleViewer",
			role:     RoleViewer,
			expected: "viewer",
		},
		{
			name:     "custom role string",
			role:     Role("custom"),
			expected: "custom",
		},
		{
			name:     "empty role",
			role:     Role(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.role.String()
			assert.Equal(t, tt.expected, result, "String() should return string representation")
		})
	}
}

// TestFormatRole tests the FormatRole function
func TestFormatRole(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		expected string
	}{
		{
			name:     "format admin role",
			role:     RoleAdmin,
			expected: "role:admin",
		},
		{
			name:     "format operator role",
			role:     RoleOperator,
			expected: "role:operator",
		},
		{
			name:     "format developer role",
			role:     RoleDeveloper,
			expected: "role:developer",
		},
		{
			name:     "format viewer role",
			role:     RoleViewer,
			expected: "role:viewer",
		},
		{
			name:     "format custom role",
			role:     Role("custom"),
			expected: "role:custom",
		},
		{
			name:     "format empty role",
			role:     Role(""),
			expected: "role:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatRole(tt.role)
			assert.Equal(t, tt.expected, result, "FormatRole should return role:X format")
		})
	}
}

// TestFormatResourceID tests the FormatResourceID function
func TestFormatResourceID(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		resourceID   string
		expected     string
	}{
		{
			name:         "format secret resource",
			resourceType: "secret",
			resourceID:   "db-password",
			expected:     "secret:db-password",
		},
		{
			name:         "format image resource",
			resourceType: "image",
			resourceID:   "ubuntu:22.04",
			expected:     "image:ubuntu:22.04",
		},
		{
			name:         "format execution resource",
			resourceType: "execution",
			resourceID:   "exec-123",
			expected:     "execution:exec-123",
		},
		{
			name:         "format api endpoint",
			resourceType: "/api",
			resourceID:   "secrets",
			expected:     "/api:secrets",
		},
		{
			name:         "empty resource type",
			resourceType: "",
			resourceID:   "test",
			expected:     ":test",
		},
		{
			name:         "empty resource id",
			resourceType: "secret",
			resourceID:   "",
			expected:     "secret:",
		},
		{
			name:         "both empty",
			resourceType: "",
			resourceID:   "",
			expected:     ":",
		},
		{
			name:         "resource id with special characters",
			resourceType: "secret",
			resourceID:   "my-secret_v2.0",
			expected:     "secret:my-secret_v2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatResourceID(tt.resourceType, tt.resourceID)
			assert.Equal(t, tt.expected, result, "FormatResourceID should return type:id format")
		})
	}
}

// TestValidRoles tests the ValidRoles function
func TestValidRoles(t *testing.T) {
	roles := ValidRoles()

	// Should return exactly 4 roles
	require.Len(t, roles, 4, "ValidRoles should return 4 roles")

	// Check that all expected roles are present
	expectedRoles := []string{"admin", "operator", "developer", "viewer"}
	for _, expected := range expectedRoles {
		assert.Contains(t, roles, expected, "ValidRoles should contain %s", expected)
	}

	// Verify no duplicates
	assert.Equal(t, len(expectedRoles), len(roles), "ValidRoles should have no duplicates")
}

// TestIsValidRole tests the IsValidRole function
func TestIsValidRole(t *testing.T) {
	tests := []struct {
		name        string
		roleStr     string
		expectValid bool
	}{
		{
			name:        "valid admin role",
			roleStr:     "admin",
			expectValid: true,
		},
		{
			name:        "valid operator role",
			roleStr:     "operator",
			expectValid: true,
		},
		{
			name:        "valid developer role",
			roleStr:     "developer",
			expectValid: true,
		},
		{
			name:        "valid viewer role",
			roleStr:     "viewer",
			expectValid: true,
		},
		{
			name:        "invalid role",
			roleStr:     "superuser",
			expectValid: false,
		},
		{
			name:        "empty string",
			roleStr:     "",
			expectValid: false,
		},
		{
			name:        "uppercase role",
			roleStr:     "ADMIN",
			expectValid: false,
		},
		{
			name:        "role with whitespace",
			roleStr:     "admin ",
			expectValid: false,
		},
		{
			name:        "role substring",
			roleStr:     "admi",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := IsValidRole(tt.roleStr)
			assert.Equal(t, tt.expectValid, valid, "IsValidRole should return %v for %q", tt.expectValid, tt.roleStr)
		})
	}
}

// TestRoleConstants tests that role constants have expected values
func TestRoleConstants(t *testing.T) {
	assert.Equal(t, RoleAdmin, Role("admin"))
	assert.Equal(t, RoleOperator, Role("operator"))
	assert.Equal(t, RoleDeveloper, Role("developer"))
	assert.Equal(t, RoleViewer, Role("viewer"))
}

// TestActionConstants tests that action constants have expected values
func TestActionConstants(t *testing.T) {
	assert.Equal(t, ActionCreate, Action("create"))
	assert.Equal(t, ActionRead, Action("read"))
	assert.Equal(t, ActionUpdate, Action("update"))
	assert.Equal(t, ActionDelete, Action("delete"))
	assert.Equal(t, ActionKill, Action("kill"))
}

// TestRoleCreationAndValidation is an integration test showing typical usage patterns
func TestRoleCreationAndValidation(t *testing.T) {
	t.Run("create and use admin role", func(t *testing.T) {
		role, err := NewRole("admin")
		require.NoError(t, err)
		assert.True(t, role.Valid())
		assert.Equal(t, "admin", role.String())
		assert.Equal(t, "role:admin", FormatRole(role))
	})

	t.Run("create and use developer role", func(t *testing.T) {
		role, err := NewRole("developer")
		require.NoError(t, err)
		assert.True(t, role.Valid())
		assert.Equal(t, "developer", role.String())
		assert.Equal(t, "role:developer", FormatRole(role))
	})

	t.Run("fail to create invalid role", func(t *testing.T) {
		role, err := NewRole("invalid")
		require.Error(t, err)
		assert.False(t, role.Valid())
		assert.Contains(t, err.Error(), "invalid role")
		assert.Contains(t, err.Error(), "valid roles:")
	})
}

// TestResourceIDFormatting is an integration test for resource formatting
func TestResourceIDFormatting(t *testing.T) {
	tests := []struct {
		name          string
		resourceType  string
		resourceID    string
		casebinFormat string
	}{
		{
			name:          "secret resource",
			resourceType:  "secret",
			resourceID:    "api-key",
			casebinFormat: "secret:api-key",
		},
		{
			name:          "image resource",
			resourceType:  "image",
			resourceID:    "node:16",
			casebinFormat: "image:node:16",
		},
		{
			name:          "execution resource",
			resourceType:  "execution",
			resourceID:    "exec-uuid-1234",
			casebinFormat: "execution:exec-uuid-1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := FormatResourceID(tt.resourceType, tt.resourceID)
			assert.Equal(t, tt.casebinFormat, formatted)
		})
	}
}

// BenchmarkNewRole benchmarks role creation with validation
func BenchmarkNewRole(b *testing.B) {
	for b.Loop() {
		_, _ = NewRole("admin")
	}
}

// BenchmarkFormatRole benchmarks role formatting
func BenchmarkFormatRole(b *testing.B) {
	role := RoleAdmin
	for b.Loop() {
		_ = FormatRole(role)
	}
}

// BenchmarkFormatResourceID benchmarks resource ID formatting
func BenchmarkFormatResourceID(b *testing.B) {
	for b.Loop() {
		_ = FormatResourceID("secret", "my-secret")
	}
}

// BenchmarkIsValidRole benchmarks role validation
func BenchmarkIsValidRole(b *testing.B) {
	for b.Loop() {
		_ = IsValidRole("admin")
	}
}
