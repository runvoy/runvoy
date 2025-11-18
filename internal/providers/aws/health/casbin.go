package health

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/health"
)

const (
	minPolicyLength = 2
	resourceIDParts = 2
)

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (m *Manager) reconcileCasbin(
	ctx context.Context,
	reqLogger *slog.Logger,
) (health.AuthorizerHealthStatus, []health.Issue, error) {
	status := health.AuthorizerHealthStatus{
		UsersWithInvalidRoles:      []string{},
		UsersWithMissingRoles:      []string{},
		ResourcesWithMissingOwners: []string{},
		OrphanedOwnerships:         []string{},
		MissingOwnerships:          []string{},
	}
	issues := []health.Issue{}

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
	_ *slog.Logger,
	status *health.AuthorizerHealthStatus,
) ([]health.Issue, error) {
	users, listErr := m.userRepo.ListUsers(ctx)
	if listErr != nil {
		return nil, fmt.Errorf("failed to list users: %w", listErr)
	}

	status.TotalUsersChecked = len(users)

	issues := []health.Issue{}
	for _, user := range users {
		userIssues := m.checkSingleUserRole(user, status)
		issues = append(issues, userIssues...)
	}

	return issues, nil
}

func (m *Manager) checkSingleUserRole(
	user *api.User,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	if user == nil {
		return nil
	}

	email := user.Email
	if email == "" {
		return m.createEmptyEmailIssue()
	}

	role := user.Role
	if role == "" {
		return m.createEmptyRoleIssue(email, status)
	}

	if !authorization.IsValidRole(role) {
		return m.createInvalidRoleIssue(email, role, status)
	}

	return m.checkUserRoleInEnforcer(email, role, status)
}

func (m *Manager) createEmptyEmailIssue() []health.Issue {
	return []health.Issue{{
		ResourceType: "user",
		ResourceID:   "unknown",
		Severity:     "error",
		Message:      "User has empty email field",
		Action:       "reported",
	}}
}

func (m *Manager) createEmptyRoleIssue(
	email string,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	status.UsersWithMissingRoles = append(status.UsersWithMissingRoles, email)
	return []health.Issue{{
		ResourceType: "user",
		ResourceID:   email,
		Severity:     "error",
		Message:      fmt.Sprintf("User %s has empty role field (required for Casbin)", email),
		Action:       "reported",
	}}
}

func (m *Manager) createInvalidRoleIssue(
	email, role string,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	status.UsersWithInvalidRoles = append(status.UsersWithInvalidRoles, email)
	return []health.Issue{{
		ResourceType: "user",
		ResourceID:   email,
		Severity:     "error",
		Message:      fmt.Sprintf("User %s has invalid role %q (required for Casbin)", email, role),
		Action:       "reported",
	}}
}

func (m *Manager) checkUserRoleInEnforcer(
	email, role string,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	roles, rolesErr := m.enforcer.GetRolesForUser(email)
	if rolesErr != nil {
		return []health.Issue{{
			ResourceType: "user",
			ResourceID:   email,
			Severity:     "error",
			Message: fmt.Sprintf(
				"Failed to check Casbin roles for user %s: %v",
				email, rolesErr,
			),
			Action: "reported",
		}}
	}

	expectedRole := authorization.FormatRole(authorization.Role(role))
	hasRole := slices.Contains(roles, expectedRole)

	if !hasRole {
		status.UsersWithMissingRoles = append(status.UsersWithMissingRoles, email)
		return []health.Issue{{
			ResourceType: "user",
			ResourceID:   email,
			Severity:     "error",
			Message: fmt.Sprintf(
				"User %s has role %q in database but not in Casbin enforcer",
				email, role,
			),
			Action: "reported",
		}}
	}

	return nil
}

