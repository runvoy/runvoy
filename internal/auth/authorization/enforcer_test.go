package authorization

import (
	"log/slog"
	"os"
	"testing"
)

// Helper function to create a test enforcer
func createTestEnforcer(t *testing.T) *Enforcer {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	enforcer, err := NewEnforcer(logger)
	if err != nil {
		t.Fatalf("Failed to create test enforcer: %v", err)
	}
	return enforcer
}

func TestNewEnforcer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("successful creation", func(t *testing.T) {
		enforcer, err := NewEnforcer(logger)
		if err != nil {
			t.Fatalf("NewEnforcer() error = %v, want nil", err)
		}
		if enforcer == nil {
			t.Fatal("NewEnforcer() returned nil enforcer")
		}
		if enforcer.enforcer == nil {
			t.Fatal("NewEnforcer() returned enforcer with nil internal enforcer")
		}
		if enforcer.logger == nil {
			t.Fatal("NewEnforcer() returned enforcer with nil logger")
		}
	})
}

func TestAddRoleForUser(t *testing.T) {
	tests := []struct {
		name      string
		user      string
		role      Role
		wantError bool
		errorMsg  string
	}{
		{
			name:      "add admin role",
			user:      "admin@example.com",
			role:      RoleAdmin,
			wantError: false,
		},
		{
			name:      "add operator role",
			user:      "operator@example.com",
			role:      RoleOperator,
			wantError: false,
		},
		{
			name:      "add developer role",
			user:      "dev@example.com",
			role:      RoleDeveloper,
			wantError: false,
		},
		{
			name:      "add viewer role",
			user:      "viewer@example.com",
			role:      RoleViewer,
			wantError: false,
		},
		{
			name:      "add invalid role",
			user:      "user@example.com",
			role:      Role("invalid"),
			wantError: true,
			errorMsg:  "invalid role",
		},
		{
			name:      "add empty role",
			user:      "user@example.com",
			role:      Role(""),
			wantError: true,
			errorMsg:  "invalid role",
		},
		{
			name:      "add duplicate role",
			user:      "dup@example.com",
			role:      RoleAdmin,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := createTestEnforcer(t)

			// For duplicate test, add the role twice
			if tt.name == "add duplicate role" {
				if err := e.AddRoleForUser(tt.user, tt.role); err != nil {
					t.Fatalf("First AddRoleForUser() failed: %v", err)
				}
			}

			err := e.AddRoleForUser(tt.user, tt.role)

			if tt.wantError {
				if err == nil {
					t.Errorf("AddRoleForUser() error = nil, want error containing %q", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("AddRoleForUser() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("AddRoleForUser() error = %v, want nil", err)
				}

				// Verify role was added
				roles, err := e.GetRolesForUser(tt.user)
				if err != nil {
					t.Fatalf("GetRolesForUser() failed: %v", err)
				}
				expectedRole := FormatRole(tt.role)
				if !containsString(roles, expectedRole) {
					t.Errorf("GetRolesForUser() = %v, want to contain %q", roles, expectedRole)
				}
			}
		})
	}
}

func TestRemoveRoleForUser(t *testing.T) {
	tests := []struct {
		name         string
		user         string
		roleToAdd    Role
		roleToRemove string
		wantError    bool
	}{
		{
			name:         "remove existing role",
			user:         "user1@example.com",
			roleToAdd:    RoleAdmin,
			roleToRemove: "role:admin",
			wantError:    false,
		},
		{
			name:         "remove non-existent role",
			user:         "user2@example.com",
			roleToAdd:    RoleAdmin,
			roleToRemove: "role:developer",
			wantError:    false,
		},
		{
			name:         "remove role from user with no roles",
			user:         "user3@example.com",
			roleToAdd:    Role(""), // Don't add any role
			roleToRemove: "role:admin",
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := createTestEnforcer(t)

			// Add role if specified
			if tt.roleToAdd != "" {
				if err := e.AddRoleForUser(tt.user, tt.roleToAdd); err != nil {
					t.Fatalf("AddRoleForUser() failed: %v", err)
				}
			}

			err := e.RemoveRoleForUser(tt.user, tt.roleToRemove)

			if tt.wantError {
				if err == nil {
					t.Errorf("RemoveRoleForUser() error = nil, want error")
				}
			} else {
				if err != nil {
					t.Errorf("RemoveRoleForUser() error = %v, want nil", err)
				}

				// Verify role was removed
				roles, err := e.GetRolesForUser(tt.user)
				if err != nil {
					t.Fatalf("GetRolesForUser() failed: %v", err)
				}
				if containsString(roles, tt.roleToRemove) {
					t.Errorf("GetRolesForUser() = %v, should not contain %q", roles, tt.roleToRemove)
				}
			}
		})
	}
}

