package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/client/output"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var traceCmd = &cobra.Command{
	Use:     "trace <request-id>",
	Short:   "Get backend logs for a given request ID",
	Example: "  runvoy trace c2584f31-f753-4a07-9556-ed79dc36a10b",
	Run:     traceRun,
	Args:    cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(traceCmd)
}

func traceRun(cmd *cobra.Command, args []string) {
	requestID := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	service := NewTraceService(c, NewOutputWrapper())
	if err = service.DisplayBackendLogs(cmd.Context(), requestID); err != nil {
		output.Errorf(err.Error())
	}
}

// TraceService handles backend logs display logic
type TraceService struct {
	client client.Interface
	output OutputInterface
}

// NewTraceService creates a new TraceService with the provided dependencies
func NewTraceService(apiClient client.Interface, outputter OutputInterface) *TraceService {
	return &TraceService{
		client: apiClient,
		output: outputter,
	}
}

// DisplayBackendLogs retrieves and displays backend infrastructure logs for a request ID
func (s *TraceService) DisplayBackendLogs(ctx context.Context, requestID string) error {
	spinner := output.NewSpinner(fmt.Sprintf("Fetching backend logs for request: %s", requestID))
	spinner.Start()
	defer spinner.Stop()

	logs, err := s.client.FetchBackendLogs(ctx, requestID)
	if err != nil {
		spinner.Error("Failed to fetch backend logs")
		return fmt.Errorf("failed to fetch backend logs: %w", err)
	}

	if len(logs) == 0 {
		spinner.Success("No logs found for request")
		return nil
	}

	spinner.Success(fmt.Sprintf("Retrieved %d log entries", len(logs)))

	// Display logs in table format
	headers := []string{"Timestamp", "Message"}
	rows := make([][]string, 0, len(logs))

	for _, log := range logs {
		// Convert milliseconds since epoch to readable timestamp
		timestamp := time.UnixMilli(log.Timestamp).UTC().Format(time.RFC3339Nano)
		message := strings.TrimRight(log.Message, "\r\n")
		rows = append(rows, []string{timestamp, message})
	}

	s.output.Table(headers, rows)
	s.output.Blank()
	s.output.Successf("Backend logs retrieved successfully")

	return nil
}
