package cmd

import (
	"fmt"
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/output"
	"time"

	"github.com/spf13/cobra"
)

const maxCommandLength = 40

var executionsCmd = &cobra.Command{
	Use:   "list",
	Short: "List executions",
	Long:  "List all executions present in the runvoy backend",
	Run:   executionsRun,
	PostRun: func(_ *cobra.Command, _ []string) {
		output.Blank()
	},
}

func init() {
	rootCmd.AddCommand(executionsCmd)
}

func executionsRun(cmd *cobra.Command, _ []string) { //nolint:funlen
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	output.Infof("Listing executions…")

	c := client.New(cfg, slog.Default())
	execs, err := c.ListExecutions(cmd.Context())
	if err != nil {
		output.Errorf("failed to list executions: %v", err)
		return
	}

	rows := make([][]string, 0, len(execs))
	for i := range execs {
		e := &execs[i]
		started := e.StartedAt.UTC().Format(time.DateTime)
		completed := ""
		if e.CompletedAt != nil {
			completed = e.CompletedAt.UTC().Format(time.DateTime)
		}
		duration := ""
		if e.DurationSeconds > 0 {
			duration = fmt.Sprintf("%ds", e.DurationSeconds)
		}

		command := ""
		if len(e.Command) > maxCommandLength {
			command = e.Command[:maxCommandLength] + "..."
		} else {
			command = e.Command
		}

		rows = append(rows, []string{
			output.Bold(e.ExecutionID),
			e.Status,
			command,
			e.UserEmail,
			started,
			completed,
			duration,
		})
	}

	output.Blank()
	output.Table(
		[]string{
			"Execution ID",
			"Status",
			"Command",
			"User",
			"Started (UTC)",
			"Completed (UTC)",
			"Duration",
		},
		rows,
	)
	output.Blank()
	output.Successf("Executions listed successfully")
}
