package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/client"
	"github.com/runvoy/runvoy/internal/client/output"

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
	service := NewKillService(c, NewOutputWrapper())
	if err = service.KillExecution(cmd.Context(), executionID); err != nil {
		output.Errorf(err.Error())
	}
}

// KillService handles execution killing logic.
type KillService struct {
	client client.Interface
	output OutputInterface
}

// NewKillService creates a new KillService with the provided dependencies.
func NewKillService(apiClient client.Interface, outputter OutputInterface) *KillService {
	return &KillService{
		client: apiClient,
		output: outputter,
	}
}

// KillExecution kills a running execution and displays the results.
func (s *KillService) KillExecution(ctx context.Context, executionID string) error {
	resp, err := s.client.KillExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to kill execution: %w", err)
	}

	if resp == nil {
		s.output.Successf("Execution is already terminated, no action taken")
		s.output.KeyValue("Execution ID", executionID)
		return nil
	}

	s.output.Successf("Execution kill started successfully")
	s.output.KeyValue("Execution ID", resp.ExecutionID)
	s.output.KeyValue("Message", resp.Message)
	return nil
}
