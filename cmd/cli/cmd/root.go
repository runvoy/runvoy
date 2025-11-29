package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/runvoy/runvoy/internal/client"
	"github.com/runvoy/runvoy/internal/client/output"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/logger"

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
	Short: constants.ProjectName,
	Long: fmt.Sprintf(`%s - %s
Isolated, repeatable execution environments for your commands`,
		constants.ProjectName, *constants.GetVersion()),
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		startTime := time.Now().UTC()
		cmd.SetContext(context.WithValue(cmd.Context(), constants.StartTimeCtxKey, startTime))
		printHeader(cmd)

		if verbose {
			output.Infof("CLI build: " + output.Bold(*constants.GetVersion()))
			output.Infof("Verbose output enabled")
		}

		logLevel := slog.LevelInfo
		if debug {
			logLevel = slog.LevelDebug
		}
		logger := logger.Initialize(constants.CLI, logLevel)

		if timeout == "0" {
			if verbose {
				output.Infof("Timeout disabled")
			}

			return nil
		}

		// NOTICE: this runs after flags are parsed but before the command runs
		timeoutDuration, err := parseTimeout(timeout)
		if err != nil {
			return fmt.Errorf("error parsing timeout: %w", err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), timeoutDuration)
		timeoutCancel = cancel // Store for cleanup in Execute()
		cmd.SetContext(ctx)

		if verbose {
			output.Infof("Timeout: %s", timeoutDuration)
		}

		cfg, err := config.LoadCLI()
		if err != nil {
			logger.Warn("failed to load configuration", "error", err)
			return nil
		}

		configPath, err := config.GetConfigPath()
		if err != nil {
			logger.Warn("failed to get config path", "error", err)
			return nil
		}

		cmd.SetContext(context.WithValue(cmd.Context(), constants.ConfigCtxKey, cfg))
		if verbose {
			output.Infof("Loaded configuration from %s", output.Bold(configPath))
			output.Infof("API endpoint: %s", output.Bold(cfg.APIEndpoint))
			if cfg.WebURL != "" {
				output.Infof("Web URL: %s", output.Bold(cfg.WebURL))
			}
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, _ []string) {
		if verbose {
			startTime := getStartTimeFromContext(cmd)
			if !startTime.IsZero() {
				output.Infof("Time elapsed: %s", output.Bold(time.Since(startTime).String()))
			}
		}
		if timeoutCancel != nil {
			timeoutCancel()
		}
	},
}

// Execute runs the root command and handles cleanup of timeout context.
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
		errMsg := fmt.Sprintf(
			"invalid timeout format: %s (use duration like '10m' or '30s', or seconds like '600')",
			timeoutStr)
		return 0, errors.New(errMsg)
	}

	return time.Duration(seconds) * time.Second, nil
}

func printHeader(cmd *cobra.Command) {
	output.Header(output.Bold("🚀 " + constants.ProjectName + " " + cmd.CalledAs()))
}

// getConfigFromContext retrieves the config from the command context
func getConfigFromContext(cmd *cobra.Command) (*config.Config, error) {
	cfg, ok := cmd.Context().Value(constants.ConfigCtxKey).(*config.Config)
	if !ok || cfg == nil {
		return nil, fmt.Errorf("config not found in context")
	}
	return cfg, nil
}

// executeWithClient consolidates the common pattern of loading config, creating a client,
// and executing a function with the client. It handles error reporting consistently.
func executeWithClient(cmd *cobra.Command, fn func(ctx context.Context, c client.Interface) error) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	if err = fn(cmd.Context(), c); err != nil {
		output.Errorf(err.Error())
	}
}

func getStartTimeFromContext(cmd *cobra.Command) time.Time {
	startTime, ok := cmd.Context().Value(constants.StartTimeCtxKey).(time.Time)
	if !ok {
		return time.Time{}
	}
	return startTime
}

// RootCmd returns the root command for use by tools like doc generators.
func RootCmd() *cobra.Command {
	return rootCmd
}
