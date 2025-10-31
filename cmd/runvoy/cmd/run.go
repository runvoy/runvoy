package cmd

import (
	"fmt"
	"log/slog"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"
	"strings"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <command>",
	Short: "Run a command",
	Long:  `Run a command in a remote environment`,
	Example: fmt.Sprintf(`  - %s run echo hello world
  - %s run terraform plan
  - %s run ansible-playbook site.yml
`, constants.ProjectName, constants.ProjectName, constants.ProjectName),
	Run:  runRun,
	Args: cobra.MinimumNArgs(1),
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringP("git-repo", "g", "", "Git repository URL")
	runCmd.Flags().StringP("git-ref", "r", "", "Git reference")
	runCmd.Flags().StringP("git-path", "p", "", "Git path")
}

func runRun(cmd *cobra.Command, args []string) {
	command := strings.Join(args, " ")
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	output.Infof("Running command: %s", output.Bold(command))
	if gitRepo := cmd.Flag("git-repo").Value.String(); gitRepo != "" {
		output.Infof("Git repository: %s", output.Bold(gitRepo))
	}
	if gitRef := cmd.Flag("git-ref").Value.String(); gitRef != "" {
		output.Infof("Git reference: %s", output.Bold(gitRef))
	}
	if gitPath := cmd.Flag("git-path").Value.String(); gitPath != "" {
		output.Infof("Git path: %s", output.Bold(gitPath))
	}

	client := client.New(cfg, slog.Default())
	resp, err := client.RunCommand(cmd.Context(), api.ExecutionRequest{
		Command: command,
		GitRepo: cmd.Flag("git-repo").Value.String(),
		GitRef:  cmd.Flag("git-ref").Value.String(),
		GitPath: cmd.Flag("git-path").Value.String(),
	})
	if err != nil {
		output.Errorf("failed to run command: %v", err)
		return
	}

	output.Successf("Command execution started successfully")
	output.KeyValue("Execution ID", resp.ExecutionID)
	output.KeyValue("Status", resp.Status)
	output.Infof("View logs in web viewer: %s?execution_id=%s",
		constants.WebviewerURL, output.Cyan(resp.ExecutionID))
}