func TestGetRolesForUser(t *testing.T) {
	e := createTestEnforcer(t)

	t.Run("user with no roles", func(t *testing.T) {
		roles, err := e.GetRolesForUser("noone@example.com")
		if err != nil {
			t.Fatalf("GetRolesForUser() error = %v, want nil", err)
		}
		if len(roles) != 0 {
			t.Errorf("GetRolesForUser() = %v, want empty slice", roles)
		}
	})

	t.Run("user with one role", func(t *testing.T) {
		user := "single@example.com"
		if err := e.AddRoleForUser(user, RoleAdmin); err != nil {
			t.Fatalf("AddRoleForUser() failed: %v", err)
		}

		roles, err := e.GetRolesForUser(user)
		if err != nil {
			t.Fatalf("GetRolesForUser() error = %v, want nil", err)
		}
		if len(roles) != 1 {
			t.Errorf("GetRolesForUser() returned %d roles, want 1", len(roles))
		}
		if roles[0] != "role:admin" {
			t.Errorf("GetRolesForUser() = %v, want [role:admin]", roles)
		}
	})

	t.Run("user with multiple roles", func(t *testing.T) {
		user := "multi@example.com"
		if err := e.AddRoleForUser(user, RoleAdmin); err != nil {
			t.Fatalf("AddRoleForUser(admin) failed: %v", err)
		}
		if err := e.AddRoleForUser(user, RoleDeveloper); err != nil {
			t.Fatalf("AddRoleForUser(developer) failed: %v", err)
		}

		roles, err := e.GetRolesForUser(user)
		if err != nil {
			t.Fatalf("GetRolesForUser() error = %v, want nil", err)
		}
		if len(roles) != 2 {
			t.Errorf("GetRolesForUser() returned %d roles, want 2", len(roles))
		}
	})
}

func TestAddOwnershipForResource(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		ownerEmail string
		wantError  bool
	}{
		{
			name:       "add secret ownership",
			resourceID: "secret:db-password",
			ownerEmail: "owner@example.com",
			wantError:  false,
		},
		{
			name:       "add execution ownership",
			resourceID: "execution:exec-123",
			ownerEmail: "dev@example.com",
			wantError:  false,
		},
		{
			name:       "add image ownership",
			resourceID: "image:img-456",
			ownerEmail: "admin@example.com",
			wantError:  false,
		},
		{
			name:       "add duplicate ownership",
			resourceID: "secret:dup-secret",
			ownerEmail: "dup@example.com",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := createTestEnforcer(t)

			// For duplicate test, add ownership twice
			if tt.name == "add duplicate ownership" {
				if err := e.AddOwnershipForResource(tt.resourceID, tt.ownerEmail); err != nil {
					t.Fatalf("First AddOwnershipForResource() failed: %v", err)
				}
			}

			err := e.AddOwnershipForResource(tt.resourceID, tt.ownerEmail)

			if tt.wantError {
				if err == nil {
					t.Errorf("AddOwnershipForResource() error = nil, want error")
				}
			} else {
				if err != nil {
					t.Errorf("AddOwnershipForResource() error = %v, want nil", err)
				}

				// Verify ownership was added
				hasOwnership, err := e.HasOwnershipForResource(tt.resourceID, tt.ownerEmail)
				if err != nil {
					t.Fatalf("HasOwnershipForResource() failed: %v", err)
				}
				if !hasOwnership {
					t.Errorf("HasOwnershipForResource() = false, want true")
				}
			}
		})
	}
}

