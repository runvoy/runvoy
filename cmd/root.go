package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var timeout string
var timeoutCancel context.CancelFunc

var rootCmd = &cobra.Command{
	Use:   "runvoy",
	Short: "Remote execution environment for your commands",
	Long: `runvoy provides isolated, repeatable execution environments for your commands.
Run commands remotely without the hassle of local execution, credential sharing, or race conditions.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if timeout == "0" {
			fmt.Println("→ Timeout is 0, disabling timeout")
			return nil
		}

		// Parse timeout value and create context
		// This runs after flags are parsed but before the command runs
		timeoutDuration, err := parseTimeout(timeout)
		if err != nil {
			return fmt.Errorf("error parsing timeout: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
		timeoutCancel = cancel // Store for cleanup in Execute()
		cmd.SetContext(ctx)

		fmt.Println("→ Timeout duration:", timeoutDuration)
		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()

	if timeoutCancel != nil {
		timeoutCancel()
	}

	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here if needed
	rootCmd.PersistentFlags().StringVar(&timeout, "timeout", "10m", "Timeout for command execution (e.g., 10m, 30s, 1h)")
}

// parseTimeout parses timeout string to time.Duration
// Supports formats: "10m", "30s", "1h", "600s" (number of seconds)
func parseTimeout(timeoutStr string) (time.Duration, error) {
	// Default to 10 minutes if empty
	if timeoutStr == "" {
		timeoutStr = "10m"
	}

	// Try parsing as duration first (supports "10m", "30s", "1h", etc.)
	duration, err := time.ParseDuration(timeoutStr)
	if err == nil {
		return duration, nil
	}

	// If duration parsing fails, try parsing as seconds (integer)
	seconds, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout format: %s (use duration like '10m' or '30s', or seconds like '600')", timeoutStr)
	}

	return time.Duration(seconds) * time.Second, nil
}
