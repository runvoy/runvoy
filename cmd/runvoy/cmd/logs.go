package cmd

import (
	"fmt"
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"
	"time"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <execution-id>",
	Short: "Get logs for an execution",
	Long:  `Get logs for an execution`,
	Run:   logsRun,
	PostRun: func(_ *cobra.Command, _ []string) {
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
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	output.Infof("Getting logs for execution: %s", output.Bold(executionID))

	c := client.New(cfg, slog.Default())
	resp, err := c.GetLogs(cmd.Context(), executionID)
	if err != nil {
		output.Errorf("failed to get logs: %v", err)
		return
	}

	output.Blank()
	rows := [][]string{}
	for _, log := range resp.Events {
		rows = append(rows, []string{
			output.Bold(fmt.Sprintf("%d", log.Line)),
			time.Unix(log.Timestamp/constants.MillisecondsPerSecond, 0).UTC().Format(time.DateTime),
			log.Message,
		})
	}
	output.Table([]string{"Line", "Timestamp (UTC)", "Message"}, rows)
	output.Blank()
	output.Successf("Logs retrieved successfully")
	output.Infof("View logs in web viewer: %s?execution_id=%s",
		constants.WebviewerURL, output.Cyan(executionID))
}
