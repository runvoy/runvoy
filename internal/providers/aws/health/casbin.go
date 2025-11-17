package health

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/health"
)

func (m *Manager) reconcileCasbin(
	ctx context.Context,
	reqLogger *slog.Logger,
) (health.CasbinHealthStatus, []health.Issue, error) {
	status := health.CasbinHealthStatus{
		UsersWithInvalidRoles:      []string{},
		UsersWithMissingRoles:      []string{},
		ResourcesWithMissingOwners: []string{},
		OrphanedOwnerships:         []string{},
		MissingOwnerships:          []string{},
	}
	issues := []health.Issue{}

	if m.userRepo == nil || m.enforcer == nil {
		reqLogger.Debug("skipping Casbin reconciliation: userRepo or enforcer not available")
		return status, issues, nil
	}

	userIssues, err := m.checkUserRoles(ctx, reqLogger, &status)
	if err != nil {
		return status, issues, fmt.Errorf("failed to check user roles: %w", err)
	}
	issues = append(issues, userIssues...)

	resourceIssues, err := m.checkResourceOwnership(ctx, reqLogger, &status)
	if err != nil {
		return status, issues, fmt.Errorf("failed to check resource ownership: %w", err)
	}
	issues = append(issues, resourceIssues...)

	orphanedIssues, err := m.checkOrphanedOwnerships(ctx, reqLogger, &status)
	if err != nil {
		return status, issues, fmt.Errorf("failed to check orphaned ownerships: %w", err)
	}
	issues = append(issues, orphanedIssues...)

	return status, issues, nil
}

func (m *Manager) checkUserRoles(
	ctx context.Context,
	reqLogger *slog.Logger,
	status *health.CasbinHealthStatus,
) ([]health.Issue, error) {
	issues := []health.Issue{}

	users, err := m.userRepo.ListUsers(ctx)
	if err != nil {
		return issues, fmt.Errorf("failed to list users: %w", err)
	}

	status.TotalUsersChecked = len(users)

	for _, user := range users {
		if user == nil {
			continue
		}

		if user.Email == "" {
			issues = append(issues, health.Issue{
				ResourceType: "user",
				ResourceID:   "unknown",
				Severity:     "error",
				Message:      "User has empty email field",
				Action:       "reported",
			})
			continue
		}

		if user.Role == "" {
			status.UsersWithMissingRoles = append(status.UsersWithMissingRoles, user.Email)
			issues = append(issues, health.Issue{
				ResourceType: "user",
				ResourceID:   user.Email,
				Severity:     "error",
				Message:      fmt.Sprintf("User %s has empty role field (required for Casbin)", user.Email),
				Action:       "reported",
			})
			continue
		}

		if !authorization.IsValidRole(user.Role) {
			status.UsersWithInvalidRoles = append(status.UsersWithInvalidRoles, user.Email)
			issues = append(issues, health.Issue{
				ResourceType: "user",
				ResourceID:   user.Email,
				Severity:     "error",
				Message:      fmt.Sprintf("User %s has invalid role %q (required for Casbin)", user.Email, user.Role),
				Action:       "reported",
			})
			continue
		}

		roles, err := m.enforcer.GetRolesForUser(user.Email)
		if err != nil {
			issues = append(issues, health.Issue{
				ResourceType: "user",
				ResourceID:   user.Email,
				Severity:     "error",
				Message:      fmt.Sprintf("Failed to check Casbin roles for user %s: %v", user.Email, err),
				Action:       "reported",
			})
			continue
		}

		expectedRole := authorization.FormatRole(authorization.Role(user.Role))
		hasRole := false
		for _, role := range roles {
			if role == expectedRole {
				hasRole = true
				break
			}
		}

		if !hasRole {
			status.UsersWithMissingRoles = append(status.UsersWithMissingRoles, user.Email)
			issues = append(issues, health.Issue{
				ResourceType: "user",
				ResourceID:   user.Email,
				Severity:     "error",
				Message:      fmt.Sprintf("User %s has role %q in database but not in Casbin enforcer", user.Email, user.Role),
				Action:       "reported",
			})
		}
	}

	return issues, nil
}

