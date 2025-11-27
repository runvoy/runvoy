package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/client/infra"
	"runvoy/internal/client/output"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

// Sleeper provides a testable interface for introducing delays
type Sleeper interface {
	Sleep(duration time.Duration)
}

// RealSleeper implements Sleeper using the standard time.Sleep
type RealSleeper struct{}

// Sleep pauses execution for the specified duration
func (r *RealSleeper) Sleep(duration time.Duration) {
	time.Sleep(duration)
}

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

const (
	minRegexMatches = 2 // Minimum number of regex matches expected: full match + capture group
)

// isLogsNotReadyError checks if an error represents a logs-not-ready condition.
// This checks for 503 Service Unavailable (log stream doesn't exist yet).
// It handles both AppError types and client error strings formatted as [status] ...
func isLogsNotReadyError(err error) bool {
	statusCode := apperrors.GetStatusCode(err)
	// Check for 503 Service Unavailable (log stream not ready yet)
	if statusCode == http.StatusServiceUnavailable {
		return true
	}

	// Check if it's an AppError with SERVICE_UNAVAILABLE error code
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Code == apperrors.ErrCodeServiceUnavailable ||
			appErr.StatusCode == http.StatusServiceUnavailable
	}

	// Parse client error format: [status] error message
	// The client formats errors as: "[%d] %s: %s"
	errStr := err.Error()
	re := regexp.MustCompile(`\[(\d+)\]`)
	matches := re.FindStringSubmatch(errStr)
	if len(matches) >= minRegexMatches {
		statusStr := matches[1]
		if statusStr == fmt.Sprintf("%d", http.StatusServiceUnavailable) {
			return true
		}
	}

	return false
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
	service := NewLogsService(c, NewOutputWrapper(), &RealSleeper{})
	if err = service.DisplayLogs(cmd.Context(), executionID, cfg.WebURL); err != nil {
		output.Errorf(err.Error())
	}
}

// LogsService handles log display logic
type LogsService struct {
	client  client.Interface
	output  OutputInterface
	sleeper Sleeper
	stream  func(websocketURL string, startingLineNumber int, webURL, executionID string) error
}

// NewLogsService creates a new LogsService with the provided dependencies
func NewLogsService(apiClient client.Interface, outputter OutputInterface, sleeper Sleeper) *LogsService {
	service := &LogsService{
		client:  apiClient,
		output:  outputter,
		sleeper: sleeper,
	}
	service.stream = func(websocketURL string, startingLineNumber int, webURL, executionID string) error {
		return service.streamLogsViaWebSocket(websocketURL, startingLineNumber, webURL, executionID)
	}
	return service
}

// fetchLogsWithRetry fetches logs with retry logic for 503 errors
// (execution starting up, log stream not ready yet).
// It intelligently handles STARTING state by waiting ~20 seconds before the first poll,
// as provisioners like Fargate typically take that long to provision and start (and even longer
// for logs to be available).
func (s *LogsService) fetchLogsWithRetry(ctx context.Context, executionID string) (*api.LogsResponse, error) {
	const (
		maxRetries         = 4
		retryDelay         = 10 * time.Second
		startingStateDelay = 30 * time.Second
	)

	// Smart initial wait: Check execution status first
	// If STARTING or TERMINATING, wait before first log poll to avoid unnecessary 503s
	status, err := s.client.GetExecutionStatus(ctx, executionID)
	if err == nil {
		s.output.Infof("Execution status: %s", status.Status)
		if status.Status == string(constants.ExecutionStarting) {
			s.output.Infof(fmt.Sprintf(
				"Execution is starting (logs usually ready after ~%d seconds)...", int(startingStateDelay.Seconds())))
			s.sleeper.Sleep(startingStateDelay)
		} else if status.Status == string(constants.ExecutionTerminating) {
			s.output.Infof("Execution is terminating, waiting for final state...")
			s.sleeper.Sleep(retryDelay)
		}
	}
	// If status check fails, proceed with normal retry logic

	var resp *api.LogsResponse
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err = s.client.GetLogs(ctx, executionID)
		if err == nil {
			return resp, nil
		}

		// Check if it's a logs-not-ready error (503 - log stream doesn't exist yet)
		if isLogsNotReadyError(err) {
			if attempt < maxRetries {
				s.output.Infof("Logs not available yet, waiting %d seconds... (attempt %d/%d)",
					int(retryDelay.Seconds()), attempt+1, maxRetries+1)
				s.sleeper.Sleep(retryDelay)
				continue
			}
		}

		// For non-retryable errors or final attempt, return error
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	return nil, fmt.Errorf("failed to get logs: %w", err)
}

// readWebSocketMessages reads messages from WebSocket and sends log events to a channel
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

// streamLogsViaWebSocket connects to WebSocket and streams logs in real-time
func (s *LogsService) streamLogsViaWebSocket(
	websocketURL string,
	startingLineNumber int,
	webURL string,
	executionID string,
) error {
	s.output.Infof("Connecting to log stream...")
	conn, httpResp, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		s.output.Warningf("Failed to connect to WebSocket: %v", err)
		return err
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
	go func() {
		lineNumber := startingLineNumber
		for logEvent := range logChan {
			lineNumber++
			s.printLogLine(lineNumber, logEvent)
		}
	}()

	select {
	case <-sigChan:
		s.output.Infof("Received interrupt signal, closing connection...")
		closeOnce.Do(func() { close(done) })
		s.printWebviewerURL(webURL, executionID)
	case <-done:
		s.output.Infof("WebSocket connection closed")
		s.printWebviewerURL(webURL, executionID)
	}

	return nil
}

// DisplayLogs retrieves static logs and then streams new logs via WebSocket in real-time
// If the execution has already completed, it displays static logs only and skips WebSocket streaming
func (s *LogsService) DisplayLogs(ctx context.Context, executionID, webURL string) error {
	// Fetch static logs with retry logic
	resp, err := s.fetchLogsWithRetry(ctx, executionID)
	if err != nil {
		return err
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
		return fmt.Errorf("websocket streaming function is not configured")
	}

	s.output.Infof("Execution status: %s. Streaming logs via WebSocket...", resp.Status)
	return s.stream(resp.WebSocketURL, len(resp.Events), webURL, executionID)
}

// displayLogEvents displays all log events in a sorted table
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

// printWebviewerURL prints the web application URL
func (s *LogsService) printWebviewerURL(webURL, executionID string) {
	urlStr := infra.BuildLogsURL(webURL, executionID)
	s.output.Infof("View logs in web viewer: %s", s.output.Cyan(urlStr))
}
