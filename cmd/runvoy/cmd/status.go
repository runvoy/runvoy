package cmd

import (
	"fmt"
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/output"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <execution-id>",
	Short: "Get the status of a command execution",
	Run:   statusRun, Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func statusRun(cmd *cobra.Command, args []string) {
	executionID := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Error("failed to load configuration: %v", err)
		return
	}

	client := client.New(cfg, slog.Default())
	status, err := client.GetExecutionStatus(cmd.Context(), executionID)
	if err != nil {
		output.Error(err.Error())
		return
	}

	output.KeyValue("Execution ID", status.ExecutionID)
	output.KeyValue("Status", status.Status)
	output.KeyValue("Started At", status.StartedAt.Format(time.DateTime))
	if status.CompletedAt != nil {
		output.KeyValue("Completed At", status.CompletedAt.Format(time.DateTime))
	}
	if status.ExitCode != nil {
		output.KeyValue("Exit Code", fmt.Sprintf("%d", *status.ExitCode))
	}
	output.Blank()
	output.Success("Status retrieved successfully")
}