func (m *Manager) checkResourceOwnership(
	ctx context.Context,
	reqLogger *slog.Logger,
	status *health.CasbinHealthStatus,
) ([]health.Issue, error) {
	issues := []health.Issue{}
	resourceCount := 0

	if m.secretsRepo != nil {
		secrets, err := m.secretsRepo.ListSecrets(ctx, false)
		if err != nil {
			return issues, fmt.Errorf("failed to list secrets: %w", err)
		}

		for _, secret := range secrets {
			resourceCount++
			if secret == nil || secret.Name == "" {
				continue
			}

			if secret.CreatedBy == "" {
				resourceID := authorization.FormatResourceID("secret", secret.Name)
				status.ResourcesWithMissingOwners = append(status.ResourcesWithMissingOwners, resourceID)
				issues = append(issues, health.Issue{
					ResourceType: "secret",
					ResourceID:   secret.Name,
					Severity:     "error",
					Message:      fmt.Sprintf("Secret %s has empty CreatedBy field (required for Casbin ownership)", secret.Name),
					Action:       "reported",
				})
				continue
			}

			resourceID := authorization.FormatResourceID("secret", secret.Name)
			hasOwnership, err := m.enforcer.HasOwnershipForResource(resourceID, secret.CreatedBy)
			if err != nil {
				issues = append(issues, health.Issue{
					ResourceType: "secret",
					ResourceID:   secret.Name,
					Severity:     "error",
					Message:      fmt.Sprintf("Failed to check Casbin ownership for secret %s: %v", secret.Name, err),
					Action:       "reported",
				})
				continue
			}

			if !hasOwnership {
				status.MissingOwnerships = append(status.MissingOwnerships, resourceID)
				issues = append(issues, health.Issue{
					ResourceType: "secret",
					ResourceID:   secret.Name,
					Severity:     "error",
					Message:      fmt.Sprintf("Secret %s has owner %s in database but not in Casbin enforcer", secret.Name, secret.CreatedBy),
					Action:       "reported",
				})
			}
		}
	}

	if m.executionRepo != nil {
		executions, err := m.executionRepo.ListExecutions(ctx, 0, nil)
		if err != nil {
			return issues, fmt.Errorf("failed to list executions: %w", err)
		}

		for _, execution := range executions {
			resourceCount++
			if execution == nil || execution.ExecutionID == "" {
				continue
			}

			if execution.UserEmail == "" {
				resourceID := authorization.FormatResourceID("execution", execution.ExecutionID)
				status.ResourcesWithMissingOwners = append(status.ResourcesWithMissingOwners, resourceID)
				issues = append(issues, health.Issue{
					ResourceType: "execution",
					ResourceID:   execution.ExecutionID,
					Severity:     "error",
					Message:      fmt.Sprintf("Execution %s has empty UserEmail field (required for Casbin ownership)", execution.ExecutionID),
					Action:       "reported",
				})
				continue
			}

			resourceID := authorization.FormatResourceID("execution", execution.ExecutionID)
			hasOwnership, err := m.enforcer.HasOwnershipForResource(resourceID, execution.UserEmail)
			if err != nil {
				issues = append(issues, health.Issue{
					ResourceType: "execution",
					ResourceID:   execution.ExecutionID,
					Severity:     "error",
					Message:      fmt.Sprintf("Failed to check Casbin ownership for execution %s: %v", execution.ExecutionID, err),
					Action:       "reported",
				})
				continue
			}

			if !hasOwnership {
				status.MissingOwnerships = append(status.MissingOwnerships, resourceID)
				issues = append(issues, health.Issue{
					ResourceType: "execution",
					ResourceID:   execution.ExecutionID,
					Severity:     "error",
					Message:      fmt.Sprintf("Execution %s has owner %s in database but not in Casbin enforcer", execution.ExecutionID, execution.UserEmail),
					Action:       "reported",
				})
			}
		}
	}

	if m.imageRepo != nil {
		images, err := m.imageRepo.ListImages(ctx)
		if err != nil {
			return issues, fmt.Errorf("failed to list images: %w", err)
		}

		for _, image := range images {
			resourceCount++
			if image.ImageID == "" {
				continue
			}

			if image.RegisteredBy == "" {
				resourceID := authorization.FormatResourceID("image", image.ImageID)
				status.ResourcesWithMissingOwners = append(status.ResourcesWithMissingOwners, resourceID)
				issues = append(issues, health.Issue{
					ResourceType: "image",
					ResourceID:   image.ImageID,
					Severity:     "warning",
					Message:      fmt.Sprintf("Image %s has empty RegisteredBy field (may affect Casbin ownership)", image.ImageID),
					Action:       "reported",
				})
			}
		}
	}

	status.TotalResourcesChecked = resourceCount
	return issues, nil
}

