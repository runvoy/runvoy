package health

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/backend/contract"
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
) (contract.AuthorizerHealthStatus, []contract.HealthIssue, error) {
	status := contract.AuthorizerHealthStatus{
		UsersWithInvalidRoles:      []string{},
		UsersWithMissingRoles:      []string{},
		ResourcesWithMissingOwners: []string{},
		OrphanedOwnerships:         []string{},
		MissingOwnerships:          []string{},
	}
	issues := []contract.HealthIssue{}

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
	status *contract.AuthorizerHealthStatus,
) ([]contract.HealthIssue, error) {
	users, listErr := m.userRepo.ListUsers(ctx)
	if listErr != nil {
		return nil, fmt.Errorf("failed to list users: %w", listErr)
	}

	status.TotalUsersChecked = len(users)

	issues := []contract.HealthIssue{}
	for _, user := range users {
		userIssues := m.checkSingleUserRole(user, status)
		issues = append(issues, userIssues...)
	}

	return issues, nil
}

func (m *Manager) checkSingleUserRole(
	user *api.User,
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
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

func (m *Manager) createEmptyEmailIssue() []contract.HealthIssue {
	return []contract.HealthIssue{{
		ResourceType: "user",
		ResourceID:   "unknown",
		Severity:     "error",
		Message:      "User has empty email field",
		Action:       "reported",
	}}
}

func (m *Manager) createEmptyRoleIssue(
	email string,
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
	status.UsersWithMissingRoles = append(status.UsersWithMissingRoles, email)
	return []contract.HealthIssue{{
		ResourceType: "user",
		ResourceID:   email,
		Severity:     "error",
		Message:      fmt.Sprintf("User %s has empty role field (required for Casbin)", email),
		Action:       "reported",
	}}
}

func (m *Manager) createInvalidRoleIssue(
	email, role string,
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
	status.UsersWithInvalidRoles = append(status.UsersWithInvalidRoles, email)
	return []contract.HealthIssue{{
		ResourceType: "user",
		ResourceID:   email,
		Severity:     "error",
		Message:      fmt.Sprintf("User %s has invalid role %q (required for Casbin)", email, role),
		Action:       "reported",
	}}
}

func (m *Manager) checkUserRoleInEnforcer(
	email, role string,
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
	roles, rolesErr := m.enforcer.GetRolesForUser(email)
	if rolesErr != nil {
		return []contract.HealthIssue{{
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
		return []contract.HealthIssue{{
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
	status *contract.AuthorizerHealthStatus,
) ([]contract.HealthIssue, error) {
	issues := []contract.HealthIssue{}
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
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
	if secret == nil || secret.Name == "" {
		return nil
	}

	return m.checkResourceOwnershipGeneric(
		"secret",
		secret.Name,
		secret.CreatedBy,
		secret.OwnedBy,
		status,
	)
}

func (m *Manager) checkExecutionOwnership(
	execution *api.Execution,
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
	if execution == nil || execution.ExecutionID == "" {
		return nil
	}

	return m.checkResourceOwnershipGeneric(
		"execution",
		execution.ExecutionID,
		execution.CreatedBy,
		execution.OwnedBy,
		status,
	)
}

//nolint:funlen // Validates multiple owners, requires iteration logic
func (m *Manager) checkResourceOwnershipGeneric(
	resourceType, resourceID, createdBy string,
	ownedBy []string,
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
	if createdBy == "" {
		formattedID := authorization.FormatResourceID(resourceType, resourceID)
		status.ResourcesWithMissingOwners = append(
			status.ResourcesWithMissingOwners, formattedID,
		)
		return []contract.HealthIssue{{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Severity:     "error",
			Message: fmt.Sprintf(
				"%s %s has empty created_by field (required for Casbin ownership)",
				capitalizeFirst(resourceType), resourceID,
			),
			Action: "reported",
		}}
	}

	if len(ownedBy) == 0 {
		formattedID := authorization.FormatResourceID(resourceType, resourceID)
		status.ResourcesWithMissingOwners = append(
			status.ResourcesWithMissingOwners, formattedID,
		)
		return []contract.HealthIssue{{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Severity:     "error",
			Message: fmt.Sprintf(
				"%s %s has empty owned_by list (required for Casbin ownership)",
				capitalizeFirst(resourceType), resourceID,
			),
			Action: "reported",
		}}
	}

	formattedID := authorization.FormatResourceID(resourceType, resourceID)
	issues := []contract.HealthIssue{}
	for _, owner := range ownedBy {
		hasOwnership, checkErr := m.enforcer.HasOwnershipForResource(formattedID, owner)
		if checkErr != nil {
			issues = append(issues, contract.HealthIssue{
				ResourceType: resourceType,
				ResourceID:   resourceID,
				Severity:     "error",
				Message: fmt.Sprintf(
					"Failed to check Casbin ownership for %s %s (owner %s): %v",
					resourceType, resourceID, owner, checkErr,
				),
				Action: "reported",
			})
			continue
		}

		if !hasOwnership {
			status.MissingOwnerships = append(status.MissingOwnerships, formattedID)
			issues = append(issues, contract.HealthIssue{
				ResourceType: resourceType,
				ResourceID:   resourceID,
				Severity:     "error",
				Message: fmt.Sprintf(
					"%s %s has owner %s in database but not in Casbin enforcer",
					capitalizeFirst(resourceType), resourceID, owner,
				),
				Action: "reported",
			})
		}
	}

	return issues
}

func (m *Manager) checkImageOwnership(
	image *api.ImageInfo,
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
	if image.ImageID == "" {
		return nil
	}

	return m.checkResourceOwnershipGeneric(
		"image",
		image.ImageID,
		image.CreatedBy,
		image.OwnedBy,
		status,
	)
}

func (m *Manager) checkOrphanedOwnerships(
	ctx context.Context,
	_ *slog.Logger,
	status *contract.AuthorizerHealthStatus,
) ([]contract.HealthIssue, error) {
	resourceMaps, mapsErr := m.buildResourceMaps(ctx)
	if mapsErr != nil {
		return nil, mapsErr
	}

	policies, policiesErr := m.enforcer.GetAllNamedGroupingPolicies("g2")
	if policiesErr != nil {
		return nil, fmt.Errorf("failed to get Casbin g2 policies: %w", policiesErr)
	}

	issues := []contract.HealthIssue{}
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
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
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
		return []contract.HealthIssue{{
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
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
	if maps.secretMap[resourceName] {
		return nil
	}

	status.OrphanedOwnerships = append(
		status.OrphanedOwnerships,
		fmt.Sprintf("%s -> %s", resourceID, ownerEmail),
	)
	return []contract.HealthIssue{{
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
	status *contract.AuthorizerHealthStatus,
) []contract.HealthIssue {
	if maps.executionMap[resourceName] {
		return nil
	}

	status.OrphanedOwnerships = append(
		status.OrphanedOwnerships,
		fmt.Sprintf("%s -> %s", resourceID, ownerEmail),
	)
	return []contract.HealthIssue{{
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
