package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"
	"time"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <execution-id>",
	Short: "Get logs for an execution",
	Long:  `Get logs for an execution`,
	Run:   logsRun,
	PostRun: func(_ *cobra.Command, _ []string) {
		output.Blank()
	},
	Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func logsRun(cmd *cobra.Command, args []string) {
	executionID := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	output.Infof("Getting logs for execution: %s", output.Bold(executionID))

	c := client.New(cfg, slog.Default())
	service := NewLogsService(c, NewOutputWrapper())
	if err := service.DisplayLogs(cmd.Context(), executionID, cfg.GetWebviewerURL()); err != nil {
		output.Errorf(err.Error())
	}
}

// LogsService handles log display logic
type LogsService struct {
	client client.Interface
	output OutputInterface
}

// NewLogsService creates a new LogsService with the provided dependencies
func NewLogsService(client client.Interface, output OutputInterface) *LogsService {
	return &LogsService{
		client: client,
		output: output,
	}
}

// DisplayLogs retrieves and displays logs for an execution
func (s *LogsService) DisplayLogs(ctx context.Context, executionID string, webviewerURL string) error {
	resp, err := s.client.GetLogs(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	s.output.Blank()
	rows := [][]string{}
	for _, log := range resp.Events {
		rows = append(rows, []string{
			s.output.Bold(fmt.Sprintf("%d", log.Line)),
			time.Unix(log.Timestamp/constants.MillisecondsPerSecond, 0).UTC().Format(time.DateTime),
			log.Message,
		})
	}
	s.output.Table([]string{"Line", "Timestamp (UTC)", "Message"}, rows)
	s.output.Blank()
	s.output.Successf("Logs retrieved successfully")
	s.output.Infof("View logs in web viewer: %s?execution_id=%s",
		webviewerURL, s.output.Cyan(executionID))
	return nil
}
