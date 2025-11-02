package aws

import (
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/constants"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/require"
)

func envPairsToMap(pairs []ecsTypes.KeyValuePair) map[string]string {
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		name := awsStd.ToString(pair.Name)
		value := awsStd.ToString(pair.Value)
		out[name] = value
	}
	return out
}

func TestBuildRunnerEnvironmentIncludesUserVariables(t *testing.T) {
	req := api.ExecutionRequest{
		Command: "echo hello",
		Env: map[string]string{
			"FOO": "bar",
			"BAR": "baz",
		},
	}

	envVars := buildRunnerEnvironment(req, "req-123")
	envMap := envPairsToMap(envVars)

	require.Equal(t, "echo hello", envMap["RUNVOY_COMMAND"])
	require.Equal(t, "req-123", envMap["RUNVOY_REQUEST_ID"])
	require.Equal(t, "bar", envMap["FOO"])
	require.Equal(t, "bar", envMap["RUNVOY_USER_FOO"])
	require.Equal(t, "baz", envMap["BAR"])
	require.Equal(t, "baz", envMap["RUNVOY_USER_BAR"])
}

func TestBuildRunnerEnvironmentSkipsReservedOverrides(t *testing.T) {
	req := api.ExecutionRequest{
		Command: "real command",
		Env: map[string]string{
			"RUNVOY_COMMAND": "user override",
		},
	}

	envVars := buildRunnerEnvironment(req, "")
	envMap := envPairsToMap(envVars)

	require.Equal(t, "real command", envMap["RUNVOY_COMMAND"])
	// The user-provided RUNVOY_COMMAND should be ignored because it's reserved
	require.NotEqual(t, "user override", envMap["RUNVOY_COMMAND"])
	require.Equal(t, "user override", envMap["RUNVOY_USER_RUNVOY_COMMAND"])
}

func TestBuildGitClonerEnvironmentIncludesUserVariables(t *testing.T) {
	req := api.ExecutionRequest{
		GitRepo: "https://github.com/runvoy/runvoy.git",
		Env: map[string]string{
			"GIT_TOKEN": "secret",
		},
	}

	envVars := buildGitClonerEnvironment(req, "main", "req-456")
	envMap := envPairsToMap(envVars)

	require.Equal(t, "https://github.com/runvoy/runvoy.git", envMap["GIT_REPO"])
	require.Equal(t, "main", envMap["GIT_REF"])
	require.Equal(t, constants.SharedVolumePath, envMap["SHARED_VOLUME_PATH"])
	require.Equal(t, "req-456", envMap["RUNVOY_REQUEST_ID"])
	require.Equal(t, "secret", envMap["GIT_TOKEN"])
}

func TestBuildGitClonerEnvironmentSkipsReservedOverrides(t *testing.T) {
	req := api.ExecutionRequest{
		GitRepo: "https://github.com/runvoy/runvoy.git",
		Env: map[string]string{
			"GIT_REPO": "override",
		},
	}

	envVars := buildGitClonerEnvironment(req, "branch", "")
	envMap := envPairsToMap(envVars)

	require.Equal(t, "https://github.com/runvoy/runvoy.git", envMap["GIT_REPO"])
	require.NotEqual(t, "override", envMap["GIT_REPO"])
	require.Equal(t, "branch", envMap["GIT_REF"])
}
