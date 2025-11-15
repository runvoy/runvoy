package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/client/output"
	"time"

	"github.com/spf13/cobra"
)

const maxCommandLength = 40

var executionsCmd = &cobra.Command{
	Use:   "list",
	Short: "List command executions",
	Long:  "List all command executions present in the runvoy backend",
	Run:   executionsRun,
}

func init() {
	rootCmd.AddCommand(executionsCmd)
}

func executionsRun(cmd *cobra.Command, _ []string) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	service := NewListService(c, NewOutputWrapper())
	if err = service.ListExecutions(cmd.Context()); err != nil {
		output.Errorf(err.Error())
	}
}

// ListService handles execution listing and formatting logic
type ListService struct {
	client client.Interface
	output OutputInterface
}

// NewListService creates a new ListService with the provided dependencies
func NewListService(apiClient client.Interface, outputter OutputInterface) *ListService {
	return &ListService{
		client: apiClient,
		output: outputter,
	}
}

// ListExecutions lists all executions and displays them in a table format
func (s *ListService) ListExecutions(ctx context.Context) error {
	s.output.Infof("Listing executions…")

	execs, err := s.client.ListExecutions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list executions: %w", err)
	}

	rows := s.formatExecutions(execs)

	s.output.Blank()
	s.output.Table(
		[]string{
			"Execution ID",
			"Status",
			"Command",
			"User",
			"Started (UTC)",
			"Completed (UTC)",
			"Duration",
		},
		rows,
	)
	s.output.Blank()
	s.output.Successf("Executions listed successfully")
	return nil
}

// formatExecutions formats execution data into table rows
func (s *ListService) formatExecutions(execs []api.Execution) [][]string {
	rows := make([][]string, 0, len(execs))
	for i := range execs {
		e := &execs[i]
		started := e.StartedAt.UTC().Format(time.DateTime)
		completed := ""
		if e.CompletedAt != nil {
			completed = e.CompletedAt.UTC().Format(time.DateTime)
		}
		duration := ""
		if e.DurationSeconds > 0 {
			duration = fmt.Sprintf("%ds", e.DurationSeconds)
		}

		command := ""
		if len(e.Command) > maxCommandLength {
			command = e.Command[:maxCommandLength] + "..."
		} else {
			command = e.Command
		}

		rows = append(rows, []string{
			s.output.Bold(e.ExecutionID),
			e.Status,
			command,
			e.UserEmail,
			started,
			completed,
			duration,
		})
	}
	return rows
}
