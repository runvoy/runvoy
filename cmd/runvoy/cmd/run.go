package cmd

import (
	"log/slog"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <command>",
	Short: "Run a command",
	Long:  `Run a command in a remote environment`,
	Run:   runRun,
	Args:  cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) {
	command := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	output.Infof("Running command: %s", output.Bold(command))

	client := client.New(cfg, slog.Default())
	resp, err := client.RunCommand(cmd.Context(), api.ExecutionRequest{Command: command})
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
