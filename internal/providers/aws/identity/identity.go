// Package identity provides helpers for retrieving AWS identity information.
package identity

import (
	"context"
	"fmt"
	"log/slog"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// GetAccountID retrieves the AWS account ID using STS GetCallerIdentity.
func GetAccountID(ctx context.Context, awsCfg *awsStd.Config, log *slog.Logger) (string, error) {
	stsClient := sts.NewFromConfig(*awsCfg)

	log.Debug("calling external service", "context", map[string]string{
		"operation": "STS.GetCallerIdentity",
	})

	output, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("STS GetCallerIdentity failed: %w", err)
	}

	if output.Account == nil || *output.Account == "" {
		return "", fmt.Errorf("STS returned empty account ID")
	}

	accountID := *output.Account

	return accountID, nil
}
