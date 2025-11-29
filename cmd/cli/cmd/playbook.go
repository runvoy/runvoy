package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/client"
	"github.com/runvoy/runvoy/internal/client/output"
	"github.com/runvoy/runvoy/internal/client/playbooks"
	"github.com/runvoy/runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var playbookCmd = &cobra.Command{
	Use:   "playbook",
	Short: "Manage and execute playbooks",
	Long:  "Manage and execute reusable command execution configurations defined in YAML files",
}

var playbookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available playbooks",
	Long:  "List all playbooks found in the .runvoy directory",
	Run:   playbookListRun,
}

var playbookShowCmd = &cobra.Command{
	Use:     "show <name>",
	Short:   "Show playbook details",
	Long:    "Display the full content of a playbook",
	Example: fmt.Sprintf(`  - %s playbook show terraform-plan`, constants.ProjectName),
	Run:     playbookShowRun,
	Args:    cobra.ExactArgs(1),
}

var playbookRunCmd = &cobra.Command{
	Use:     "run <name>",
	Short:   "Execute a playbook",
	Long:    "Execute a playbook with optional flag overrides",
	Example: fmt.Sprintf(`  - %s playbook run terraform-plan`, constants.ProjectName),
	Run:     playbookRunRun,
	Args:    cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(playbookCmd)
	playbookCmd.AddCommand(playbookListCmd)
	playbookCmd.AddCommand(playbookShowCmd)
	playbookCmd.AddCommand(playbookRunCmd)

	playbookRunCmd.Flags().StringP("image", "i", "", "Override image")
	playbookRunCmd.Flags().StringP("git-repo", "g", "", "Override git repository URL")
	playbookRunCmd.Flags().StringP("git-ref", "r", "", "Override git reference")
	playbookRunCmd.Flags().StringP("git-path", "p", "", "Override git path")
	playbookRunCmd.Flags().StringSlice("secret", []string{}, "Add additional secrets (merge with playbook secrets)")
}

func playbookListRun(cmd *cobra.Command, _ []string) {
	loader := playbooks.NewPlaybookLoader()
	service := NewPlaybookService(loader, nil, NewOutputWrapper())
	if err := service.ListPlaybooks(cmd.Context()); err != nil {
		output.Errorf(err.Error())
	}
}

func playbookShowRun(cmd *cobra.Command, args []string) {
	name := args[0]
	loader := playbooks.NewPlaybookLoader()
	service := NewPlaybookService(loader, nil, NewOutputWrapper())
	if err := service.ShowPlaybook(cmd.Context(), name); err != nil {
		output.Errorf(err.Error())
	}
}

func playbookRunRun(cmd *cobra.Command, args []string) {
	name := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	loader := playbooks.NewPlaybookLoader()
	executor := playbooks.NewPlaybookExecutor()
	c := client.New(cfg, slog.Default())
	runService := NewRunService(c, NewOutputWrapper())
	service := NewPlaybookService(loader, executor, NewOutputWrapper())

	image, _ := cmd.Flags().GetString("image")
	gitRepo, _ := cmd.Flags().GetString("git-repo")
	gitRef, _ := cmd.Flags().GetString("git-ref")
	gitPath, _ := cmd.Flags().GetString("git-path")
	secrets, _ := cmd.Flags().GetStringSlice("secret")

	userEnv := extractUserEnvVars(os.Environ())

	overrides := &PlaybookOverrides{
		Image:   image,
		GitRepo: gitRepo,
		GitRef:  gitRef,
		GitPath: gitPath,
		Secrets: secrets,
	}

	webURL := ""
	if cfg.WebURL != "" {
		webURL = cfg.WebURL
	}

	if runErr := service.RunPlaybook(cmd.Context(), name, userEnv, overrides, webURL, runService); runErr != nil {
		output.Errorf(runErr.Error())
	}
}

// PlaybookService handles playbook operations
type PlaybookService struct {
	loader   *playbooks.PlaybookLoader
	executor *playbooks.PlaybookExecutor
	output   OutputInterface
}

// NewPlaybookService creates a new PlaybookService
func NewPlaybookService(
	loader *playbooks.PlaybookLoader,
	executor *playbooks.PlaybookExecutor,
	outputter OutputInterface,
) *PlaybookService {
	return &PlaybookService{
		loader:   loader,
		executor: executor,
		output:   outputter,
	}
}

// PlaybookOverrides contains values to override in a playbook
type PlaybookOverrides struct {
	Image   string
	GitRepo string
	GitRef  string
	GitPath string
	Secrets []string
}

