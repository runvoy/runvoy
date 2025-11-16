package health

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"runvoy/internal/backend/health"
	awsConstants "runvoy/internal/providers/aws/constants"
	"runvoy/internal/providers/aws/secrets"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func (m *Manager) reconcileSecrets(
	ctx context.Context,
	reqLogger *slog.Logger,
) (health.SecretsHealthStatus, []health.Issue, error) {
	status := health.SecretsHealthStatus{
		OrphanedParameters: []string{},
	}
	issues := []health.Issue{}

	secretsList, err := m.secretsRepo.ListSecrets(ctx, false)
	if err != nil {
		return status, issues, fmt.Errorf("failed to list secrets: %w", err)
	}
	status.TotalSecrets = len(secretsList)

	seenParameters := make(map[string]bool)

	for _, secret := range secretsList {
		parameterName := m.getParameterName(secret.Name)
		seenParameters[parameterName] = true

		secretIssues := m.checkSecretParameter(ctx, parameterName, secret.Name, reqLogger, &status)
		issues = append(issues, secretIssues...)
	}

	orphanedParams, orphanErr := m.findOrphanedParameters(ctx, seenParameters, reqLogger)
	if orphanErr != nil {
		reqLogger.Warn("failed to find orphaned parameters", "error", orphanErr)
	} else {
		status.OrphanedCount = len(orphanedParams)
		status.OrphanedParameters = orphanedParams
		for _, param := range orphanedParams {
			issues = append(issues, health.Issue{
				ResourceType: "ssm_parameter",
				ResourceID:   param,
				Severity:     "warning",
				Message:      "Parameter exists in SSM but not in DynamoDB (orphaned)",
				Action:       "reported",
			})
		}
	}

	return status, issues, nil
}

func (m *Manager) checkSecretParameter(
	ctx context.Context,
	parameterName string,
	secretName string,
	reqLogger *slog.Logger,
	status *health.SecretsHealthStatus,
) []health.Issue {
	issues := []health.Issue{}

	_, getParamErr := m.ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           awsStd.String(parameterName),
		WithDecryption: awsStd.Bool(false),
	})
	if getParamErr != nil {
		if isParameterNotFound(getParamErr) {
			status.MissingCount++
			issues = append(issues, health.Issue{
				ResourceType: "ssm_parameter",
				ResourceID:   secretName,
				Severity:     "error",
				Message:      "Secret parameter missing in SSM Parameter Store (cannot recreate without value)",
				Action:       "requires_manual_intervention",
			})
			reqLogger.Warn("secret parameter missing", "name", secretName, "parameter", parameterName)
		} else {
			issues = append(issues, health.Issue{
				ResourceType: "ssm_parameter",
				ResourceID:   secretName,
				Severity:     "error",
				Message:      fmt.Sprintf("Failed to check parameter: %v", getParamErr),
				Action:       "reported",
			})
		}
		return issues
	}

	tagUpdated, tagErr := m.verifyAndUpdateSecretTags(ctx, parameterName, secretName, reqLogger)
	if tagErr != nil {
		issues = append(issues, health.Issue{
			ResourceType: "ssm_parameter",
			ResourceID:   secretName,
			Severity:     "warning",
			Message:      fmt.Sprintf("Failed to verify/update tags: %v", tagErr),
			Action:       "reported",
		})
	} else if tagUpdated {
		status.TagUpdatedCount++
		issues = append(issues, health.Issue{
			ResourceType: "ssm_parameter",
			ResourceID:   secretName,
			Severity:     "warning",
			Message:      "Secret parameter tags were updated to match DynamoDB state",
			Action:       "tag_updated",
		})
	} else {
		status.VerifiedCount++
	}

	return issues
}

func (m *Manager) getParameterName(secretName string) string {
	return fmt.Sprintf("%s/%s", m.secretsPrefix, secretName)
}

func (m *Manager) verifyAndUpdateSecretTags(
	ctx context.Context,
	parameterName string,
	secretName string,
	reqLogger *slog.Logger,
) (bool, error) {
	tagsOutput, err := m.ssmClient.ListTagsForResource(ctx, &ssm.ListTagsForResourceInput{
		ResourceType: ssmTypes.ResourceTypeForTaggingParameter,
		ResourceId:   awsStd.String(parameterName),
	})
	if err != nil {
		return false, fmt.Errorf("failed to list tags: %w", err)
	}

	expectedTags := secrets.GetStandardTags()
	expectedTagMap := make(map[string]string)
	for _, tag := range expectedTags {
		expectedTagMap[tag.Key] = tag.Value
	}

	currentTagMap := make(map[string]string)
	for _, tag := range tagsOutput.TagList {
		if tag.Key != nil && tag.Value != nil {
			currentTagMap[*tag.Key] = *tag.Value
		}
	}

	tagsMatch := true
	for key, expectedValue := range expectedTagMap {
		currentValue, exists := currentTagMap[key]
		if !exists || currentValue != expectedValue {
			tagsMatch = false
			break
		}
	}

	if tagsMatch {
		return false, nil
	}

	ssmTags := []ssmTypes.Tag{}
	for _, tag := range expectedTags {
		ssmTags = append(ssmTags, ssmTypes.Tag{
			Key:   awsStd.String(tag.Key),
			Value: awsStd.String(tag.Value),
		})
	}

	_, err = m.ssmClient.AddTagsToResource(ctx, &ssm.AddTagsToResourceInput{
		ResourceType: ssmTypes.ResourceTypeForTaggingParameter,
		ResourceId:   awsStd.String(parameterName),
		Tags:         ssmTags,
	})
	if err != nil {
		return false, fmt.Errorf("failed to update tags: %w", err)
	}

	reqLogger.Debug("updated secret parameter tags", "name", secretName, "parameter", parameterName)
	return true, nil
}

func (m *Manager) findOrphanedParameters(
	ctx context.Context,
	seenParameters map[string]bool,
	_ *slog.Logger,
) ([]string, error) {
	orphaned := []string{}

	nextToken := ""
	for {
		listOutput, err := m.ssmClient.DescribeParameters(ctx, &ssm.DescribeParametersInput{
			ParameterFilters: []ssmTypes.ParameterStringFilter{
				{
					Key:    awsStd.String("Path"),
					Option: awsStd.String("BeginsWith"),
					Values: []string{m.secretsPrefix},
				},
			},
			NextToken:  awsStd.String(nextToken),
			MaxResults: awsStd.Int32(awsConstants.SSMParameterMaxResults),
		})
		if err != nil {
			return orphaned, fmt.Errorf("failed to describe parameters: %w", err)
		}

		for i := range listOutput.Parameters {
			param := &listOutput.Parameters[i]
			if param.Name != nil {
				paramName := *param.Name
				if !seenParameters[paramName] {
					secretName := strings.TrimPrefix(paramName, m.secretsPrefix+"/")
					orphaned = append(orphaned, secretName)
				}
			}
		}

		if listOutput.NextToken == nil {
			break
		}
		nextToken = *listOutput.NextToken
	}

	return orphaned, nil
}

func isParameterNotFound(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "ParameterNotFound") ||
		strings.Contains(errMsg, "InvalidParameter")
}
