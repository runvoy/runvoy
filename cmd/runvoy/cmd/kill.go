package cmd

import (
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:   "kill <execution-id>",
	Short: "Kill a running command execution",
	Long:  `Kill a running command execution`,
	Run:   killRun,
	Args:  cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(killCmd)
}

func killRun(cmd *cobra.Command, args []string) {
	executionID := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	resp, err := c.KillExecution(cmd.Context(), executionID)
	if err != nil {
		output.Errorf(err.Error())
		return
	}

	output.Successf("Execution killed successfully")
	output.KeyValue("Execution ID", resp.ExecutionID)
	output.KeyValue("Message", resp.Message)
}
