package playbooks

import (
	"maps"
	"strings"

	"github.com/runvoy/runvoy/internal/api"
)

// PlaybookExecutor converts Playbook to ExecutionRequest
type PlaybookExecutor struct{}

// NewPlaybookExecutor creates a new PlaybookExecutor
func NewPlaybookExecutor() *PlaybookExecutor {
	return &PlaybookExecutor{}
}

// ToExecutionRequest converts a Playbook to an ExecutionRequest.
// Combines multiple commands with && operator and merges env vars and secrets.
func (e *PlaybookExecutor) ToExecutionRequest(
	playbook *api.Playbook,
	userEnv map[string]string,
	userSecrets []string,
) *api.ExecutionRequest {
	command := strings.Join(playbook.Commands, " && ")

	env := make(map[string]string)
	maps.Copy(env, playbook.Env)
	maps.Copy(env, userEnv)

	secrets := make([]string, 0, len(playbook.Secrets)+len(userSecrets))
	secrets = append(secrets, playbook.Secrets...)
	secrets = append(secrets, userSecrets...)

	return &api.ExecutionRequest{
		Command: command,
		Image:   playbook.Image,
		GitRepo: playbook.GitRepo,
		GitRef:  playbook.GitRef,
		GitPath: playbook.GitPath,
		Env:     env,
		Secrets: secrets,
	}
}
