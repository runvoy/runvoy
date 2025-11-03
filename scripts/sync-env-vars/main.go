// Package main provides a utility to synchronize environment variables between Lambda functions and local .env files.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"runvoy/internal/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

const (
	functionName = "runvoy-orchestrator"
	envFile      = ".env"
)

var (
	// envLineRegex matches a key-value pair in .env format: KEY=VALUE or KEY="VALUE"
	envLineRegex = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ScriptContextTimeout)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	cancel()
	if err != nil {
		log.Fatalf("error: failed to load AWS configuration: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), constants.ScriptContextTimeout)

	lambdaClient := lambda.NewFromConfig(awsCfg)

	functionConfig, err := lambdaClient.GetFunctionConfiguration(ctx2, &lambda.GetFunctionConfigurationInput{
		FunctionName: aws.String(functionName),
	})
	cancel2()
	if err != nil {
		log.Fatalf("error: failed to get Lambda function configuration: %v", err)
	}

	lambdaVars := make(map[string]string)
	if functionConfig.Environment != nil && functionConfig.Environment.Variables != nil {
		for k, v := range functionConfig.Environment.Variables {
			lambdaVars[k] = v
		}
	}

	if len(lambdaVars) == 0 {
		log.Fatalf("error: no environment variables found for Lambda function %s", functionName)
	}

	totalCount := len(lambdaVars)
	envContent, updatedCount, newCount, err := mergeEnvFile(envFile, lambdaVars)
	if err != nil {
		log.Fatalf("error: failed to merge .env file: %v", err)
	}

	// Write merged content back to .env file
	if err = os.WriteFile(envFile, []byte(envContent), constants.ConfigFilePermissions); err != nil {
		log.Fatalf("error: failed to write .env file: %v", err)
	}

	log.Printf(
		"successfully synced %d environment variables from %s to .env (%d updated, %d new)",
		totalCount, functionName, updatedCount, newCount,
	)
}

// readExistingEnvFile reads the existing .env file and returns lines.
func readExistingEnvFile(filePath string) ([]string, error) {
	var lines []string

	file, err := os.Open(filePath) //nolint:gosec // G304: File path from CLI arg is intentional
	if err == nil {
		defer func() {
			_ = file.Close()
		}()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err = scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading .env file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("error opening .env file: %w", err)
	}

	return lines, nil
}

// processExistingLines processes existing lines and updates with Lambda vars.
func processExistingLines(lines []string, lambdaVars map[string]string) (content strings.Builder, updated int) {
	var result strings.Builder
	updatedCount := 0

	for i, line := range lines {
		matches := envLineRegex.FindStringSubmatch(line)
		if len(matches) == constants.RegexMatchCountEnvVar {
			key := matches[1]
			if newValue, exists := lambdaVars[key]; exists {
				formattedValue := formatEnvValue(newValue)
				result.WriteString(fmt.Sprintf("%s=%s\n", key, formattedValue))
				updatedCount++
				delete(lambdaVars, key)
			} else {
				result.WriteString(line + "\n")
			}
		} else {
			result.WriteString(line + "\n")
		}

		if i == len(lines)-1 && len(lambdaVars) > 0 {
			result.WriteString("\n")
		}
	}

	return result, updatedCount
}

// appendNewVars appends new variables from Lambda that weren't in the existing file.
func appendNewVars(result *strings.Builder, lambdaVars map[string]string, hasExistingLines bool) int {
	if len(lambdaVars) == 0 {
		return 0
	}

	keys := make([]string, 0, len(lambdaVars))
	for k := range lambdaVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if hasExistingLines {
		result.WriteString("# Synced from Lambda function\n")
	}

	newCount := 0
	for _, key := range keys {
		formattedValue := formatEnvValue(lambdaVars[key])
		fmt.Fprintf(result, "%s=%s\n", key, formattedValue)
		newCount++
	}

	return newCount
}

// mergeEnvFile reads the existing .env file (if it exists) and merges it with Lambda values.
// It preserves comments, blank lines, and formatting while updating existing values and adding new ones.
// Returns: merged content, count of updated vars, count of new vars, error
func mergeEnvFile(filePath string, lambdaVars map[string]string) (content string, updated, added int, err error) {
	lines, err := readExistingEnvFile(filePath)
	if err != nil {
		return "", 0, 0, err
	}

	result, updatedCount := processExistingLines(lines, lambdaVars)
	newCount := appendNewVars(&result, lambdaVars, len(lines) > 0)

	return result.String(), updatedCount, newCount, nil
}

// formatEnvValue formats a value for .env file output, adding quotes if necessary.
// Handles values that contain spaces, quotes, or special characters.
func formatEnvValue(value string) string {
	// If value contains quotes, spaces, or starts with #, wrap in quotes
	if strings.Contains(value, "\"") || strings.Contains(value, " ") ||
		strings.Contains(value, "#") || strings.Contains(value, "\n") ||
		strings.Contains(value, "\t") || strings.HasPrefix(value, "'") {
		// Escape quotes in the value
		escaped := strings.ReplaceAll(value, "\"", "\\\"")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		escaped = strings.ReplaceAll(escaped, "\t", "\\t")
		return fmt.Sprintf("%q", escaped)
	}
	return value
}
