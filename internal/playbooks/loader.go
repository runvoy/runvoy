// Package playbooks provides functionality for loading and managing playbooks.
package playbooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/constants"

	"gopkg.in/yaml.v3"
)

// PlaybookLoader handles loading and discovery of playbooks
type PlaybookLoader struct{}

// NewPlaybookLoader creates a new PlaybookLoader
func NewPlaybookLoader() *PlaybookLoader {
	return &PlaybookLoader{}
}

// GetPlaybookDir returns the path to the playbook directory.
// Checks current working directory first, falls back to home directory.
func (l *PlaybookLoader) GetPlaybookDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	playbookDir := filepath.Join(cwd, constants.PlaybookDirName)
	if _, err := os.Stat(playbookDir); err == nil {
		return playbookDir, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	playbookDir = filepath.Join(homeDir, constants.PlaybookDirName)
	return playbookDir, nil
}

// ListPlaybooks scans the playbook directory for YAML files and returns playbook names.
// Returns empty list if directory doesn't exist.
func (l *PlaybookLoader) ListPlaybooks() ([]string, error) {
	playbookDir, err := l.GetPlaybookDir()
	if err != nil {
		return []string{}, nil
	}

	if _, err := os.Stat(playbookDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(playbookDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read playbook directory: %w", err)
	}

	var playbooks []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		for _, validExt := range constants.PlaybookFileExtensions {
			if ext == validExt {
				name := strings.TrimSuffix(entry.Name(), ext)
				playbooks = append(playbooks, name)
				break
			}
		}
	}

	return playbooks, nil
}

// LoadPlaybook loads and parses a playbook YAML file.
func (l *PlaybookLoader) LoadPlaybook(name string) (*api.Playbook, error) {
	playbookDir, err := l.GetPlaybookDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get playbook directory: %w", err)
	}

	var playbookPath string
	var found bool
	for _, ext := range constants.PlaybookFileExtensions {
		candidatePath := filepath.Join(playbookDir, name+ext)
		if _, err := os.Stat(candidatePath); err == nil {
			playbookPath = candidatePath
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("playbook not found: %s", name)
	}

	data, err := os.ReadFile(playbookPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read playbook file: %w", err)
	}

	var playbook api.Playbook
	if err := yaml.Unmarshal(data, &playbook); err != nil {
		return nil, fmt.Errorf("failed to parse playbook YAML in %s: %w", playbookPath, err)
	}

	if err := l.validatePlaybook(&playbook); err != nil {
		return nil, fmt.Errorf("invalid playbook %s: %w", name, err)
	}

	return &playbook, nil
}

// validatePlaybook validates that a playbook has required fields
func (l *PlaybookLoader) validatePlaybook(p *api.Playbook) error {
	if len(p.Commands) == 0 {
		return fmt.Errorf("commands must not be empty")
	}
	return nil
}
