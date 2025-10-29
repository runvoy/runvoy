package cmd

import (
	"log/slog"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a command",
	Long:  `Run a command in a remote environment`,
	Run:   runRun,
	Args:  cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) {
	output.Header("🚀 " + constants.ProjectName)

	command := args[0]
	cfg, err := config.Load()
	if err != nil {
		output.Error("failed to load configuration: %v", err)
		return
	}

	output.Info("Running command: %s", output.Bold(command))
	if verbose {
		output.Info("Endpoint: %s", output.Bold(cfg.APIEndpoint))
	}

	client := client.New(cfg, slog.Default())
	resp, err := client.RunCommand(cmd.Context(), api.ExecutionRequest{Command: command})
	if err != nil {
		output.Error("failed to run command: %v", err)
		return
	}

	output.Success("Command execution started successfully")
	output.KeyValue("Execution ID", resp.ExecutionID)
	output.KeyValue("Log URL", resp.LogURL)
	output.KeyValue("Status", resp.Status)
}
