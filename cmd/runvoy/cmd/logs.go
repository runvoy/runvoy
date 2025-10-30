package cmd

import (
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/output"
	"time"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <execution-id>",
	Short: "Get logs for an execution",
	Long:  `Get logs for an execution`,
	Run:   logsRun,
	PostRun: func(cmd *cobra.Command, _ []string) {
		output.Blank()
	},
	Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func logsRun(cmd *cobra.Command, args []string) {
	executionID := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Error("failed to load configuration: %v", err)
		return
	}

	output.Info("Getting logs for execution: %s", output.Bold(executionID))

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