func (m *Manager) checkResourceOwnership(
	ctx context.Context,
	_ *slog.Logger,
	status *health.AuthorizerHealthStatus,
) ([]health.Issue, error) {
	issues := []health.Issue{}
	resourceCount := 0

	if m.secretsRepo != nil {
		secrets, secretsErr := m.secretsRepo.ListSecrets(ctx, false)
		if secretsErr != nil {
			return issues, fmt.Errorf("failed to list secrets: %w", secretsErr)
		}

		for _, secret := range secrets {
			resourceCount++
			secretIssues := m.checkSecretOwnership(secret, status)
			issues = append(issues, secretIssues...)
		}
	}

	if m.executionRepo != nil {
		executions, execErr := m.executionRepo.ListExecutions(ctx, 0, nil)
		if execErr != nil {
			return issues, fmt.Errorf("failed to list executions: %w", execErr)
		}

		for _, execution := range executions {
			resourceCount++
			execIssues := m.checkExecutionOwnership(execution, status)
			issues = append(issues, execIssues...)
		}
	}

	if m.imageRepo != nil {
		images, imagesErr := m.imageRepo.ListImages(ctx)
		if imagesErr != nil {
			return issues, fmt.Errorf("failed to list images: %w", imagesErr)
		}

		for i := range images {
			resourceCount++
			imageIssues := m.checkImageOwnership(&images[i], status)
			issues = append(issues, imageIssues...)
		}
	}

	status.TotalResourcesChecked = resourceCount
	return issues, nil
}

func (m *Manager) checkSecretOwnership(
	secret *api.Secret,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	if secret == nil || secret.Name == "" {
		return nil
	}

	return m.checkResourceOwnershipGeneric(
		"secret",
		secret.Name,
		secret.CreatedBy,
		status,
	)
}

func (m *Manager) checkExecutionOwnership(
	execution *api.Execution,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	if execution == nil || execution.ExecutionID == "" {
		return nil
	}

	return m.checkResourceOwnershipGeneric(
		"execution",
		execution.ExecutionID,
		execution.UserEmail,
		status,
	)
}

func (m *Manager) checkResourceOwnershipGeneric(
	resourceType, resourceID, ownerEmail string,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	if ownerEmail == "" {
		formattedID := authorization.FormatResourceID(resourceType, resourceID)
		status.ResourcesWithMissingOwners = append(
			status.ResourcesWithMissingOwners, formattedID,
		)
		return []health.Issue{{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Severity:     "error",
			Message: fmt.Sprintf(
				"%s %s has empty owner field (required for Casbin ownership)",
				capitalizeFirst(resourceType), resourceID,
			),
			Action: "reported",
		}}
	}

	formattedID := authorization.FormatResourceID(resourceType, resourceID)
	hasOwnership, checkErr := m.enforcer.HasOwnershipForResource(formattedID, ownerEmail)
	if checkErr != nil {
		return []health.Issue{{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Severity:     "error",
			Message: fmt.Sprintf(
				"Failed to check Casbin ownership for %s %s: %v",
				resourceType, resourceID, checkErr,
			),
			Action: "reported",
		}}
	}

	if !hasOwnership {
		status.MissingOwnerships = append(status.MissingOwnerships, formattedID)
		return []health.Issue{{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Severity:     "error",
			Message: fmt.Sprintf(
				"%s %s has owner %s in database but not in Casbin enforcer",
				capitalizeFirst(resourceType), resourceID, ownerEmail,
			),
			Action: "reported",
		}}
	}

	return nil
}

func (m *Manager) checkImageOwnership(
	image *api.ImageInfo,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	if image.ImageID == "" {
		return nil
	}

	if image.RegisteredBy == "" {
		resourceID := authorization.FormatResourceID("image", image.ImageID)
		status.ResourcesWithMissingOwners = append(
			status.ResourcesWithMissingOwners, resourceID,
		)
		return []health.Issue{{
			ResourceType: "image",
			ResourceID:   image.ImageID,
			Severity:     "warning",
			Message: fmt.Sprintf(
				"Image %s has empty RegisteredBy field (may affect Casbin ownership)",
				image.ImageID,
			),
			Action: "reported",
		}}
	}

	return nil
}

func (m *Manager) checkOrphanedOwnerships(
	ctx context.Context,
	_ *slog.Logger,
	status *health.AuthorizerHealthStatus,
) ([]health.Issue, error) {
	if m.enforcer == nil || m.userRepo == nil {
		return nil, nil
	}

	resourceMaps, mapsErr := m.buildResourceMaps(ctx)
	if mapsErr != nil {
		return nil, mapsErr
	}

	policies, policiesErr := m.enforcer.GetAllNamedGroupingPolicies("g2")
	if policiesErr != nil {
		return nil, fmt.Errorf("failed to get Casbin g2 policies: %w", policiesErr)
	}

	issues := []health.Issue{}
	for _, policy := range policies {
		policyIssues := m.checkPolicyOrphaned(policy, resourceMaps, status)
		issues = append(issues, policyIssues...)
	}

	return issues, nil
}

