package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/client/output"
	"runvoy/internal/constants"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <command>",
	Short: "Run a command",
	Long: `Run a command in a remote environment with optional Git repository cloning.

User environment variables prefixed with RUNVOY_USER_ are saved to .env file
in the command working directory.`,
	Example: fmt.Sprintf(`  - %s run echo hello world
  - %s run terraform plan

  # With private Git repository cloning
  - %s run --secret github-token \
               --git-repo https://github.com/mycompany/myproject.git \
               npm run test

  # With public Git repository cloning and a specific Git reference and path
  - %s run --git-repo https://github.com/ansible/ansible-examples.git \
               --git-ref main \
               --git-path ansible-examples/playbooks/hello_world \
               ansible-playbook site.yml

  # With user environment variables
  - RUNVOY_USER_MY_VAR=1234567890 %s run cat .env # Outputs => MY_VAR=1234567890
`, constants.ProjectName, constants.ProjectName, constants.ProjectName, constants.ProjectName, constants.ProjectName),
	Run:  runRun,
	Args: cobra.MinimumNArgs(1),
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringP("git-repo", "g", "", "Git repository URL")
	runCmd.Flags().StringP("git-ref", "r", "", "Git reference")
	runCmd.Flags().StringP("git-path", "p", "", "Git path")
	runCmd.Flags().StringP("image", "i", "", "Image to use")
	runCmd.Flags().StringSlice("secret", []string{}, "Secret name to inject (repeatable)")
}

func runRun(cmd *cobra.Command, args []string) {
	command := strings.Join(args, " ")
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	envs := extractUserEnvVars(os.Environ())
	gitRepo := cmd.Flag("git-repo").Value.String()
	gitRef := cmd.Flag("git-ref").Value.String()
	gitPath := cmd.Flag("git-path").Value.String()
	image := cmd.Flag("image").Value.String()
	secrets, err := cmd.Flags().GetStringSlice("secret")
	if err != nil {
		output.Fatalf("failed to parse secrets: %v", err)
	}

	c := client.New(cfg, slog.Default())
	service := NewRunService(c, NewOutputWrapper())
	req := ExecuteCommandRequest{
		Command: command,
		GitRepo: gitRepo,
		GitRef:  gitRef,
		GitPath: gitPath,
		Image:   image,
		Env:     envs,
		Secrets: secrets,
		WebURL:  cfg.WebURL,
	}
	if err = service.ExecuteCommand(cmd.Context(), &req); err != nil {
		output.Errorf(err.Error())
	}
}

func extractUserEnvVars(envVars []string) map[string]string {
	envs := make(map[string]string)
	for _, env := range envVars {
		parts := strings.SplitN(env, "=", constants.EnvVarSplitLimit)
		if len(parts) != constants.EnvVarSplitLimit {
			continue
		}

		key := parts[0]
		if after, ok := strings.CutPrefix(key, "RUNVOY_USER_"); ok {
			envs[after] = parts[1]
		}
	}

	return envs
}

// ExecuteCommandRequest contains all parameters needed to execute a command
type ExecuteCommandRequest struct {
	Command string
	GitRepo string
	GitRef  string
	GitPath string
	Image   string
	Env     map[string]string
	Secrets []string
	WebURL  string
}

// RunService handles command execution logic
type RunService struct {
	client client.Interface
	output OutputInterface
}

// NewRunService creates a new RunService with the provided dependencies
func NewRunService(apiClient client.Interface, outputter OutputInterface) *RunService {
	return &RunService{
		client: apiClient,
		output: outputter,
	}
}

// ExecuteCommand executes a command remotely and displays the results
func (s *RunService) ExecuteCommand(ctx context.Context, req *ExecuteCommandRequest) error {
	s.output.Infof("Running command: %s", s.output.Bold(req.Command))
	if req.GitRepo != "" {
		s.output.Infof("Git repository: %s", s.output.Bold(req.GitRepo))
	}
	if req.GitRef != "" {
		s.output.Infof("Git reference: %s", s.output.Bold(req.GitRef))
	}
	if req.GitPath != "" {
		s.output.Infof("Git path: %s", s.output.Bold(req.GitPath))
	}

	var envKeys []string
	for key := range req.Env {
		envKeys = append(envKeys, key)
	}
	if len(envKeys) > 0 {
		sort.Strings(envKeys)
		s.output.Infof("Injecting user environment variables: %s", s.output.Bold(strings.Join(envKeys, ", ")))
	}

	execReq := api.ExecutionRequest{
		Command: req.Command,
		GitRepo: req.GitRepo,
		GitRef:  req.GitRef,
		GitPath: req.GitPath,
		Env:     req.Env,
		Image:   req.Image,
		Secrets: req.Secrets,
	}
	resp, err := s.client.RunCommand(ctx, &execReq)
	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	s.output.Successf("Command execution started successfully")
	s.output.KeyValue("Execution ID", s.output.Cyan(resp.ExecutionID))
	s.output.KeyValue("Status", resp.Status)
	if req.Image != "" {
		s.output.KeyValue("Image", s.output.Cyan(req.Image))
	}

	// Stream logs similar to the logs command
	logsService := NewLogsService(s.client, s.output)
	if serviceErr := logsService.DisplayLogs(ctx, resp.ExecutionID, req.WebURL); serviceErr != nil {
		return fmt.Errorf("failed to stream logs: %w", serviceErr)
	}

	return nil
}
