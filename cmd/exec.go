package cmd

import (
	"context"
	"fmt"
	"strings"

	"mycli/internal/api"
	internalConfig "mycli/internal/config"
	"mycli/internal/git"
	"mycli/internal/project"

	"github.com/spf13/cobra"
)

var (
	repo       string
	branch     string
	image      string
	envVars    []string
	timeout    int
)

var execCmd = &cobra.Command{
	Use:   "exec [flags] \"command\"",
	Short: "Execute a command remotely",
	Long: `Execute a command in a remote container with Git repository support.

Configuration Priority (highest to lowest):
  1. Command-line flags (--repo, --branch, --image, --env)
  2. .mycli.yaml in current directory
  3. Git remote URL (auto-detected via 'git remote get-url origin')
  4. Error if no repo specified

Examples:
  # With .mycli.yaml (simplest)
  mycli exec "terraform plan"

  # Override specific settings
  mycli exec --branch=dev "terraform plan"
  mycli exec --env TF_VAR_region=us-west-2 "terraform apply"

  # Without .mycli.yaml (explicit)
  mycli exec --repo=https://github.com/user/infra "terraform apply"

  # Auto-detect from git remote
  cd my-git-repo  # no .mycli.yaml, but has git remote
  mycli exec "make deploy"
`,
	Args: cobra.ExactArgs(1),
	RunE: runExec,
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringVar(&repo, "repo", "", "Git repository URL (overrides .mycli.yaml and git remote)")
	execCmd.Flags().StringVar(&branch, "branch", "", "Git branch to checkout (overrides .mycli.yaml)")
	execCmd.Flags().StringVar(&image, "image", "", "Docker image to use (overrides .mycli.yaml)")
	execCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables KEY=VALUE (merges with .mycli.yaml)")
	execCmd.Flags().IntVar(&timeout, "timeout", 0, "Timeout in seconds (overrides .mycli.yaml)")
}

func runExec(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	command := args[0]

	// Load global config
	cfg, err := internalConfig.Load()
	if err != nil {
		return fmt.Errorf("not configured. Run 'mycli init' first: %w", err)
	}

	// Build execution config from multiple sources
	execConfig, err := buildExecutionConfig()
	if err != nil {
		return err
	}

	// Parse CLI environment variables
	cliEnv := make(map[string]string)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid env var format: %s (expected KEY=VALUE)", env)
		}
		cliEnv[parts[0]] = parts[1]
	}

	// Merge CLI overrides
	execConfig.Merge(repo, branch, image, cliEnv, timeout)

	// Validate final config
	if err := execConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Show what we're executing
	fmt.Println("→ Repository:", execConfig.Repo)
	if execConfig.Branch != "" {
		fmt.Println("→ Branch:", execConfig.Branch)
	} else {
		fmt.Println("→ Branch: main (default)")
	}
	if execConfig.Image != "" {
		fmt.Println("→ Image:", execConfig.Image)
	}
	if len(execConfig.Env) > 0 {
		fmt.Printf("→ Environment variables: %d set\n", len(execConfig.Env))
	}
	fmt.Println("→ Command:", command)
	fmt.Println()

	// Call API to start execution
	fmt.Println("→ Starting execution...")
	apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)

	resp, err := apiClient.Exec(ctx, api.ExecRequest{
		Repo:           execConfig.Repo,
		Branch:         execConfig.Branch,
		Command:        command,
		Image:          execConfig.Image,
		Env:            execConfig.Env,
		TimeoutSeconds: execConfig.TimeoutSeconds,
	})
	if err != nil {
		return fmt.Errorf("failed to start execution: %w", err)
	}

	fmt.Printf("✓ Execution started\n\n")
	fmt.Println("Execution Details:")
	fmt.Printf("  Execution ID: %s\n", resp.ExecutionID)
	fmt.Printf("  Task ARN:     %s\n", resp.TaskArn)
	if resp.LogStream != "" {
		fmt.Printf("  Log Stream:   %s\n", resp.LogStream)
	}
	fmt.Println()
	fmt.Println("Monitor execution:")
	fmt.Printf("  mycli status %s\n", resp.TaskArn)
	fmt.Printf("  mycli logs %s\n", resp.ExecutionID)
	fmt.Printf("  mycli logs -f %s  # Follow logs in real-time\n", resp.ExecutionID)

	return nil
}

// buildExecutionConfig builds the execution configuration from multiple sources
// Priority: CLI flags > .mycli.yaml > Git auto-detect > Error
func buildExecutionConfig() (*project.Config, error) {
	execConfig := &project.Config{}

	// Try to load .mycli.yaml
	if project.ExistsInCurrentDir() {
		projectConfig, err := project.LoadFromCurrentDir()
		if err != nil {
			return nil, fmt.Errorf("failed to load .mycli.yaml: %w", err)
		}
		execConfig = projectConfig
		fmt.Println("→ Loaded configuration from .mycli.yaml")
	} else if git.IsGitRepository() {
		// Auto-detect from git remote
		remoteURL, detectedBranch, err := git.GetRepositoryInfo()
		if err == nil {
			execConfig.Repo = remoteURL
			execConfig.Branch = detectedBranch
			fmt.Printf("→ Auto-detected git repository: %s\n", remoteURL)
			fmt.Printf("→ Auto-detected branch: %s\n", detectedBranch)
		} else {
			// Not fatal - user can still provide --repo flag
			fmt.Println("→ No .mycli.yaml found and git auto-detect failed")
			fmt.Println("  (You can specify --repo flag)")
		}
	} else {
		fmt.Println("→ No .mycli.yaml found and not a git repository")
		fmt.Println("  (You must specify --repo flag)")
	}

	return execConfig, nil
}
