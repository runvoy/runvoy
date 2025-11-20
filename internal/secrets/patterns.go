// Package secrets provides shared utilities for secret detection and handling.
package secrets

import "strings"

// DefaultSecretPatterns contains the default patterns used to identify
// environment variable names that should be treated as secrets.
var DefaultSecretPatterns = []string{
	"GITHUB_SECRET",
	"GITHUB_TOKEN",
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"API_KEY",
	"API_SECRET",
	"PRIVATE_KEY",
	"ACCESS_KEY",
	"SECRET_KEY",
}

// GetSecretVariableNames returns a list of variable names from the given environment
// that should be treated as secrets based on pattern matching.
// These variables will be processed without exposing their values in logs.
func GetSecretVariableNames(env map[string]string) []string {
	return GetSecretVariableNamesWithPatterns(env, DefaultSecretPatterns)
}

// GetSecretVariableNamesWithPatterns returns a list of variable names that match
// any of the provided patterns. Pattern matching is case-insensitive.
func GetSecretVariableNamesWithPatterns(env map[string]string, patterns []string) []string {
	secretNames := []string{}

	for key := range env {
		upperKey := strings.ToUpper(key)
		for _, pattern := range patterns {
			if strings.Contains(upperKey, pattern) {
				secretNames = append(secretNames, key)
				break
			}
		}
	}

	return secretNames
}

// MergeSecretVarNames merges known secret variable names with pattern-detected ones,
// removing duplicates. This allows combining explicitly known secrets with
// pattern-based detection for comprehensive coverage.
func MergeSecretVarNames(known, detected []string) []string {
	seen := make(map[string]struct{}, len(known)+len(detected))
	result := make([]string, 0, len(known)+len(detected))

	for _, name := range known {
		if _, exists := seen[name]; !exists {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	for _, name := range detected {
		if _, exists := seen[name]; !exists {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	return result
}
