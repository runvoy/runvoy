package cmd

import (
	"bytes"
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
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	"runvoy/internal/output"
	"strings"
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
	Args:  cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

const (
	minRegexMatches = 2 // Minimum number of regex matches expected: full match + capture group
)

// isNotFoundError checks if an error represents a 404 Not Found status.
// It handles both AppError types and client error strings formatted as [404] ...
func isNotFoundError(err error) bool {
	// First check if it's an AppError with status code 404
	if statusCode := apperrors.GetStatusCode(err); statusCode == http.StatusNotFound {
		return true
	}

	// Check if it's an AppError with NOT_FOUND error code
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Code == apperrors.ErrCodeNotFound || appErr.StatusCode == http.StatusNotFound
	}

	// Parse client error format: [404] error message
	// The client formats errors as: "[%d] %s: %s"
	errStr := err.Error()
	re := regexp.MustCompile(`\[(\d+)\]`)
	matches := re.FindStringSubmatch(errStr)
	if len(matches) >= minRegexMatches {
		if matches[1] == fmt.Sprintf("%d", http.StatusNotFound) {
			return true
		}
	}

	return false
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

// fetchLogsWithRetry fetches logs with retry logic for 404 errors (execution starting up)
func (s *LogsService) fetchLogsWithRetry(ctx context.Context, executionID string) (*api.LogsResponse, error) {
	const (
		maxRetries = 2
		retryDelay = 10 * time.Second
	)

	var resp *api.LogsResponse
	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err = s.client.GetLogs(ctx, executionID)
		if err == nil {
			return resp, nil
		}

		// Check if it's a 404 error (log stream doesn't exist yet)
		if isNotFoundError(err) {
			if attempt < maxRetries {
				s.output.Infof("Logs not available yet, waiting %d seconds... (attempt %d/%d)",
					int(retryDelay.Seconds()), attempt+1, maxRetries+1)
				time.Sleep(retryDelay)
				continue
			}
		}

		// For non-404 errors or final attempt, return error
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	return nil, fmt.Errorf("failed to get logs: %w", err)
}

// readWebSocketMessages reads messages from WebSocket and updates the log map
// Handles newline-delimited JSON (NDJSON) format for batched log events
// Updates lastIndex as logs are received
func (s *LogsService) readWebSocketMessages(
	conn *websocket.Conn,
	logMap map[int64]api.LogEvent,
	mu *sync.Mutex,
	lastIndex *int64,
	done <-chan struct{},
) {
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

			s.processWebSocketMessage(messageBytes, logMap, mu, lastIndex, conn)
		}
	}
}

// processWebSocketMessage processes a single WebSocket message (NDJSON format).
func (s *LogsService) processWebSocketMessage(
	messageBytes []byte,
	logMap map[int64]api.LogEvent,
	mu *sync.Mutex,
	lastIndex *int64,
	conn *websocket.Conn,
) {
	//nolint:modernize // bytes.Split is fine for NDJSON processing
	lines := bytes.Split(messageBytes, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var rawMessage map[string]any
		if err := json.Unmarshal(line, &rawMessage); err != nil {
			continue
		}

		if msgType, ok := rawMessage["type"].(string); ok && msgType == string(api.WebSocketMessageTypeDisconnect) {
			s.output.Blank()
			s.output.Infof("Execution completed. Closing connection...")
			_ = conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Execution completed"),
			)
			return
		}

		var logEvent api.LogEvent
		if err := json.Unmarshal(line, &logEvent); err != nil {
			continue
		}

		s.processLogEvent(logEvent, logMap, mu, lastIndex)
	}
}

