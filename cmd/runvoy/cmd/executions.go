package cmd

import (
    "fmt"
    "log/slog"
    "runvoy/internal/output"
    "runvoy/internal/client"
    "time"

    "github.com/spf13/cobra"
)

var executionsCmd = &cobra.Command{
    Use:   "list",
    Short: "List executions",
    Long:  "List all executions present in the runvoy backend",
    Run:   executionsRun,
    PostRun: func(cmd *cobra.Command, _ []string) {
        output.Blank()
    },
}

func init() {
    rootCmd.AddCommand(executionsCmd)
}

func executionsRun(cmd *cobra.Command, _ []string) {
    cfg, err := getConfigFromContext(cmd)
    if err != nil {
        output.Error("failed to load configuration: %v", err)
        return
    }

    output.Info("Listing executions…")

    client := client.New(cfg, slog.Default())
    execs, err := client.ListExecutions(cmd.Context())
    if err != nil {
        output.Error("failed to list executions: %v", err)
        return
    }

    rows := make([][]string, 0, len(execs))
    for _, e := range execs {
        started := e.StartedAt.UTC().Format(time.DateTime)
        completed := ""
        if e.CompletedAt != nil {
            completed = e.CompletedAt.UTC().Format(time.DateTime)
        }
        duration := ""
        if e.DurationSeconds > 0 {
            duration = fmt.Sprintf("%ds", e.DurationSeconds)
        }

        rows = append(rows, []string{
            output.Bold(e.ExecutionID),
            e.Status,
            e.Command,
            started,
            completed,
            duration,
            e.ComputePlatform,
        })
    }

    output.Blank()
    output.Table(
        []string{"Execution ID", "Status", "Command", "Started (UTC)", "Completed (UTC)", "Duration", "Cloud"},
        rows,
    )
    output.Blank()
    output.Success("Executions listed successfully")
}