func TestRemoveOwnershipForResource(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		ownerEmail string
		addFirst   bool
		wantError  bool
	}{
		{
			name:       "remove existing ownership",
			resourceID: "secret:test-secret",
			ownerEmail: "owner@example.com",
			addFirst:   true,
			wantError:  false,
		},
		{
			name:       "remove non-existent ownership",
			resourceID: "secret:nonexistent",
			ownerEmail: "nobody@example.com",
			addFirst:   false,
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := createTestEnforcer(t)

			if tt.addFirst {
				if err := e.AddOwnershipForResource(tt.resourceID, tt.ownerEmail); err != nil {
					t.Fatalf("AddOwnershipForResource() failed: %v", err)
				}
			}

			err := e.RemoveOwnershipForResource(tt.resourceID, tt.ownerEmail)

			if tt.wantError {
				if err == nil {
					t.Errorf("RemoveOwnershipForResource() error = nil, want error")
				}
			} else {
				if err != nil {
					t.Errorf("RemoveOwnershipForResource() error = %v, want nil", err)
				}

				// Verify ownership was removed
				hasOwnership, err := e.HasOwnershipForResource(tt.resourceID, tt.ownerEmail)
				if err != nil {
					t.Fatalf("HasOwnershipForResource() failed: %v", err)
				}
				if hasOwnership {
					t.Errorf("HasOwnershipForResource() = true, want false")
				}
			}
		})
	}
}

func TestRemoveAllOwnershipsForResource(t *testing.T) {
	e := createTestEnforcer(t)
	resourceID := "secret:multi-owner"

	// Add multiple owners
	owners := []string{"owner1@example.com", "owner2@example.com", "owner3@example.com"}
	for _, owner := range owners {
		if err := e.AddOwnershipForResource(resourceID, owner); err != nil {
			t.Fatalf("AddOwnershipForResource() failed for %s: %v", owner, err)
		}
	}

	// Verify all ownerships exist
	for _, owner := range owners {
		hasOwnership, err := e.HasOwnershipForResource(resourceID, owner)
		if err != nil {
			t.Fatalf("HasOwnershipForResource() failed: %v", err)
		}
		if !hasOwnership {
			t.Errorf("Expected ownership for %s before removal", owner)
		}
	}

	// Remove all ownerships
	err := e.RemoveAllOwnershipsForResource(resourceID)
	if err != nil {
		t.Fatalf("RemoveAllOwnershipsForResource() error = %v, want nil", err)
	}

	// Verify all ownerships are gone
	for _, owner := range owners {
		hasOwnership, err := e.HasOwnershipForResource(resourceID, owner)
		if err != nil {
			t.Fatalf("HasOwnershipForResource() failed: %v", err)
		}
		if hasOwnership {
			t.Errorf("Ownership still exists for %s after removal", owner)
		}
	}
}

func TestHasOwnershipForResource(t *testing.T) {
	e := createTestEnforcer(t)

	resourceID := "secret:test"
	ownerEmail := "owner@example.com"

	t.Run("no ownership", func(t *testing.T) {
		hasOwnership, err := e.HasOwnershipForResource(resourceID, ownerEmail)
		if err != nil {
			t.Fatalf("HasOwnershipForResource() error = %v, want nil", err)
		}
		if hasOwnership {
			t.Errorf("HasOwnershipForResource() = true, want false")
		}
	})

	t.Run("with ownership", func(t *testing.T) {
		if err := e.AddOwnershipForResource(resourceID, ownerEmail); err != nil {
			t.Fatalf("AddOwnershipForResource() failed: %v", err)
		}

		hasOwnership, err := e.HasOwnershipForResource(resourceID, ownerEmail)
		if err != nil {
			t.Fatalf("HasOwnershipForResource() error = %v, want nil", err)
		}
		if !hasOwnership {
			t.Errorf("HasOwnershipForResource() = false, want true")
		}
	})
}

