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
		result[parts[0]] = parts[1]
	}

	return result, nil
}
