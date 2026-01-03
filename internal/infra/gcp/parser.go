package gcp

import (
	"fmt"
	"os"
	"strings"
	"gopkg.in/yaml.v3"
)

// ParseResourceFile parses GCP resource YAML file with environment variable substitution.
func ParseResourceFile(filePath string) (*ResourceFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource file: %w", err)
	}

	content := string(data)
	content = substituteEnvVars(content)

	var file ResourceFile
	if err := yaml.Unmarshal([]byte(content), &file); err != nil {
		return nil, fmt.Errorf("failed to parse resources: %w", err)
	}

	return &file, nil
}

// substituteEnvVars replaces ${VAR} patterns with environment variable values.
func substituteEnvVars(content string) string {
	result := content

	for {
		startIdx := strings.Index(result, "${")
		if startIdx == -1 {
			break
		}

		endIdx := strings.Index(result[startIdx:], "}")
		if endIdx == -1 {
			break
		}
		endIdx += startIdx

		varName := result[startIdx+3 : endIdx]
		varValue := os.Getenv(varName)

		if varValue == "" {
			result = result[:startIdx] + result[endIdx+1:]
			continue
		}

		result = result[:startIdx] + varValue + result[endIdx+1:]
	}

	return result
}
