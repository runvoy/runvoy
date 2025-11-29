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

	"github.com/spf13/cobra"
)

var traceCmd = &cobra.Command{
	Use:     "trace <request-id>",
	Short:   "Get backend logs and related resources for a given request ID",
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

// DisplayBackendLogs retrieves and displays backend infrastructure logs and related resources for a request ID
func (s *TraceService) DisplayBackendLogs(ctx context.Context, requestID string) error {
	spinner := output.NewSpinner(fmt.Sprintf("Fetching trace for request: %s", requestID))
	spinner.Start()
	defer spinner.Stop()

	trace, err := s.client.FetchBackendLogs(ctx, requestID)
	if err != nil {
		spinner.Error("Failed to fetch trace")
		return fmt.Errorf("failed to fetch trace: %w", err)
	}

	if s.isTraceEmpty(trace) {
		spinner.Success("No logs or related resources found for request")
		return nil
	}

	spinner.Success(fmt.Sprintf("Retrieved %d log entries and related resources", len(trace.Logs)))

	s.displayLogs(trace.Logs)
	s.displayExecutions(trace.RelatedResources.Executions)
	s.displaySecrets(trace.RelatedResources.Secrets)
	s.displayUsers(trace.RelatedResources.Users)
	s.displayImages(trace.RelatedResources.Images)

	s.output.Blank()
	s.output.Successf("Trace retrieved successfully")

	return nil
}

func (s *TraceService) isTraceEmpty(trace *api.TraceResponse) bool {
	return len(trace.Logs) == 0 && len(trace.RelatedResources.Executions) == 0 &&
		len(trace.RelatedResources.Secrets) == 0 && len(trace.RelatedResources.Users) == 0 &&
		len(trace.RelatedResources.Images) == 0
}

func (s *TraceService) displayLogs(logs []api.LogEvent) {
	if len(logs) == 0 {
		return
	}

	s.output.Blank()
	s.output.Infof("Backend Logs (%d entries)", len(logs))
	s.output.Blank()

	headers := []string{"Timestamp", "Message"}
	rows := make([][]string, 0, len(logs))

	for _, log := range logs {
		timestamp := time.UnixMilli(log.Timestamp).UTC().Format(time.RFC3339Nano)
		message := strings.TrimRight(log.Message, "\r\n")
		rows = append(rows, []string{timestamp, message})
	}

	s.output.Table(headers, rows)
}

func (s *TraceService) displayExecutions(executions []*api.Execution) {
	if len(executions) == 0 {
		return
	}

	s.output.Blank()
	s.output.Infof("Related Executions (%d)", len(executions))
	s.output.Blank()

	headers := []string{"Execution ID", "Status", "Started At", "Created By"}
	rows := make([][]string, 0, len(executions))

	for _, exec := range executions {
		rows = append(rows, []string{
			exec.ExecutionID,
			exec.Status,
			exec.StartedAt.Format(time.RFC3339),
			exec.CreatedBy,
		})
	}

	s.output.Table(headers, rows)
}

func (s *TraceService) displaySecrets(secrets []*api.Secret) {
	if len(secrets) == 0 {
		return
	}

	s.output.Blank()
	s.output.Infof("Related Secrets (%d)", len(secrets))
	s.output.Blank()

	headers := []string{"Name", "Key Name", "Created By", "Updated By"}
	rows := make([][]string, 0, len(secrets))

	for _, secret := range secrets {
		rows = append(rows, []string{
			secret.Name,
			secret.KeyName,
			secret.CreatedBy,
			secret.UpdatedBy,
		})
	}

	s.output.Table(headers, rows)
}

func (s *TraceService) displayUsers(users []*api.User) {
	if len(users) == 0 {
		return
	}

	s.output.Blank()
	s.output.Infof("Related Users (%d)", len(users))
	s.output.Blank()

	headers := []string{"Email", "Role", "Created At", "Revoked"}
	rows := make([][]string, 0, len(users))

	for _, user := range users {
		revoked := "No"
		if user.Revoked {
			revoked = "Yes"
		}
		rows = append(rows, []string{
			user.Email,
			user.Role,
			user.CreatedAt.Format(time.RFC3339),
			revoked,
		})
	}

	s.output.Table(headers, rows)
}

func (s *TraceService) displayImages(images []api.ImageInfo) {
	if len(images) == 0 {
		return
	}

	s.output.Blank()
	s.output.Infof("Related Images (%d)", len(images))
	s.output.Blank()

	headers := []string{"Image ID", "Image", "Created By", "Created At"}
	rows := make([][]string, 0, len(images))

	for i := range images {
		img := images[i]
		rows = append(rows, []string{
			img.ImageID,
			img.Image,
			img.CreatedBy,
			img.CreatedAt.Format(time.RFC3339),
		})
	}

	s.output.Table(headers, rows)
}
