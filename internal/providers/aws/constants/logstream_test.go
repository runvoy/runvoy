package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildLogStreamName(t *testing.T) {
	tests := []struct {
		name        string
		executionID string
		expected    string
	}{
		{
			name:        "normal execution ID",
			executionID: "abc123",
			expected:    "task/runner/abc123",
		},
		{
			name:        "UUID-like execution ID",
			executionID: "550e8400-e29b-41d4-a716-446655440000",
			expected:    "task/runner/550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:        "empty execution ID",
			executionID: "",
			expected:    "task/runner/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildLogStreamName(tt.executionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractExecutionIDFromLogStream(t *testing.T) {
	tests := []struct {
		name      string
		logStream string
		expected  string
	}{
		{
			name:      "valid runner log stream",
			logStream: "task/runner/abc123",
			expected:  "abc123",
		},
		{
			name:      "valid sidecar log stream",
			logStream: "task/sidecar/abc123",
			expected:  "abc123",
		},
		{
			name:      "UUID-like execution ID",
			logStream: "task/runner/550e8400-e29b-41d4-a716-446655440000",
			expected:  "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:      "empty log stream",
			logStream: "",
			expected:  "",
		},
		{
			name:      "invalid format - too few parts",
			logStream: "task/runner",
			expected:  "",
		},
		{
			name:      "invalid format - too many parts",
			logStream: "task/runner/abc123/extra",
			expected:  "",
		},
		{
			name:      "invalid format - wrong prefix",
			logStream: "log/runner/abc123",
			expected:  "",
		},
		{
			name:      "invalid format - unknown container",
			logStream: "task/unknown/abc123",
			expected:  "",
		},
		{
			name:      "invalid format - empty execution ID",
			logStream: "task/runner/",
			expected:  "",
		},
		{
			name:      "invalid format - no slashes",
			logStream: "taskrunnerabc123",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractExecutionIDFromLogStream(tt.logStream)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildAndExtractLogStreamRoundTrip(t *testing.T) {
	t.Run("build and extract round trip", func(t *testing.T) {
		executionID := "test-exec-123"
		logStream := BuildLogStreamName(executionID)
		extracted := ExtractExecutionIDFromLogStream(logStream)

		assert.Equal(t, executionID, extracted)
	})
}