// ListPlaybooks lists all available playbooks
func (s *PlaybookService) ListPlaybooks(_ context.Context) error {
	s.output.Infof("Discovering playbooks…")

	names, err := s.loader.ListPlaybooks()
	if err != nil {
		return fmt.Errorf("failed to list playbooks: %w", err)
	}

	if len(names) == 0 {
		s.output.Warningf("No playbooks found in %s directory", constants.PlaybookDirName)
		return nil
	}

	playbookList := make([]*api.Playbook, 0, len(names))
	for _, name := range names {
		pb, loadErr := s.loader.LoadPlaybook(name)
		if loadErr != nil {
			s.output.Warningf("Failed to load playbook %s: %v", name, loadErr)
			continue
		}
		playbookList = append(playbookList, pb)
	}

	rows := s.formatPlaybooks(playbookList, names)

	s.output.Blank()
	s.output.Table(
		[]string{"Name", "Description"},
		rows,
	)
	s.output.Blank()
	s.output.Successf("Found %d playbook(s)", len(playbookList))
	return nil
}

// formatPlaybooks formats playbook data into table rows
func (s *PlaybookService) formatPlaybooks(playbookList []*api.Playbook, names []string) [][]string {
	rows := make([][]string, 0, len(playbookList))
	for i, pb := range playbookList {
		description := pb.Description
		if description == "" {
			description = "-"
		}
		rows = append(rows, []string{
			s.output.Bold(names[i]),
			description,
		})
	}
	return rows
}

// ShowPlaybook displays the full content of a playbook
func (s *PlaybookService) ShowPlaybook(_ context.Context, name string) error {
	pb, err := s.loader.LoadPlaybook(name)
	if err != nil {
		return fmt.Errorf("failed to load playbook: %w", err)
	}

	s.output.Blank()
	s.output.KeyValue("Name", s.output.Bold(name))
	if pb.Description != "" {
		s.output.KeyValue("Description", pb.Description)
	}
	if pb.Image != "" {
		s.output.KeyValue("Image", s.output.Cyan(pb.Image))
	}
	if pb.GitRepo != "" {
		s.output.KeyValue("Git Repository", s.output.Cyan(pb.GitRepo))
	}
	if pb.GitRef != "" {
		s.output.KeyValue("Git Reference", pb.GitRef)
	}
	if pb.GitPath != "" {
		s.output.KeyValue("Git Path", pb.GitPath)
	}
	if len(pb.Secrets) > 0 {
		s.output.KeyValue("Secrets", strings.Join(pb.Secrets, ", "))
	}
	if len(pb.Env) > 0 {
		var envPairs []string
		for k, v := range pb.Env {
			envPairs = append(envPairs, fmt.Sprintf("%s=%s", k, v))
		}
		s.output.KeyValue("Environment Variables", strings.Join(envPairs, ", "))
	}
	s.output.KeyValue("Commands", strings.Join(pb.Commands, " && "))
	s.output.Blank()

	return nil
}

// RunPlaybook executes a playbook with optional overrides
func (s *PlaybookService) RunPlaybook(
	ctx context.Context,
	name string,
	userEnv map[string]string,
	overrides *PlaybookOverrides,
	webURL string,
	runService *RunService,
) error {
	pb, loadErr := s.loader.LoadPlaybook(name)
	if loadErr != nil {
		return fmt.Errorf("failed to load playbook: %w", loadErr)
	}

	s.output.Infof("Executing playbook: %s", s.output.Bold(name))

	applyOverrides(pb, overrides)

	execReq := s.executor.ToExecutionRequest(pb, userEnv, overrides.Secrets)

	req := ExecuteCommandRequest{
		Command: execReq.Command,
		GitRepo: execReq.GitRepo,
		GitRef:  execReq.GitRef,
		GitPath: execReq.GitPath,
		Image:   execReq.Image,
		Env:     execReq.Env,
		Secrets: execReq.Secrets,
		WebURL:  webURL,
	}

	if execErr := runService.ExecuteCommand(ctx, &req); execErr != nil {
		return fmt.Errorf("failed to execute playbook: %w", execErr)
	}

	return nil
}

// applyOverrides applies CLI flag overrides to a playbook
func applyOverrides(pb *api.Playbook, overrides *PlaybookOverrides) {
	if overrides.Image != "" {
		pb.Image = overrides.Image
	}
	if overrides.GitRepo != "" {
		pb.GitRepo = overrides.GitRepo
	}
	if overrides.GitRef != "" {
		pb.GitRef = overrides.GitRef
	}
	if overrides.GitPath != "" {
		pb.GitPath = overrides.GitPath
	}
}