type resourceMaps struct {
	userMap      map[string]bool
	secretMap    map[string]bool
	executionMap map[string]bool
}

func (m *Manager) buildResourceMaps(ctx context.Context) (*resourceMaps, error) {
	users, usersErr := m.userRepo.ListUsers(ctx)
	if usersErr != nil {
		return nil, fmt.Errorf("failed to list users for orphaned ownership check: %w", usersErr)
	}

	userMap := make(map[string]bool)
	for _, user := range users {
		if user != nil && user.Email != "" {
			userMap[user.Email] = true
		}
	}

	secrets, secretsErr := m.secretsRepo.ListSecrets(ctx, false)
	if secretsErr != nil {
		return nil, fmt.Errorf("failed to list secrets for orphaned ownership check: %w", secretsErr)
	}

	secretMap := make(map[string]bool)
	for _, secret := range secrets {
		if secret != nil && secret.Name != "" {
			secretMap[secret.Name] = true
		}
	}

	executions, execErr := m.executionRepo.ListExecutions(ctx, 0, nil)
	if execErr != nil {
		return nil, fmt.Errorf("failed to list executions for orphaned ownership check: %w", execErr)
	}

	executionMap := make(map[string]bool)
	for _, execution := range executions {
		if execution != nil && execution.ExecutionID != "" {
			executionMap[execution.ExecutionID] = true
		}
	}

	return &resourceMaps{
		userMap:      userMap,
		secretMap:    secretMap,
		executionMap: executionMap,
	}, nil
}

func (m *Manager) checkPolicyOrphaned(
	policy []string,
	maps *resourceMaps,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	if len(policy) < minPolicyLength {
		return nil
	}

	resourceID := policy[0]
	ownerEmail := policy[1]

	if !maps.userMap[ownerEmail] {
		status.OrphanedOwnerships = append(
			status.OrphanedOwnerships,
			fmt.Sprintf("%s -> %s", resourceID, ownerEmail),
		)
		return []health.Issue{{
			ResourceType: "casbin_ownership",
			ResourceID:   resourceID,
			Severity:     "error",
			Message: fmt.Sprintf(
				"Casbin ownership %s -> %s is orphaned: owner user does not exist",
				resourceID, ownerEmail,
			),
			Action: "reported",
		}}
	}

	parts := strings.Split(resourceID, ":")
	if len(parts) != resourceIDParts {
		return nil
	}

	resourceType := parts[0]
	resourceName := parts[1]

	switch resourceType {
	case "secret":
		return m.checkSecretOrphaned(resourceID, resourceName, ownerEmail, maps, status)
	case "execution":
		return m.checkExecutionOrphaned(resourceID, resourceName, ownerEmail, maps, status)
	default:
		return nil
	}
}

func (m *Manager) checkSecretOrphaned(
	resourceID, resourceName, ownerEmail string,
	maps *resourceMaps,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	if maps.secretMap[resourceName] {
		return nil
	}

	status.OrphanedOwnerships = append(
		status.OrphanedOwnerships,
		fmt.Sprintf("%s -> %s", resourceID, ownerEmail),
	)
	return []health.Issue{{
		ResourceType: "casbin_ownership",
		ResourceID:   resourceID,
		Severity:     "error",
		Message: fmt.Sprintf(
			"Casbin ownership %s -> %s is orphaned: secret resource does not exist",
			resourceID, ownerEmail,
		),
		Action: "reported",
	}}
}

func (m *Manager) checkExecutionOrphaned(
	resourceID, resourceName, ownerEmail string,
	maps *resourceMaps,
	status *health.AuthorizerHealthStatus,
) []health.Issue {
	if maps.executionMap[resourceName] {
		return nil
	}

	status.OrphanedOwnerships = append(
		status.OrphanedOwnerships,
		fmt.Sprintf("%s -> %s", resourceID, ownerEmail),
	)
	return []health.Issue{{
		ResourceType: "casbin_ownership",
		ResourceID:   resourceID,
		Severity:     "error",
		Message: fmt.Sprintf(
			"Casbin ownership %s -> %s is orphaned: execution resource does not exist",
			resourceID, ownerEmail,
		),
		Action: "reported",
	}}
}
