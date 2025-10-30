package cmd

import (
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/output"
	"time"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Get logs for an execution",
	Long:  `Get logs for an execution`,
	Run:   logsRun,
	Args:  cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func logsRun(cmd *cobra.Command, args []string) {
	output.Header("🚀 " + constants.ProjectName)

	executionID := args[0]
	cfg, err := config.Load()
	if err != nil {
		output.Error("failed to load configuration: %v", err)
		return
	}

	output.Info("Getting logs for execution: %s", output.Bold(executionID))
	if verbose {
		output.Info("Endpoint: %s", output.Bold(cfg.APIEndpoint))
	}

	client := client.New(cfg, slog.Default())
	resp, err := client.GetLogs(cmd.Context(), executionID)
	if err != nil {
		output.Error("failed to get logs: %v", err)
		return
	}

	output.Blank()
	rows := [][]string{}
	for _, log := range resp.Events {
		rows = append(rows, []string{
			time.Unix(log.Timestamp/1000, 0).UTC().Format(time.DateTime),
			log.Message,
		})
	}
	output.Table([]string{"Timestamp (UTC)", "Message"}, rows)
	output.Blank()
	output.Success("Logs retrieved successfully")
}
