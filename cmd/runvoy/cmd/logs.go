package cmd

import (
    "context"
    "log/slog"
    "strings"
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

    cl := client.New(cfg, slog.Default())

    // 1) Wait until execution is running (or completed) with a spinner
    spinner := output.NewSpinner("Waiting for execution to start...")
    spinner.Start()

    // Consider these as non-started states
    isWaitingStatus := func(s string) bool {
        st := strings.ToLower(strings.TrimSpace(s))
        return st == "pending" || st == "queued" || st == "starting" || st == "scheduled"
    }

    // Poll status until not waiting anymore or context canceled
    for {
        statusResp, err := cl.GetExecutionStatus(cmd.Context(), executionID)
        if err == nil {
            if !isWaitingStatus(statusResp.Status) {
                spinner.Success("Execution started: " + output.StatusBadge(statusResp.Status))
                break
            }
        }

        // Respect context cancellation
        select {
        case <-cmd.Context().Done():
            spinner.Error("Canceled while waiting for execution to start")
            return
        case <-time.After(2 * time.Second):
        }
    }

    // 2) Tail logs: fetch every 5s and print only new lines
    output.Blank()
    output.Info("Streaming logs (press Ctrl+C to stop)...")

    // Track seen events using "timestamp|message" keys to avoid duplicates
    seen := make(map[string]struct{})
    var lastPrinted time.Time

    printNew := func(ctx context.Context) (int, error) {
        resp, err := cl.GetLogs(ctx, executionID)
        if err != nil {
            return 0, err
        }
        printed := 0
        for _, ev := range resp.Events {
            ts := time.Unix(ev.Timestamp/1000, 0).UTC()
            key := ts.Format(time.RFC3339Nano) + "|" + ev.Message
            if _, ok := seen[key]; ok {
                continue
            }
            seen[key] = struct{}{}
            lastPrinted = ts
            output.Printf("%s  %s\n", ts.Format(time.DateTime), ev.Message)
            printed++
        }
        return printed, nil
    }

    // Initial dump (in case the task already started and has logs)
    _, _ = printNew(cmd.Context())

    // Loop until status is terminal and no new logs arrive
    isTerminal := func(s string) bool {
        st := strings.ToLower(strings.TrimSpace(s))
        return st == "completed" || st == "success" || st == "succeeded" || st == "failed" || st == "error" || st == "cancelled" || st == "canceled"
    }

    for {
        // Sleep between polls
        select {
        case <-cmd.Context().Done():
            output.Blank()
            output.Warning("Log tail canceled")
            return
        case <-time.After(5 * time.Second):
        }

        // Fetch and print any new logs
        _, err := printNew(cmd.Context())
        if err != nil {
            output.Warning("failed to fetch logs: %v", err)
        }

        // Check status to decide if we should stop
        statusResp, err := cl.GetExecutionStatus(cmd.Context(), executionID)
        if err != nil {
            // Non-fatal; continue trying until context canceled
            continue
        }
        if isTerminal(statusResp.Status) {
            // One final fetch to flush any stragglers
            _, _ = printNew(cmd.Context())

            output.Blank()
            if strings.EqualFold(statusResp.Status, "completed") || strings.EqualFold(statusResp.Status, "success") || strings.EqualFold(statusResp.Status, "succeeded") {
                output.Success("Execution finished: %s", output.StatusBadge(statusResp.Status))
            } else {
                output.Error("Execution finished: %s", output.StatusBadge(statusResp.Status))
            }
            if !lastPrinted.IsZero() {
                output.KeyValue("Last Log", lastPrinted.Format(time.DateTime))
            }
            return
        }
    }
}
