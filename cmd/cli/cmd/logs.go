package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/client"
	"github.com/runvoy/runvoy/internal/client/infra"
	"github.com/runvoy/runvoy/internal/client/output"
	"github.com/runvoy/runvoy/internal/constants"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <execution-id>",
	Short: "Get logs for an execution",
	Long:  `Get logs for an execution`,
	Run:   logsRun,
	Args:  cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

// isTerminalStatus reports whether the provided execution status is terminal.
func isTerminalStatus(status string) bool {
	return slices.Contains(constants.TerminalExecutionStatuses(), constants.ExecutionStatus(status))
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
	if err = service.DisplayLogs(cmd.Context(), executionID, cfg.WebURL); err != nil {
		output.Errorf(err.Error())
	}
}

// LogsService handles log display logic.
type LogsService struct {
	client client.Interface
	output OutputInterface
	stream func(websocketURL string, webURL, executionID string) error
}

// NewLogsService creates a new LogsService with the provided dependencies.
func NewLogsService(apiClient client.Interface, outputter OutputInterface) *LogsService {
	service := &LogsService{
		client: apiClient,
		output: outputter,
	}
	service.stream = func(websocketURL string, webURL, executionID string) error {
		return service.streamLogsViaWebSocket(websocketURL, webURL, executionID)
	}
	return service
}

// readWebSocketMessages reads messages from WebSocket and sends log events to a channel.
func (s *LogsService) readWebSocketMessages(
	conn *websocket.Conn,
	logChan chan<- api.LogEvent,
	done chan struct{},
	closeOnce *sync.Once,
) {
	defer close(logChan)
	defer closeOnce.Do(func() { close(done) })
	for {
		select {
		case <-done:
			return
		default:
			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					s.output.Warningf("WebSocket connection closed: %v", err)
				}
				return
			}

			// Check for disconnect message
			var msg struct {
				Type string `json:"type,omitempty"`
			}
			if err = json.Unmarshal(messageBytes, &msg); err == nil && msg.Type == string(api.WebSocketMessageTypeDisconnect) {
				s.output.Infof("Execution completed. Closing connection...")
				_ = conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Execution completed"),
				)
				return
			}

			var logEvent api.LogEvent
			if err = json.Unmarshal(messageBytes, &logEvent); err != nil {
				continue
			}

			select {
			case logChan <- logEvent:
			case <-done:
				return
			}
		}
	}
}

// streamLogsViaWebSocket connects to WebSocket and streams logs in real-time.
// The backend handles incremental log delivery, so we just append and count from 1.
func (s *LogsService) streamLogsViaWebSocket(
	websocketURL string,
	webURL string,
	executionID string,
) error {
	s.printWebviewerURL(webURL, executionID)

	s.output.Infof("Connecting to log stream...")
	conn, httpResp, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		s.output.Warningf("Failed to connect to WebSocket: %v", err)
		return fmt.Errorf("failed to connect to websocket: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()
	if httpResp != nil && httpResp.Body != nil {
		defer func() {
			_ = httpResp.Body.Close()
		}()
	}

	s.output.Successf("Connected to log stream. Press Ctrl+C to exit.")
	s.output.Blank()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	bufferSize := 10
	done := make(chan struct{})
	logChan := make(chan api.LogEvent, bufferSize) // buffered channel for better throughput
	var closeOnce sync.Once

	// Goroutine 1: Read from websocket and send to channel
	go s.readWebSocketMessages(conn, logChan, done, &closeOnce)

	// Goroutine 2: Read from channel and print logs
	// Backend sends incremental logs, so we just count from 1
	go func() {
		lineNumber := 0
		for logEvent := range logChan {
			lineNumber++
			s.printLogLine(lineNumber, logEvent)
		}
	}()

	select {
	case <-sigChan:
		s.output.Infof("Received interrupt signal, closing connection...")
		closeOnce.Do(func() { close(done) })
	case <-done:
		s.output.Infof("WebSocket connection closed")
	}

	return nil
}

// DisplayLogs retrieves static logs and then streams new logs via WebSocket in real-time
// If the execution has already completed, it displays static logs only and skips WebSocket streaming.
func (s *LogsService) DisplayLogs(ctx context.Context, executionID, webURL string) error {
	resp, err := s.client.GetLogs(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	if isTerminalStatus(resp.Status) {
		s.displayLogEvents(resp.Events)
		s.output.Infof("Execution has completed with status: %s", resp.Status)
		return nil
	}

	if resp.WebSocketURL == "" {
		return fmt.Errorf("execution is %s but no websocket URL was provided for streaming", resp.Status)
	}

	if s.stream == nil {
		return errors.New("websocket streaming function is not configured")
	}

	s.output.Infof("Execution status: %s. Streaming logs via WebSocket...", resp.Status)
	return s.stream(resp.WebSocketURL, webURL, executionID)
}

// displayLogEvents displays all log events in a sorted table.
func (s *LogsService) displayLogEvents(logEvents []api.LogEvent) {
	// Sort logs by timestamp (and preserve order for same timestamps)
	sortedEvents := make([]api.LogEvent, len(logEvents))
	copy(sortedEvents, logEvents)
	slices.SortFunc(sortedEvents, func(a, b api.LogEvent) int {
		if a.Timestamp < b.Timestamp {
			return -1
		}
		if a.Timestamp > b.Timestamp {
			return 1
		}
		return 0
	})

	s.output.Blank()
	rows := [][]string{}
	for i, log := range sortedEvents {
		lineNumber := i + 1 // Compute line number client-side (1-indexed)
		rows = append(rows, []string{
			s.output.Bold(strconv.Itoa(lineNumber)),
			time.Unix(log.Timestamp/constants.MillisecondsPerSecond, 0).UTC().Format(time.DateTime),
			log.Message,
		})
	}
	s.output.Table([]string{"Line", "Timestamp (UTC)", "Message"}, rows)
	s.output.Blank()
}

// printLogLine prints a single log line (used for streaming).
func (s *LogsService) printLogLine(lineNumber int, log api.LogEvent) {
	timestamp := time.Unix(log.Timestamp/constants.MillisecondsPerSecond, 0).UTC().Format(time.DateTime)
	fmt.Printf("%s %s %s\n",
		s.output.Bold(strconv.Itoa(lineNumber)),
		timestamp,
		log.Message,
	)
}

// printWebviewerURL prints the web application URL.
func (s *LogsService) printWebviewerURL(webURL, executionID string) {
	urlStr := infra.BuildLogsURL(webURL, executionID)
	s.output.Infof("View logs in web viewer: %s", s.output.Cyan(urlStr))
}