func TestLoadRolesForUsers(t *testing.T) {
	tests := []struct {
		name      string
		userRoles map[string]string
		wantError bool
		errorMsg  string
	}{
		{
			name: "load valid roles",
			userRoles: map[string]string{
				"admin@example.com":     "admin",
				"operator@example.com":  "operator",
				"developer@example.com": "developer",
				"viewer@example.com":    "viewer",
			},
			wantError: false,
		},
		{
			name:      "load empty map",
			userRoles: map[string]string{},
			wantError: false,
		},
		{
			name: "load invalid role",
			userRoles: map[string]string{
				"user@example.com": "invalid-role",
			},
			wantError: true,
			errorMsg:  "invalid role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := createTestEnforcer(t)

			err := e.LoadRolesForUsers(tt.userRoles)

			if tt.wantError {
				if err == nil {
					t.Errorf("LoadRolesForUsers() error = nil, want error containing %q", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("LoadRolesForUsers() error = %v, want error containing %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("LoadRolesForUsers() error = %v, want nil", err)
				}

				// Verify all roles were loaded
				for user, roleStr := range tt.userRoles {
					roles, err := e.GetRolesForUser(user)
					if err != nil {
						t.Fatalf("GetRolesForUser(%s) failed: %v", user, err)
					}
					expectedRole := "role:" + roleStr
					if !containsString(roles, expectedRole) {
						t.Errorf("GetRolesForUser(%s) = %v, want to contain %q", user, roles, expectedRole)
					}
				}
			}
		})
	}
}

func TestLoadResourceOwnerships(t *testing.T) {
	e := createTestEnforcer(t)

	ownerships := map[string]string{
		"secret:db-password":    "admin@example.com",
		"execution:exec-123":    "dev@example.com",
		"image:img-456":         "operator@example.com",
		"secret:api-key":        "admin@example.com",
		"execution:long-job-77": "dev@example.com",
	}

	err := e.LoadResourceOwnerships(ownerships)
	if err != nil {
		t.Fatalf("LoadResourceOwnerships() error = %v, want nil", err)
	}

	// Verify all ownerships were loaded
	for resourceID, ownerEmail := range ownerships {
		hasOwnership, err := e.HasOwnershipForResource(resourceID, ownerEmail)
		if err != nil {
			t.Fatalf("HasOwnershipForResource(%s, %s) failed: %v", resourceID, ownerEmail, err)
		}
		if !hasOwnership {
			t.Errorf("HasOwnershipForResource(%s, %s) = false, want true", resourceID, ownerEmail)
		}
	}
}

func TestGetAllNamedGroupingPolicies(t *testing.T) {
	e := createTestEnforcer(t)

	// Add some ownership policies (g2)
	ownerships := map[string]string{
		"secret:test1": "owner1@example.com",
		"secret:test2": "owner2@example.com",
	}
	for resourceID, ownerEmail := range ownerships {
		if err := e.AddOwnershipForResource(resourceID, ownerEmail); err != nil {
			t.Fatalf("AddOwnershipForResource() failed: %v", err)
		}
	}

	policies, err := e.GetAllNamedGroupingPolicies("g2")
	if err != nil {
		t.Fatalf("GetAllNamedGroupingPolicies() error = %v, want nil", err)
	}

	if len(policies) != len(ownerships) {
		t.Errorf("GetAllNamedGroupingPolicies() returned %d policies, want %d", len(policies), len(ownerships))
	}
}

func TestEnforce(t *testing.T) {
	e := createTestEnforcer(t)

	tests := []struct {
		name    string
		setup   func()
		subject string
		object  string
		action  Action
		want    bool
	}{
		{
			name: "admin can do everything",
			setup: func() {
				_ = e.AddRoleForUser("admin@example.com", RoleAdmin)
			},
			subject: "admin@example.com",
			object:  "/api/v1/secrets/db-password",
			action:  ActionRead,
			want:    true,
		},
		{
			name: "operator can read secrets",
			setup: func() {
				_ = e.AddRoleForUser("operator@example.com", RoleOperator)
			},
			subject: "operator@example.com",
			object:  "/api/v1/secrets/api-key",
			action:  ActionRead,
			want:    true,
		},
		{
			name: "developer can create secrets",
			setup: func() {
				_ = e.AddRoleForUser("developer@example.com", RoleDeveloper)
			},
			subject: "developer@example.com",
			object:  "/api/v1/secrets",
			action:  ActionCreate,
			want:    true,
		},
		{
			name: "viewer can read executions",
			setup: func() {
				_ = e.AddRoleForUser("viewer@example.com", RoleViewer)
			},
			subject: "viewer@example.com",
			object:  "/api/v1/executions",
			action:  ActionRead,
			want:    true,
		},
		{
			name: "viewer cannot delete executions",
			setup: func() {
				_ = e.AddRoleForUser("viewer2@example.com", RoleViewer)
			},
			subject: "viewer2@example.com",
			object:  "/api/v1/executions/exec-123",
			action:  ActionDelete,
			want:    false,
		},
		{
			name: "owner can access their secret",
			setup: func() {
				_ = e.AddOwnershipForResource("/api/v1/secrets/my-secret", "owner@example.com")
			},
			subject: "owner@example.com",
			object:  "/api/v1/secrets/my-secret",
			action:  ActionRead,
			want:    true,
		},
		{
			name: "owner can delete their execution",
			setup: func() {
				_ = e.AddOwnershipForResource("/api/v1/executions/my-exec", "exec-owner@example.com")
			},
			subject: "exec-owner@example.com",
			object:  "/api/v1/executions/my-exec",
			action:  ActionDelete,
			want:    true,
		},
		{
			name: "non-owner cannot access resource",
			setup: func() {
				_ = e.AddOwnershipForResource("secret:other-secret", "someone@example.com")
			},
			subject: "notowner@example.com",
			object:  "/api/v1/secrets/other-secret",
			action:  ActionRead,
			want:    false,
		},
		{
			name: "developer denied user management",
			setup: func() {
				_ = e.AddRoleForUser("dev-denied@example.com", RoleDeveloper)
			},
			subject: "dev-denied@example.com",
			object:  "/api/v1/users/some-user",
			action:  ActionRead,
			want:    false,
		},
		{
			name: "viewer denied user management",
			setup: func() {
				_ = e.AddRoleForUser("viewer-denied@example.com", RoleViewer)
			},
			subject: "viewer-denied@example.com",
			object:  "/api/v1/users/some-user",
			action:  ActionUpdate,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			allowed, err := e.Enforce(tt.subject, tt.object, tt.action)
			if err != nil {
				t.Fatalf("Enforce() error = %v, want nil", err)
			}
			if allowed != tt.want {
				t.Errorf("Enforce(%s, %s, %s) = %v, want %v", tt.subject, tt.object, tt.action, allowed, tt.want)
			}
		})
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Benchmarks
func BenchmarkNewEnforcer(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewEnforcer(logger)
	}
}

func BenchmarkEnforce(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	enforcer, _ := NewEnforcer(logger)
	_ = enforcer.AddRoleForUser("user@example.com", RoleAdmin)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enforcer.Enforce("user@example.com", "/api/v1/secrets/test", ActionRead)
	}
}

func BenchmarkAddRoleForUser(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	enforcer, _ := NewEnforcer(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = enforcer.AddRoleForUser("user@example.com", RoleAdmin)
	}
}

func BenchmarkAddOwnershipForResource(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	enforcer, _ := NewEnforcer(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = enforcer.AddOwnershipForResource("secret:test", "owner@example.com")
	}
}
