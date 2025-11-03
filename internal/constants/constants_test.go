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

func TestContainerConstants(t *testing.T) {
	t.Run("container constants are set", func(t *testing.T) {
		assert.Equal(t, "runner", RunnerContainerName)
		assert.Equal(t, "sidecar", SidecarContainerName)
		assert.Equal(t, "workspace", SharedVolumeName)
		assert.Equal(t, "/workspace", SharedVolumePath)
	})
}

func TestEcsStatus(t *testing.T) {
	t.Run("ECS status constants are set", func(t *testing.T) {
		assert.Equal(t, EcsStatus("PROVISIONING"), EcsStatusProvisioning)
		assert.Equal(t, EcsStatus("PENDING"), EcsStatusPending)
		assert.Equal(t, EcsStatus("ACTIVATING"), EcsStatusActivating)
		assert.Equal(t, EcsStatus("RUNNING"), EcsStatusRunning)
		assert.Equal(t, EcsStatus("DEACTIVATING"), EcsStatusDeactivating)
		assert.Equal(t, EcsStatus("STOPPING"), EcsStatusStopping)
		assert.Equal(t, EcsStatus("DEPROVISIONING"), EcsStatusDeprovisioning)
		assert.Equal(t, EcsStatus("STOPPED"), EcsStatusStopped)
	})
}

func TestExecutionStatus(t *testing.T) {
	t.Run("execution status constants are set", func(t *testing.T) {
		assert.Equal(t, ExecutionStatus("RUNNING"), ExecutionRunning)
		assert.Equal(t, ExecutionStatus("SUCCEEDED"), ExecutionSucceeded)
		assert.Equal(t, ExecutionStatus("FAILED"), ExecutionFailed)
		assert.Equal(t, ExecutionStatus("STOPPED"), ExecutionStopped)
	})
}

func TestTerminalExecutionStatuses(t *testing.T) {
	t.Run("returns all terminal statuses", func(t *testing.T) {
		statuses := TerminalExecutionStatuses()

		assert.Len(t, statuses, 3, "Should have 3 terminal statuses")
		assert.Contains(t, statuses, ExecutionSucceeded)
		assert.Contains(t, statuses, ExecutionFailed)
		assert.Contains(t, statuses, ExecutionStopped)
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

func TestWebviewerURL(t *testing.T) {
	t.Run("default URL is set", func(t *testing.T) {
		assert.NotEmpty(t, DefaultWebviewerURL)
		assert.Contains(t, DefaultWebviewerURL, "http", "Webviewer URL should be an HTTP(S) URL")
	})

	t.Run("WebviewerURL constant matches default (backward compatibility)", func(t *testing.T) {
		assert.Equal(t, DefaultWebviewerURL, WebviewerURL,
			"WebviewerURL should equal DefaultWebviewerURL for backward compatibility")
	})
}

func TestClaimURLExpirationMinutes(t *testing.T) {
	assert.Equal(t, 15, ClaimURLExpirationMinutes)
	assert.Greater(t, ClaimURLExpirationMinutes, 0, "Expiration should be positive")
}

func TestClaimEndpointPath(t *testing.T) {
	assert.Equal(t, "/claim", ClaimEndpointPath)
	assert.True(t, len(ClaimEndpointPath) > 0, "Endpoint path should not be empty")
}

func TestTaskDefinitionConstants(t *testing.T) {
	t.Run("task definition constants are set", func(t *testing.T) {
		assert.Equal(t, "runvoy-image", TaskDefinitionFamilyPrefix)
		assert.Equal(t, "IsDefault", TaskDefinitionIsDefaultTagKey)
		assert.Equal(t, "DockerImage", TaskDefinitionDockerImageTagKey)
	})
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
