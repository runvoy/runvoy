package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	v := GetVersion()
	assert.NotNil(t, v, "Version should not be nil")
	assert.NotEmpty(t, *v, "Version should not be empty")

	// Check that it returns a pointer to the same variable
	v2 := GetVersion()
	assert.Equal(t, v, v2, "GetVersion should return the same pointer")
}

func TestConfigDirPath(t *testing.T) {
	tests := []struct {
		name     string
		homeDir  string
		expected string
	}{
		{
			name:     "standard home directory",
			homeDir:  "/home/user",
			expected: "/home/user/.runvoy",
		},
		{
			name:     "root home directory",
			homeDir:  "/root",
			expected: "/root/.runvoy",
		},
		{
			name:     "empty home directory",
			homeDir:  "",
			expected: "/.runvoy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConfigDirPath(tt.homeDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigFilePath(t *testing.T) {
	tests := []struct {
		name     string
		homeDir  string
		expected string
	}{
		{
			name:     "standard home directory",
			homeDir:  "/home/user",
			expected: "/home/user/.runvoy/config.yaml",
		},
		{
			name:     "root home directory",
			homeDir:  "/root",
			expected: "/root/.runvoy/config.yaml",
		},
		{
			name:     "empty home directory",
			homeDir:  "",
			expected: "/.runvoy/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConfigFilePath(tt.homeDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackendProvider(t *testing.T) {
	t.Run("AWS constant is set", func(t *testing.T) {
		assert.Equal(t, BackendProvider("AWS"), AWS)
	})
}

func TestEnvironment(t *testing.T) {
	t.Run("environment constants are set", func(t *testing.T) {
		assert.Equal(t, Environment("development"), Development)
		assert.Equal(t, Environment("production"), Production)
		assert.Equal(t, Environment("cli"), CLI)
	})
}

func TestConstants(t *testing.T) {
	t.Run("project constants are set correctly", func(t *testing.T) {
		assert.Equal(t, "runvoy", ProjectName)
		assert.Equal(t, ".runvoy", ConfigDirName)
		assert.Equal(t, "config.yaml", ConfigFileName)
		assert.Equal(t, "X-API-Key", APIKeyHeader)
		assert.Equal(t, "Content-Type", ContentTypeHeader)
	})
}

func TestServiceConstants(t *testing.T) {
	t.Run("service constants are set", func(t *testing.T) {
		assert.Equal(t, Service("orchestrator"), OrchestratorService)
		assert.Equal(t, Service("event-processor"), EventProcessorService)
	})
}

// Container and ECS status constants have been moved to internal/providers/aws/constants/
// See awsConstants = internal/providers/aws/constants/ for ECS-specific tests

func TestExecutionStatus(t *testing.T) {
	t.Run("execution status constants are set", func(t *testing.T) {
		assert.Equal(t, ExecutionStatus("STARTING"), ExecutionStarting)
		assert.Equal(t, ExecutionStatus("RUNNING"), ExecutionRunning)
		assert.Equal(t, ExecutionStatus("SUCCEEDED"), ExecutionSucceeded)
		assert.Equal(t, ExecutionStatus("FAILED"), ExecutionFailed)
		assert.Equal(t, ExecutionStatus("STOPPED"), ExecutionStopped)
		assert.Equal(t, ExecutionStatus("TERMINATING"), ExecutionTerminating)
	})
}

func TestTerminalExecutionStatuses(t *testing.T) {
	t.Run("returns all terminal statuses", func(t *testing.T) {
		statuses := TerminalExecutionStatuses()

		assert.Len(t, statuses, 4, "Should have 4 terminal statuses")
		assert.Contains(t, statuses, ExecutionSucceeded)
		assert.Contains(t, statuses, ExecutionFailed)
		assert.Contains(t, statuses, ExecutionStopped)
		assert.Contains(t, statuses, ExecutionTerminating)
		assert.NotContains(t, statuses, ExecutionRunning, "RUNNING should not be terminal")
	})

	t.Run("terminal statuses are unique", func(t *testing.T) {
		statuses := TerminalExecutionStatuses()
		seen := make(map[ExecutionStatus]bool)

		for _, status := range statuses {
			assert.False(t, seen[status], "Status %s appears multiple times", status)
			seen[status] = true
		}
	})
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     ExecutionStatus
		to       ExecutionStatus
		expected bool
	}{
		// Valid transitions from STARTING
		{
			name:     "STARTING to RUNNING",
			from:     ExecutionStarting,
			to:       ExecutionRunning,
			expected: true,
		},
		{
			name:     "STARTING to FAILED",
			from:     ExecutionStarting,
			to:       ExecutionFailed,
			expected: true,
		},
		// Invalid transitions from STARTING
		{
			name:     "STARTING to SUCCEEDED",
			from:     ExecutionStarting,
			to:       ExecutionSucceeded,
			expected: false,
		},
		{
			name:     "STARTING to STOPPED",
			from:     ExecutionStarting,
			to:       ExecutionStopped,
			expected: false,
		},
		{
			name:     "STARTING to TERMINATING",
			from:     ExecutionStarting,
			to:       ExecutionTerminating,
			expected: true,
		},
		// Valid transitions from RUNNING
		{
			name:     "RUNNING to SUCCEEDED",
			from:     ExecutionRunning,
			to:       ExecutionSucceeded,
			expected: true,
		},
		{
			name:     "RUNNING to FAILED",
			from:     ExecutionRunning,
			to:       ExecutionFailed,
			expected: true,
		},
		{
			name:     "RUNNING to STOPPED",
			from:     ExecutionRunning,
			to:       ExecutionStopped,
			expected: true,
		},
		{
			name:     "RUNNING to TERMINATING",
			from:     ExecutionRunning,
			to:       ExecutionTerminating,
			expected: true,
		},
		// Invalid transitions from RUNNING
		{
			name:     "RUNNING to STARTING",
			from:     ExecutionRunning,
			to:       ExecutionStarting,
			expected: false,
		},
		// Valid transitions from TERMINATING
		{
			name:     "TERMINATING to STOPPED",
			from:     ExecutionTerminating,
			to:       ExecutionStopped,
			expected: true,
		},
		// Invalid transitions from TERMINATING
		{
			name:     "TERMINATING to RUNNING",
			from:     ExecutionTerminating,
			to:       ExecutionRunning,
			expected: false,
		},
		{
			name:     "TERMINATING to SUCCEEDED",
			from:     ExecutionTerminating,
			to:       ExecutionSucceeded,
			expected: false,
		},
		// Terminal states cannot transition
		{
			name:     "SUCCEEDED to any status",
			from:     ExecutionSucceeded,
			to:       ExecutionRunning,
			expected: false,
		},
		{
			name:     "FAILED to any status",
			from:     ExecutionFailed,
			to:       ExecutionRunning,
			expected: false,
		},
		{
			name:     "STOPPED to any status",
			from:     ExecutionStopped,
			to:       ExecutionRunning,
			expected: false,
		},
		// Same status (no-op transitions)
		{
			name:     "STARTING to STARTING",
			from:     ExecutionStarting,
			to:       ExecutionStarting,
			expected: false,
		},
		{
			name:     "RUNNING to RUNNING",
			from:     ExecutionRunning,
			to:       ExecutionRunning,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanTransition(tt.from, tt.to)
			assert.Equal(t, tt.expected, result,
				"CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.expected)
		})
	}
}

func TestWebURL(t *testing.T) {
	t.Run("default URL is set", func(t *testing.T) {
		assert.NotEmpty(t, DefaultWebURL)
		assert.Contains(t, DefaultWebURL, "http", "Web URL should be an HTTP(S) URL")
	})
}

func TestClaimURLExpirationMinutes(t *testing.T) {
	assert.Equal(t, 15, ClaimURLExpirationMinutes)
	assert.Greater(t, ClaimURLExpirationMinutes, 0, "Expiration should be positive")
}

func TestContextKeys(t *testing.T) {
	t.Run("context key types are unique", func(t *testing.T) {
		configKey := ConfigCtxKey
		startTimeKey := StartTimeCtxKey

		// These should be different types/values
		assert.NotEqual(t, string(configKey), string(startTimeKey))
	})

	t.Run("context key values are set", func(t *testing.T) {
		assert.Equal(t, ConfigCtxKeyType("config"), ConfigCtxKey)
		assert.Equal(t, StartTimeCtxKeyType("startTime"), StartTimeCtxKey)
	})
}

// Log stream building and extraction tests have been moved to internal/providers/aws/constants/
// See internal/providers/aws/constants/ for ECS log stream tests