func (m *Manager) checkOrphanedOwnerships(
	ctx context.Context,
	reqLogger *slog.Logger,
	status *health.CasbinHealthStatus,
) ([]health.Issue, error) {
	issues := []health.Issue{}

	if m.enforcer == nil || m.userRepo == nil {
		return issues, nil
	}

	users, err := m.userRepo.ListUsers(ctx)
	if err != nil {
		return issues, fmt.Errorf("failed to list users for orphaned ownership check: %w", err)
	}

	userMap := make(map[string]bool)
	for _, user := range users {
		if user != nil && user.Email != "" {
			userMap[user.Email] = true
		}
	}

	secrets, err := m.secretsRepo.ListSecrets(ctx, false)
	if err != nil {
		return issues, fmt.Errorf("failed to list secrets for orphaned ownership check: %w", err)
	}

	secretMap := make(map[string]bool)
	for _, secret := range secrets {
		if secret != nil && secret.Name != "" {
			secretMap[secret.Name] = true
		}
	}

	executions, err := m.executionRepo.ListExecutions(ctx, 0, nil)
	if err != nil {
		return issues, fmt.Errorf("failed to list executions for orphaned ownership check: %w", err)
	}

	executionMap := make(map[string]bool)
	for _, execution := range executions {
		if execution != nil && execution.ExecutionID != "" {
			executionMap[execution.ExecutionID] = true
		}
	}

	if m.enforcer == nil {
		return issues, nil
	}

	allGroupingPolicies, err := m.enforcer.GetAllNamedGroupingPolicies("g2")
	if err != nil {
		return issues, fmt.Errorf("failed to get Casbin g2 policies: %w", err)
	}

	for _, policy := range allGroupingPolicies {
		if len(policy) < 2 {
			continue
		}

		resourceID := policy[0]
		ownerEmail := policy[1]

		if !userMap[ownerEmail] {
			status.OrphanedOwnerships = append(status.OrphanedOwnerships, fmt.Sprintf("%s -> %s", resourceID, ownerEmail))
			issues = append(issues, health.Issue{
				ResourceType: "casbin_ownership",
				ResourceID:   resourceID,
				Severity:     "error",
				Message:      fmt.Sprintf("Casbin ownership %s -> %s is orphaned: owner user does not exist", resourceID, ownerEmail),
				Action:       "reported",
			})
			continue
		}

		parts := strings.Split(resourceID, ":")
		if len(parts) != 2 {
			continue
		}

		resourceType := parts[0]
		resourceName := parts[1]

		switch resourceType {
		case "secret":
			if !secretMap[resourceName] {
				status.OrphanedOwnerships = append(status.OrphanedOwnerships, fmt.Sprintf("%s -> %s", resourceID, ownerEmail))
				issues = append(issues, health.Issue{
					ResourceType: "casbin_ownership",
					ResourceID:   resourceID,
					Severity:     "error",
					Message:      fmt.Sprintf("Casbin ownership %s -> %s is orphaned: secret resource does not exist", resourceID, ownerEmail),
					Action:       "reported",
				})
			}
		case "execution":
			if !executionMap[resourceName] {
				status.OrphanedOwnerships = append(status.OrphanedOwnerships, fmt.Sprintf("%s -> %s", resourceID, ownerEmail))
				issues = append(issues, health.Issue{
					ResourceType: "casbin_ownership",
					ResourceID:   resourceID,
					Severity:     "error",
					Message:      fmt.Sprintf("Casbin ownership %s -> %s is orphaned: execution resource does not exist", resourceID, ownerEmail),
					Action:       "reported",
				})
			}
		}
	}

	return issues, nil
}
