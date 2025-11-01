package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

const (
	functionName = "runvoy-orchestrator"
	envFile      = ".env"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("error: failed to load AWS configuration: %v", err)
	}

	lambdaClient := lambda.NewFromConfig(awsCfg)
	
	// Get the Lambda function configuration
	functionConfig, err := lambdaClient.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		log.Fatalf("error: failed to get Lambda function configuration: %v", err)
	}

	// Extract environment variables
	envVars := make(map[string]string)
	if functionConfig.Environment != nil && functionConfig.Environment.Variables != nil {
		for k, v := range functionConfig.Environment.Variables {
			envVars[k] = v
		}
	}

	if len(envVars) == 0 {
		log.Fatalf("error: no environment variables found for Lambda function %s", functionName)
	}

	// Write to .env file
	envContent := buildEnvFile(envVars)
	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		log.Fatalf("error: failed to write .env file: %v", err)
	}

	log.Printf("successfully synced %d environment variables from %s to .env", len(envVars), functionName)
}

// buildEnvFile creates the content for the .env file with sorted keys for consistency
func buildEnvFile(envVars map[string]string) string {
	var sb strings.Builder
	
	// Sort keys for consistent output
	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Write each key-value pair
	for _, key := range keys {
		value := envVars[key]
		// Escape quotes in values if needed
		if strings.Contains(value, "\"") || strings.Contains(value, " ") || strings.Contains(value, "#") {
			value = fmt.Sprintf("\"%s\"", value)
		}
		sb.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}

	return sb.String()
}
