package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
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
	if err = service.DisplayLogs(cmd.Context(), executionID, cfg.GetWebviewerURL()); err != nil {
		output.Errorf(err.Error())
	}
}

// LogsService handles log display logic
type LogsService struct {
	client client.Interface
	output OutputInterface
}

// NewLogsService creates a new LogsService with the provided dependencies
func NewLogsService(apiClient client.Interface, outputter OutputInterface) *LogsService {
	return &LogsService{
		client: apiClient,
		output: outputter,
	}
}

// DisplayLogs retrieves static logs and then streams new logs via WebSocket in real-time
func (s *LogsService) DisplayLogs( //nolint:funlen
	ctx context.Context, executionID, webviewerURL string) error {
	// Fetch static logs first
	resp, err := s.client.GetLogs(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	logMap := make(map[int64]api.LogEvent)
	var mu sync.Mutex

	for _, log := range resp.Events {
		logMap[log.Timestamp] = log
	}

	s.displayLogEvents(logMap)

	streamResp, err := s.client.GetLogStreamURL(ctx, executionID)
	if err != nil {
		s.output.Warningf("Failed to get WebSocket URL: %v", err)
		s.printWebviewerURL(webviewerURL, executionID)
		return nil
	}

	if streamResp.WebSocketURL == "" {
		s.output.Warningf("WebSocket streaming not configured on server")
		s.printWebviewerURL(webviewerURL, executionID)
		return nil
	}

	// Connect to WebSocket
	s.output.Infof("Connecting to log stream...")
	conn, _, err := websocket.DefaultDialer.Dial(streamResp.WebSocketURL, nil)
	if err != nil {
		s.output.Warningf("Failed to connect to WebSocket: %v", err)
		s.printWebviewerURL(webviewerURL, executionID)
		return nil
	}
	defer func() {
		_ = conn.Close()
	}()

	s.output.Successf("Connected to log stream. Press Ctrl+C to exit.")
	s.output.Blank()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	done := make(chan struct{})

	var closeOnce sync.Once

	go func() {
		defer closeOnce.Do(func() { close(done) })

		for {
			select {
			case <-done:
				return
			default:
				var logEvent api.LogEvent
				err = conn.ReadJSON(&logEvent)
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
						s.output.Warningf("WebSocket connection closed: %v", err)
					}
					return
				}

				mu.Lock()
				if _, exists := logMap[logEvent.Timestamp]; !exists {
					logMap[logEvent.Timestamp] = logEvent
					s.printLogLine(len(logMap), logEvent)
				}
				mu.Unlock()
			}
		}
	}()

	select {
	case <-sigChan:
		s.output.Blank()
		s.output.Infof("Received interrupt signal, closing connection...")
		closeOnce.Do(func() { close(done) })
	case <-done:
		s.output.Blank()
		s.output.Infof("WebSocket connection closed")
	}

	s.printWebviewerURL(webviewerURL, executionID)
	return nil
}

// displayLogEvents displays all log events in a sorted table
func (s *LogsService) displayLogEvents(logMap map[int64]api.LogEvent) {
	// Sort logs by timestamp
	var timestamps []int64
	for ts := range logMap {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})

	s.output.Blank()
	rows := [][]string{}
	for i, ts := range timestamps {
		log := logMap[ts]
		lineNumber := i + 1 // Compute line number client-side (1-indexed)
		rows = append(rows, []string{
			s.output.Bold(fmt.Sprintf("%d", lineNumber)),
			time.Unix(log.Timestamp/constants.MillisecondsPerSecond, 0).UTC().Format(time.DateTime),
			log.Message,
		})
	}
	s.output.Table([]string{"Line", "Timestamp (UTC)", "Message"}, rows)
	s.output.Blank()
}

// printLogLine prints a single log line (used for streaming)
func (s *LogsService) printLogLine(lineNumber int, log api.LogEvent) {
	timestamp := time.Unix(log.Timestamp/constants.MillisecondsPerSecond, 0).UTC().Format(time.DateTime)
	fmt.Printf("%s %s %s\n",
		s.output.Bold(fmt.Sprintf("%d", lineNumber)),
		timestamp,
		log.Message,
	)
}

// printWebviewerURL prints the webviewer URL
func (s *LogsService) printWebviewerURL(webviewerURL, executionID string) {
	s.output.Blank()
	s.output.Infof("View logs in web viewer: %s?execution_id=%s",
		webviewerURL, s.output.Cyan(executionID))
}
