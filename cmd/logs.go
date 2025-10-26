package cmd

import (
	"context"
	"fmt"
	"time"

	"mycli/internal/api"
	internalConfig "mycli/internal/config"

	"github.com/spf13/cobra"
)

var (
	follow bool
)

var logsCmd = &cobra.Command{
	Use:   "logs <execution-id>",
	Short: "View logs from an execution",
	Long: `View CloudWatch logs from a running or completed execution.
Provide the execution ID returned by the exec command.`,
	Args: cobra.ExactArgs(1),
	RunE: runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (poll for new logs)")
}

func runLogs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	executionID := args[0]

	// Load config
	cfg, err := internalConfig.Load()
	if err != nil {
		return fmt.Errorf("not configured. Run 'mycli init' first: %w", err)
	}

	apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)

	if follow {
		// Poll for logs every 2 seconds
		fmt.Printf("Following logs for execution: %s\n", executionID)
		fmt.Println("(Press Ctrl+C to stop)")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		lastLogs := ""
		for {
			logs, err := apiClient.GetLogs(ctx, executionID)
			if err != nil {
				return fmt.Errorf("failed to get logs: %w", err)
			}

			// Only print new logs
			if logs.Logs != lastLogs {
				if lastLogs == "" {
					// First time, print all logs
					fmt.Print(logs.Logs)
				} else {
					// Print only the new part
					newPart := logs.Logs[len(lastLogs):]
					fmt.Print(newPart)
				}
				lastLogs = logs.Logs
			}

			time.Sleep(2 * time.Second)
		}
	} else {
		// Get logs once
		logs, err := apiClient.GetLogs(ctx, executionID)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}

		if logs.Logs == "" {
			fmt.Printf("No logs available yet for execution: %s\n", executionID)
			fmt.Println("(Logs may take a few seconds to appear)")
			fmt.Printf("\nTry: mycli logs -f %s\n", executionID)
		} else {
			fmt.Printf("Logs for execution: %s\n", executionID)
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Print(logs.Logs)
		}
	}

	return nil
}
