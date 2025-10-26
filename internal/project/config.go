package project

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the .mycli.yaml project configuration file
type Config struct {
	Repo           string            `yaml:"repo"`
	Branch         string            `yaml:"branch,omitempty"`
	Image          string            `yaml:"image,omitempty"`
	Env            map[string]string `yaml:"env,omitempty"`
	TimeoutSeconds int               `yaml:"timeout,omitempty"`
}

// Load reads the .mycli.yaml file from the specified directory
func Load(dir string) (*Config, error) {
	path := dir + "/.mycli.yaml"
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(".mycli.yaml not found in %s", dir)
		}
		return nil, fmt.Errorf("failed to read .mycli.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse .mycli.yaml: %w", err)
	}

	return &cfg, nil
}

// LoadFromCurrentDir loads the .mycli.yaml from the current working directory
func LoadFromCurrentDir() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return Load(cwd)
}

// Exists checks if .mycli.yaml exists in the specified directory
func Exists(dir string) bool {
	path := dir + "/.mycli.yaml"
	_, err := os.Stat(path)
	return err == nil
}

// ExistsInCurrentDir checks if .mycli.yaml exists in the current working directory
func ExistsInCurrentDir() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	return Exists(cwd)
}

// Save writes the configuration to a .mycli.yaml file
func Save(cfg *Config, dir string) error {
	path := dir + "/.mycli.yaml"
	
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write .mycli.yaml: %w", err)
	}

	return nil
}

// SaveToCurrentDir writes the configuration to the current working directory
func SaveToCurrentDir(cfg *Config) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return Save(cfg, cwd)
}

// Merge merges command-line overrides with project config
// CLI flags take precedence over config file values
func (c *Config) Merge(cliRepo, cliBranch, cliImage string, cliEnv map[string]string, cliTimeout int) {
	if cliRepo != "" {
		c.Repo = cliRepo
	}
	if cliBranch != "" {
		c.Branch = cliBranch
	}
	if cliImage != "" {
		c.Image = cliImage
	}
	if cliTimeout > 0 {
		c.TimeoutSeconds = cliTimeout
	}

	// Merge environment variables (CLI env vars override config env vars)
	if len(cliEnv) > 0 {
		if c.Env == nil {
			c.Env = make(map[string]string)
		}
		for k, v := range cliEnv {
			c.Env[k] = v
		}
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Repo == "" {
		return fmt.Errorf("repo is required")
	}
	return nil
}
