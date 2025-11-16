package health

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"runvoy/internal/backend/health"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

func (m *Manager) reconcileIAMRoles(
	ctx context.Context,
	_ *slog.Logger,
) (health.IdentityHealthStatus, []health.Issue, error) {
	status := health.IdentityHealthStatus{
		MissingRoles: []string{},
	}
	issues := []health.Issue{}

	defaultIssues := m.verifyDefaultRoles(ctx, &status)
	issues = append(issues, defaultIssues...)

	customIssues, err := m.verifyCustomRoles(ctx, &status)
	if err != nil {
		return status, issues, fmt.Errorf("failed to verify custom roles: %w", err)
	}
	issues = append(issues, customIssues...)

	return status, issues, nil
}

func (m *Manager) verifyDefaultRoles(ctx context.Context, status *health.IdentityHealthStatus) []health.Issue {
	issues := []health.Issue{}

	if m.cfg.DefaultTaskRoleARN != "" {
		issues = append(issues, m.verifyRole(ctx, m.cfg.DefaultTaskRoleARN, "Default task role", status)...)
	}

	if m.cfg.DefaultTaskExecRoleARN != "" {
		issues = append(issues, m.verifyRole(ctx, m.cfg.DefaultTaskExecRoleARN, "Default task execution role", status)...)
	}

	if len(status.MissingRoles) == 0 {
		status.DefaultRolesVerified = true
	}

	return issues
}

func (m *Manager) verifyRole(
	ctx context.Context,
	roleARN string,
	roleDescription string,
	status *health.IdentityHealthStatus,
) []health.Issue {
	roleName := extractRoleNameFromARN(roleARN)
	_, err := m.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: awsStd.String(roleName),
	})
	if err == nil {
		return []health.Issue{}
	}

	if strings.Contains(err.Error(), "NoSuchEntity") {
		status.MissingRoles = append(status.MissingRoles, roleARN)
		return []health.Issue{
			{
				ResourceType: "iam_role",
				ResourceID:   roleARN,
				Severity:     "error",
				Message:      fmt.Sprintf("%s missing (managed by CloudFormation)", roleDescription),
				Action:       "requires_manual_intervention",
			},
		}
	}

	return []health.Issue{
		{
			ResourceType: "iam_role",
			ResourceID:   roleARN,
			Severity:     "error",
			Message:      fmt.Sprintf("Failed to verify %s: %v", roleDescription, err),
			Action:       "reported",
		},
	}
}

func (m *Manager) verifyCustomRoles(
	ctx context.Context,
	status *health.IdentityHealthStatus,
) ([]health.Issue, error) {
	images, err := m.imageRepo.ListImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	customRoles := make(map[string]bool)
	for i := range images {
		img := &images[i]
		if img.TaskRoleName != nil && *img.TaskRoleName != "" {
			customRoles[*img.TaskRoleName] = true
		}
		if img.TaskExecutionRoleName != nil && *img.TaskExecutionRoleName != "" {
			customRoles[*img.TaskExecutionRoleName] = true
		}
	}

	status.CustomRolesTotal = len(customRoles)
	issues := []health.Issue{}

	for roleName := range customRoles {
		roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", m.cfg.AccountID, roleName)
		_, getRoleErr := m.iamClient.GetRole(ctx, &iam.GetRoleInput{
			RoleName: awsStd.String(roleName),
		})
		if getRoleErr != nil {
			if strings.Contains(getRoleErr.Error(), "NoSuchEntity") {
				status.MissingRoles = append(status.MissingRoles, roleARN)
				issues = append(issues, health.Issue{
					ResourceType: "iam_role",
					ResourceID:   roleARN,
					Severity:     "error",
					Message:      "Custom IAM role missing (cannot recreate without policies)",
					Action:       "requires_manual_intervention",
				})
			} else {
				issues = append(issues, health.Issue{
					ResourceType: "iam_role",
					ResourceID:   roleARN,
					Severity:     "error",
					Message:      fmt.Sprintf("Failed to verify custom role: %v", getRoleErr),
					Action:       "reported",
				})
			}
		} else {
			status.CustomRolesVerified++
		}
	}

	return issues, nil
}
