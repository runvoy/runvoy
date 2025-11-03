// Package main provides a utility to create a configuration file for runvoy.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"runvoy/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("error: usage: %s <stack-name>", os.Args[0])
	}

	stackName := os.Args[1]
	if stackName == "" {
		log.Fatalf("error: stack name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("error: failed to load AWS configuration: %v", err)
	}

	cfnClient := cloudformation.NewFromConfig(awsCfg)
	apiEndpoint, err := getAPIEndpointFromStack(ctx, cfnClient, stackName)
	if err != nil {
		log.Fatalf("error: failed to resolve API endpoint from CloudFormation outputs: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		// Config doesn't exist yet, create a new one
		cfg = &config.Config{
			APIEndpoint: apiEndpoint,
			APIKey:      "", // API key will be set separately (e.g., via seed-admin-user)
		}
	} else {
		// Update existing config with new endpoint
		cfg.APIEndpoint = apiEndpoint
	}

	if err := config.Save(cfg); err != nil {
		log.Fatalf("error: failed to save config file: %v", err)
	}

	log.Printf("config file updated with API endpoint: %s", apiEndpoint)
}

func getAPIEndpointFromStack(ctx context.Context, client *cloudformation.Client, stackName string) (string, error) {
	output, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe stack: %w", err)
	}

	if len(output.Stacks) == 0 {
		return "", fmt.Errorf("stack %s not found", stackName)
	}

	stack := output.Stacks[0]
	for _, out := range stack.Outputs {
		if out.OutputKey != nil && *out.OutputKey == "APIEndpoint" {
			if out.OutputValue != nil {
				return *out.OutputValue, nil
			}
		}
	}

	return "", fmt.Errorf("APIEndpoint output not found in stack %s", stackName)
}