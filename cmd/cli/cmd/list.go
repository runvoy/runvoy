package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/client"
	"github.com/runvoy/runvoy/internal/client/output"
	"github.com/runvoy/runvoy/internal/constants"

	"github.com/spf13/cobra"
)

const maxCommandLength = 40

var executionsCmd = &cobra.Command{
	Use:   "list",
	Short: "List command executions",
	Long: fmt.Sprintf(
		`List command executions present in the runvoy backend with optional filtering.
Show last %d executions and all statuses by default. Use --limit and --status flags to customize the output.`,
		constants.DefaultExecutionListLimit,
	),
	Example: fmt.Sprintf(`  # Show last %d executions
  - %s list

  # Show last 100 executions
  - %s list --limit 100

  # Show last 20 executions and filter by RUNNING and SUCCEEDED statuses
  - %s list --limit 20 --status RUNNING,SUCCEEDED`,
		constants.DefaultExecutionListLimit,
		constants.ProjectName, constants.ProjectName, constants.ProjectName),
	Run: executionsRun,
}

var (
	limitFlag  int
	statusFlag string
)

func init() {
	rootCmd.AddCommand(executionsCmd)

	executionsCmd.Flags().IntVar(
		&limitFlag,
		"limit",
		constants.DefaultExecutionListLimit,
		fmt.Sprintf("maximum number of executions to return (default: %d, use 0 for all)",
			constants.DefaultExecutionListLimit),
	)
	executionsCmd.Flags().StringVar(&statusFlag, "status", "",
		"comma-separated list of execution statuses to filter by (e.g., RUNNING,TERMINATING)")
}

func executionsRun(cmd *cobra.Command, _ []string) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	service := NewListService(c, NewOutputWrapper())
	// Convert status flag to uppercase to allow case-insensitive input
	upperStatus := strings.ToUpper(statusFlag)
	if err = service.ListExecutions(cmd.Context(), limitFlag, upperStatus); err != nil {
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

// ListExecutions lists executions with optional filtering and displays them in a table format
func (s *ListService) ListExecutions(ctx context.Context, limit int, statuses string) error {
	if limit < 0 {
		return fmt.Errorf("limit must be zero or a positive integer, got %d", limit)
	}

	s.output.Infof("Listing executions…")

	execs, err := s.client.ListExecutions(ctx, limit, statuses)
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
			e.CreatedBy,
			started,
			completed,
			duration,
		})
	}
	return rows
}
