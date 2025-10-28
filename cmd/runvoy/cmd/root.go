package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"

	"github.com/spf13/cobra"
)

var timeout string
var timeoutCancel context.CancelFunc
var verbose bool

var rootCmd = &cobra.Command{
	Use:   constants.ProjectName,
	Short: "Remote execution environment for your commands",
	Long: fmt.Sprintf(`%s provides isolated, repeatable execution environments for your commands.
Run commands remotely without the hassle of local execution, credential sharing, or race conditions.`, constants.ProjectName),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize logger based on verbose flag
		logLevel := slog.LevelInfo
		if verbose {
			logLevel = slog.LevelDebug
		}
		logger.Initialize(constants.Development, logLevel)

		if timeout == "0" {
			slog.Debug("Timeout disabled")
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

		slog.Debug("Timeout configured", "duration", timeoutDuration)

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
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Verbose output")
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

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the version of the CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s CLI version: %s\n", constants.ProjectName, constants.Version)

		cfg, err := config.Load()
		if err != nil {
			slog.Error("failed to load configuration", "error", err)
			return
		}

		var resp api.HealthResponse
		req := client.Request{
			Method: "GET",
			Path:   "health",
		}

		client := client.New(cfg, slog.Default())
		if err := client.DoJSON(req, &resp); err != nil {
			fmt.Printf("❌ %s\n", err)
			return
		}

		fmt.Printf("Backend version: %s\n", resp.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
