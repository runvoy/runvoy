package cmd

import (
    "context"
    "log/slog"
    "strconv"
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
    // --follow flag to enable tailing
    logsCmd.Flags().BoolP("follow", "f", false, "Follow log output and continue streaming")
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

    // Decide follow vs single-shot behavior
    follow, _ := cmd.Flags().GetBool("follow")

    output.Blank()
    if follow {
        output.Info("Streaming logs (press Ctrl+C to stop)...")
    }

    // Track highest seen line number to avoid duplicates when following
    maxSeenLine := 0
    var lastPrinted time.Time

    printNew := func(ctx context.Context) (int, error) {
        resp, err := cl.GetLogs(ctx, executionID)
        if err != nil {
            return 0, err
        }
        printed := 0
        for _, ev := range resp.Events {
            if ev.Line <= maxSeenLine {
                continue
            }
            ts := time.Unix(ev.Timestamp/1000, 0).UTC()
            lastPrinted = ts
            output.Printf("[%d] %s  %s\n", ev.Line, ts.Format(time.DateTime), ev.Message)
            if ev.Line > maxSeenLine {
                maxSeenLine = ev.Line
            }
            printed++
        }
        return printed, nil
    }

    // Initial fetch
    firstPrinted, firstErr := printNew(cmd.Context())
    if firstErr != nil && !follow {
        // If logs not ready yet (e.g., 404), keep waiting a bit to provide a better UX
        // but only for the non-follow case so user gets the complete set then exits
        for i := 0; i < 5; i++ { // up to ~10s
            select {
            case <-cmd.Context().Done():
                output.Warning("Canceled while waiting for logs")
                return
            case <-time.After(2 * time.Second):
            }
            if _, err := printNew(cmd.Context()); err == nil {
                firstErr = nil
                break
            }
        }
    }
    if firstErr != nil {
        output.Warning("failed to fetch logs: %v", firstErr)
    }

    // If not following, print a table of everything and exit
    if !follow {
        // Re-fetch for a full dataset to render as a table
        resp, err := cl.GetLogs(cmd.Context(), executionID)
        if err != nil {
            output.Error("failed to get logs: %v", err)
            return
        }
        rows := make([][]string, 0, len(resp.Events))
        for _, ev := range resp.Events {
            ts := time.Unix(ev.Timestamp/1000, 0).UTC().Format(time.DateTime)
            rows = append(rows, []string{
                // Columns: Line, Timestamp, Message
                strconv.Itoa(ev.Line),
                ts,
                ev.Message,
            })
        }
        output.Table([]string{"Line", "Timestamp (UTC)", "Message"}, rows)
        output.Blank()
        output.Success("Logs retrieved successfully")
        return
    }

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
