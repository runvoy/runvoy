// Package api defines the API types and structures used across runvoy.
package api

// Playbook represents a reusable command execution configuration
type Playbook struct {
	Description string            `yaml:"description,omitempty"`
	Image       string            `yaml:"image,omitempty"`
	GitRepo     string            `yaml:"git_repo,omitempty"`
	GitRef      string            `yaml:"git_ref,omitempty"`
	GitPath     string            `yaml:"git_path,omitempty"`
	Secrets     []string          `yaml:"secrets,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	Commands    []string          `yaml:"commands"`
}
