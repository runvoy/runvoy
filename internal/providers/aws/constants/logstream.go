// Package constants provides AWS-specific constants and utilities for log stream handling.
package constants

import (
	"strings"
)

// BuildLogStreamName constructs a CloudWatch Logs stream name for an execution.
// Format: task/{container}/{execution_id}
// Example: task/runner/abc123
func BuildLogStreamName(executionID string) string {
	return "task/" + RunnerContainerName + "/" + executionID
}

// ExtractExecutionIDFromLogStream extracts the execution ID from a CloudWatch Logs stream name.
// Expected format: task/{container}/{execution_id}
// Returns empty string if the format is not recognized.
func ExtractExecutionIDFromLogStream(logStream string) string {
	if logStream == "" {
		return ""
	}

	parts := strings.Split(logStream, "/")
	if len(parts) != LogStreamPartsCount {
		return ""
	}

	if parts[0] != LogStreamPrefix {
		return ""
	}

	if parts[1] != RunnerContainerName && parts[1] != SidecarContainerName {
		return ""
	}

	executionID := parts[2]
	if executionID == "" {
		return ""
	}

	return executionID
}
