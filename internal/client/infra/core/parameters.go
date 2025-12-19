// Package core provides shared types and helpers for infrastructure deployments.
package core

import (
	"fmt"
	"strings"
)

const parameterSplitParts = 2

// ParseParameters parses KEY=VALUE parameter strings.
func ParseParameters(params []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, param := range params {
		parts := strings.SplitN(param, "=", parameterSplitParts)
		if len(parts) != parameterSplitParts {
			return nil, fmt.Errorf("invalid parameter format: %s (expected KEY=VALUE)", param)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		result[key] = value
	}

	return result, nil
}
