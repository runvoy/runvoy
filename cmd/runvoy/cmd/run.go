package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"
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

  # With Git repository cloning
  - %s run --git-repo https://github.com/mycompany/myproject.git npm run test

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
	envs := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", constants.EnvVarSplitLimit)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "RUNVOY_USER_") {
			envs[strings.TrimPrefix(parts[0], "RUNVOY_USER_")] = parts[1]
		}
	}
	var envKeys []string
	for key := range envs {
		envKeys = append(envKeys, key)
	}
	if len(envKeys) > 0 {
		sort.Strings(envKeys)
		output.Infof("Injecting user environment variables: %s", output.Bold(strings.Join(envKeys, ", ")))
	}

	c := client.New(cfg, slog.Default())
	resp, err := c.RunCommand(cmd.Context(), api.ExecutionRequest{
		Command: command,
		GitRepo: cmd.Flag("git-repo").Value.String(),
		GitRef:  cmd.Flag("git-ref").Value.String(),
		GitPath: cmd.Flag("git-path").Value.String(),
		Env:     envs,
		Image:   cmd.Flag("image").Value.String(),
	})
	if err != nil {
		output.Errorf("failed to run command: %v", err)
		return
	}

	output.Successf("Command execution started successfully")
	output.KeyValue("Execution ID", output.Cyan(resp.ExecutionID))
	output.KeyValue("Status", resp.Status)
	if image := cmd.Flag("image").Value.String(); image != "" {
		output.KeyValue("Image", output.Cyan(image))
	}
	webviewerURL := cfg.GetWebviewerURL()
	output.Infof("View logs in web viewer: %s?execution_id=%s",
		webviewerURL, output.Cyan(resp.ExecutionID))
}