// processLogEvent processes a single log event, updating the log map and lastIndex.
func (s *LogsService) processLogEvent(
	logEvent api.LogEvent,
	logMap map[int64]api.LogEvent,
	mu *sync.Mutex,
	lastIndex *int64,
) {
	mu.Lock()
	defer mu.Unlock()

	// Use index-based deduplication
	if logEvent.Index > 0 {
		if _, exists := logMap[logEvent.Index]; !exists {
			logMap[logEvent.Index] = logEvent
			s.printLogLine(int(logEvent.Index), logEvent)
			// Update lastIndex to highest seen index
			if logEvent.Index > *lastIndex {
				*lastIndex = logEvent.Index
			}
		}
	} else {
		// Fallback for logs without index (shouldn't happen with new system)
		// Use timestamp-based deduplication as fallback
		if _, exists := logMap[logEvent.Timestamp]; !exists {
			logMap[logEvent.Timestamp] = logEvent
			lineNumber := len(logMap)
			s.printLogLine(lineNumber, logEvent)
		}
	}
}

// streamLogsViaWebSocket connects to WebSocket and streams logs in real-time
func (s *LogsService) streamLogsViaWebSocket(
	websocketURL string,
	logMap map[int64]api.LogEvent,
	mu *sync.Mutex,
	lastIndex *int64,
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

	done := make(chan struct{})
	var closeOnce sync.Once

	go func() {
		defer closeOnce.Do(func() { close(done) })
		s.readWebSocketMessages(conn, logMap, mu, lastIndex, done)
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

	return nil
}

// DisplayLogs retrieves static logs and then streams new logs via WebSocket in real-time
func (s *LogsService) DisplayLogs(ctx context.Context, executionID, webURL string) error {
	// Fetch static logs with retry logic
	resp, err := s.fetchLogsWithRetry(ctx, executionID)
	if err != nil {
		return err
	}

	s.displayLogEvents(resp.Events)

	if resp.WebSocketURL == "" {
		// No WebSocket URL means the execution is completed
		s.printWebviewerURL(webURL, executionID)
		return nil
	}

	// Track last_index from initial response
	lastIndex := resp.LastIndex
	if lastIndex == 0 && len(resp.Events) > 0 {
		// Fallback: use highest index from events
		for _, event := range resp.Events {
			if event.Index > lastIndex {
				lastIndex = event.Index
			}
		}
	}

	// Build log map for WebSocket streaming deduplication (index-based)
	logMap := make(map[int64]api.LogEvent)
	var mu sync.Mutex
	for _, log := range resp.Events {
		if log.Index > 0 {
			logMap[log.Index] = log
		}
	}

	// Update WebSocket URL with last_index if not already present
	websocketURL := s.buildWebSocketURL(resp.WebSocketURL, lastIndex)

	// Stream logs via WebSocket
	_ = s.streamLogsViaWebSocket(websocketURL, logMap, &mu, &lastIndex)

	s.printWebviewerURL(webURL, executionID)
	return nil
}

// buildWebSocketURL ensures the WebSocket URL includes last_index parameter.
func (s *LogsService) buildWebSocketURL(baseURL string, lastIndex int64) string {
	// Check if last_index is already in the URL
	if strings.Contains(baseURL, "last_index=") {
		return baseURL
	}

	// Append last_index parameter
	separator := "?"
	if strings.Contains(baseURL, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%slast_index=%d", baseURL, separator, lastIndex)
}

// displayLogEvents displays all log events in a sorted table
func (s *LogsService) displayLogEvents(events []api.LogEvent) {
	s.output.Blank()
	rows := [][]string{}
	for _, event := range events {
		// Use Index as line number if available, otherwise use position (1-indexed)
		var lineNumber int64
		if event.Index > 0 {
			lineNumber = event.Index
		} else {
			lineNumber = int64(len(rows) + 1)
		}
		rows = append(rows, []string{
			s.output.Bold(fmt.Sprintf("%d", lineNumber)),
			time.Unix(event.Timestamp/constants.MillisecondsPerSecond, 0).UTC().Format(time.DateTime),
			event.Message,
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
	s.output.Infof("View logs in web viewer: %s?execution_id=%s",
		webURL, s.output.Cyan(executionID))
}
