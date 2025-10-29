package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"runvoy/internal/constants"
	"runvoy/internal/logger"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var (
	debug         bool
	timeout       string
	timeoutCancel context.CancelFunc
	verbose       bool
)

var rootCmd = &cobra.Command{
	Use:   constants.ProjectName,
	Short: "Remote execution environment for your commands",
	Long: fmt.Sprintf(`%s provides isolated, repeatable execution environments for your commands.
Run commands remotely without the hassle of local execution, credential sharing, or race conditions.`, constants.ProjectName),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			output.Header(output.Bold(constants.ProjectName) + " " + *constants.GetVersion())
			output.Info("verbose output enabled")
		}

		logLevel := slog.LevelInfo
		if debug {
			logLevel = slog.LevelDebug
		}
		logger.Initialize(constants.CLI, logLevel)

		if timeout == "0" {
			if verbose {
				output.Info("timeout disabled")
			}

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

		if verbose {
			output.Info("timeout: %s", timeoutDuration)
		}
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
	rootCmd.PersistentFlags().StringVar(&timeout, "timeout", "10m", "Timeout for command execution (e.g., 10m, 30s, 1h)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debugging logs")
}

// parseTimeout parses timeout string to time.Duration
// defaults to 10 minutes if empty
// Supports formats: "10m", "30s", "1h", "600s" (number of seconds)
func parseTimeout(timeoutStr string) (time.Duration, error) {
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
