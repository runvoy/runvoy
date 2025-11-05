package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/output"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <execution-id>",
	Short: "Get the status of a command execution",
	Run:   statusRun, Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func statusRun(cmd *cobra.Command, args []string) {
	executionID := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	service := NewStatusService(c, NewOutputWrapper())
	if err = service.DisplayStatus(cmd.Context(), executionID); err != nil {
		output.Errorf(err.Error())
	}
}

// StatusService handles status display logic
type StatusService struct {
	client client.Interface
	output OutputInterface
}

// NewStatusService creates a new StatusService with the provided dependencies
func NewStatusService(apiClient client.Interface, outputter OutputInterface) *StatusService {
	return &StatusService{
		client: apiClient,
		output: outputter,
	}
}

// DisplayStatus retrieves and displays the status of an execution
func (s *StatusService) DisplayStatus(ctx context.Context, executionID string) error {
	status, err := s.client.GetExecutionStatus(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	s.output.KeyValue("Execution ID", status.ExecutionID)
	s.output.KeyValue("Status", status.Status)
	s.output.KeyValue("Started At", status.StartedAt.Format(time.DateTime))
	if status.CompletedAt != nil {
		s.output.KeyValue("Completed At", status.CompletedAt.Format(time.DateTime))
	}
	if status.ExitCode != nil {
		s.output.KeyValue("Exit Code", fmt.Sprintf("%d", *status.ExitCode))
	}
	s.output.Blank()
	s.output.Successf("Status retrieved successfully")
	return nil
}
