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
	Use:   "logs <task-arn>",
	Short: "View logs from an execution",
	Long: `View CloudWatch logs from a running or completed execution.
Provide the task ARN returned by the exec command.

Example:
  mycli logs arn:aws:ecs:us-east-1:123456789:task/mycli-cluster/abc123def456`,
	Args: cobra.ExactArgs(1),
	RunE: runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (poll for new logs)")
}

func runLogs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	taskArn := args[0]

	// Load config
	cfg, err := internalConfig.Load()
	if err != nil {
		return fmt.Errorf("not configured. Run 'mycli init' first: %w", err)
	}

	apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)

	if follow {
		// Poll for logs every 2 seconds
		fmt.Printf("Following logs for task: %s\n", taskArn)
		fmt.Println("(Press Ctrl+C to stop)")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		lastLogs := ""
		for {
			logs, err := apiClient.GetLogsByTaskArn(ctx, taskArn)
			if err != nil {
				return fmt.Errorf("failed to get logs: %w", err)
			}

			// Only print new logs
			if logs.Logs != lastLogs {
				if lastLogs == "" {
					// First time, print all logs
					fmt.Print(logs.Logs)
					lastLogs = logs.Logs
				} else if len(logs.Logs) > len(lastLogs) && logs.Logs[:len(lastLogs)] == lastLogs {
					// Logs have grown - print only the new part
					newPart := logs.Logs[len(lastLogs):]
					fmt.Print(newPart)
					lastLogs = logs.Logs
				} else {
					// Logs changed completely (e.g., from "No logs available" to actual logs)
					// Clear and reprint everything
					fmt.Print("\r\033[K") // Clear line
					fmt.Print(logs.Logs)
					lastLogs = logs.Logs
				}
			}

			time.Sleep(2 * time.Second)
		}
	} else {
		// Get logs once
		logs, err := apiClient.GetLogsByTaskArn(ctx, taskArn)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}

		if logs.Logs == "" {
			fmt.Printf("No logs available yet for task: %s\n", taskArn)
			fmt.Println("(Logs may take a few seconds to appear)")
			fmt.Printf("\nTry: mycli logs -f %s\n", taskArn)
		} else {
			fmt.Printf("Logs for task: %s\n", taskArn)
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Print(logs.Logs)
		}
	}

	return nil
}
