package aws

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"runvoy/internal/api"
	awsConstants "runvoy/internal/providers/aws/constants"
	"runvoy/internal/testutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogEventRepoForLogsEvents implements a mock log event repository for testing logs_events.go
type mockLogEventRepoForLogsEvents struct {
	saveLogEventsFunc func(ctx context.Context, executionID string, logEvents []api.LogEvent) error
}

func (m *mockLogEventRepoForLogsEvents) SaveLogEvents(
	ctx context.Context, executionID string, logEvents []api.LogEvent) error {
	if m.saveLogEventsFunc != nil {
		return m.saveLogEventsFunc(ctx, executionID, logEvents)
	}
	return nil
}

func (m *mockLogEventRepoForLogsEvents) ListLogEvents(_ context.Context, _ string) ([]api.LogEvent, error) {
	return nil, nil
}

func (m *mockLogEventRepoForLogsEvents) DeleteLogEvents(_ context.Context, _ string) error {
	return nil
}

// mockWebSocketManagerForLogsEvents implements a mock WebSocket manager for testing logs_events.go
type mockWebSocketManagerForLogsEvents struct {
	sendLogsFunc func(ctx context.Context, executionID *string) error
}

func (m *mockWebSocketManagerForLogsEvents) HandleRequest(
	_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
	return false, nil
}

func (m *mockWebSocketManagerForLogsEvents) NotifyExecutionCompletion(_ context.Context, _ *string) error {
	return nil
}

func (m *mockWebSocketManagerForLogsEvents) SendLogsToExecution(ctx context.Context, executionID *string) error {
	if m.sendLogsFunc != nil {
		return m.sendLogsFunc(ctx, executionID)
	}
	return nil
}

func (m *mockWebSocketManagerForLogsEvents) GenerateWebSocketURL(
	_ context.Context, _ string, _, _ *string) string {
	return ""
}

// createValidCloudWatchLogsData creates a valid base64-encoded gzipped CloudWatch Logs data.
// The logGroup parameter is kept for API consistency even though it's currently always "/aws/ecs/runvoy".
//
//nolint:unparam // logGroup is kept as parameter for API consistency and future flexibility
func createValidCloudWatchLogsData(
	logGroup, logStream string, logEvents []events.CloudwatchLogsLogEvent) (string, error) {
	data := events.CloudwatchLogsData{
		LogGroup:  logGroup,
		LogStream: logStream,
		LogEvents: logEvents,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, writeErr := gz.Write(jsonData); writeErr != nil {
		return "", writeErr
	}
	if closeErr := gz.Close(); closeErr != nil {
		return "", closeErr
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func TestHandleLogsEvent_Comprehensive_Success(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	executionID := "exec-123"

	var savedExecutionID string
	var savedLogEvents []api.LogEvent

	mockLogRepo := &mockLogEventRepoForLogsEvents{
		saveLogEventsFunc: func(_ context.Context, execID string, events []api.LogEvent) error {
			savedExecutionID = execID
			savedLogEvents = events
			return nil
		},
	}

	wsManager := &mockWebSocketManagerForLogsEvents{
		sendLogsFunc: func(_ context.Context, _ *string) error {
			return nil
		},
	}

	processor := NewProcessor(nil, mockLogRepo, wsManager, nil, logger)

	// Create valid CloudWatch Logs event
	logStream := awsConstants.BuildLogStreamName(executionID)
	logEvents := []events.CloudwatchLogsLogEvent{
		{
			ID:        "event-1",
			Timestamp: time.Now().UnixMilli(),
			Message:   "Test log message 1",
		},
		{
			ID:        "event-2",
			Timestamp: time.Now().UnixMilli(),
			Message:   "Test log message 2",
		},
	}

	logsData, err := createValidCloudWatchLogsData("/aws/ecs/runvoy", logStream, logEvents)
	require.NoError(t, err)

	logsEvent := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: logsData,
		},
	}

	eventJSON, err := json.Marshal(logsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleLogsEvent(ctx, &rawMsg, logger)

	assert.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, executionID, savedExecutionID)
	assert.Len(t, savedLogEvents, 2)
	assert.Equal(t, "event-1", savedLogEvents[0].EventID)
	assert.Equal(t, "Test log message 1", savedLogEvents[0].Message)
	assert.Equal(t, "event-2", savedLogEvents[1].EventID)
	assert.Equal(t, "Test log message 2", savedLogEvents[1].Message)
}

func TestHandleLogsEvent_Comprehensive_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	processor := NewProcessor(nil, &mockLogEventRepoForLogsEvents{}, &mockWebSocketManagerForLogsEvents{}, nil, logger)

	// Invalid JSON
	rawMsg := json.RawMessage(`{"invalid": json}`)

	handled, err := processor.handleLogsEvent(ctx, &rawMsg, logger)

	assert.NoError(t, err)
	assert.False(t, handled) // Should return false for invalid JSON
}

func TestHandleLogsEvent_Comprehensive_EmptyData(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	processor := NewProcessor(nil, &mockLogEventRepoForLogsEvents{}, &mockWebSocketManagerForLogsEvents{}, nil, logger)

	logsEvent := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: "", // Empty data
		},
	}

	eventJSON, err := json.Marshal(logsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleLogsEvent(ctx, &rawMsg, logger)

	assert.NoError(t, err)
	assert.False(t, handled) // Should return false for empty data
}

func TestHandleLogsEvent_Comprehensive_ParseError(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	processor := NewProcessor(nil, &mockLogEventRepoForLogsEvents{}, &mockWebSocketManagerForLogsEvents{}, nil, logger)

	logsEvent := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: "invalid-base64-data!!!", // Invalid base64/gzip data
		},
	}

	eventJSON, err := json.Marshal(logsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleLogsEvent(ctx, &rawMsg, logger)

	assert.Error(t, err)
	assert.True(t, handled) // Should return true (handled) even on error
}

func TestHandleLogsEvent_Comprehensive_MissingExecutionID(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	var saveCalled bool
	mockLogRepo := &mockLogEventRepoForLogsEvents{
		saveLogEventsFunc: func(_ context.Context, _ string, _ []api.LogEvent) error {
			saveCalled = true
			return nil
		},
	}

	processor := NewProcessor(nil, mockLogRepo, &mockWebSocketManagerForLogsEvents{}, nil, logger)

	// Create logs event with invalid log stream (no execution ID)
	logStream := "invalid/log/stream/format"
	logEvents := []events.CloudwatchLogsLogEvent{
		{
			ID:        "event-1",
			Timestamp: time.Now().UnixMilli(),
			Message:   "Test message",
		},
	}

	logsData, err := createValidCloudWatchLogsData("/aws/ecs/runvoy", logStream, logEvents)
	require.NoError(t, err)

	logsEvent := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: logsData,
		},
	}

	eventJSON, err := json.Marshal(logsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleLogsEvent(ctx, &rawMsg, logger)

	assert.NoError(t, err)
	assert.True(t, handled)
	assert.False(t, saveCalled) // Should not save when execution ID is missing
}

func TestHandleLogsEvent_Comprehensive_SaveLogEventsError(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	executionID := "exec-123"

	mockLogRepo := &mockLogEventRepoForLogsEvents{
		saveLogEventsFunc: func(_ context.Context, _ string, _ []api.LogEvent) error {
			return assert.AnError // Simulate save error
		},
	}

	processor := NewProcessor(nil, mockLogRepo, &mockWebSocketManagerForLogsEvents{}, nil, logger)

	logStream := awsConstants.BuildLogStreamName(executionID)
	logEvents := []events.CloudwatchLogsLogEvent{
		{
			ID:        "event-1",
			Timestamp: time.Now().UnixMilli(),
			Message:   "Test message",
		},
	}

	logsData, err := createValidCloudWatchLogsData("/aws/ecs/runvoy", logStream, logEvents)
	require.NoError(t, err)

	logsEvent := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: logsData,
		},
	}

	eventJSON, err := json.Marshal(logsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleLogsEvent(ctx, &rawMsg, logger)

	assert.Error(t, err)
	assert.True(t, handled)
	assert.Contains(t, err.Error(), "failed to persist log events")
}

func TestHandleLogsEvent_Comprehensive_WebSocketError(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	executionID := "exec-123"

	var savedLogEvents []api.LogEvent
	mockLogRepo := &mockLogEventRepoForLogsEvents{
		saveLogEventsFunc: func(_ context.Context, _ string, events []api.LogEvent) error {
			savedLogEvents = events
			return nil
		},
	}

	wsManager := &mockWebSocketManagerForLogsEvents{
		sendLogsFunc: func(_ context.Context, _ *string) error {
			return assert.AnError // Simulate WebSocket error
		},
	}

	processor := NewProcessor(nil, mockLogRepo, wsManager, nil, logger)

	logStream := awsConstants.BuildLogStreamName(executionID)
	logEvents := []events.CloudwatchLogsLogEvent{
		{
			ID:        "event-1",
			Timestamp: time.Now().UnixMilli(),
			Message:   "Test message",
		},
	}

	logsData, err := createValidCloudWatchLogsData("/aws/ecs/runvoy", logStream, logEvents)
	require.NoError(t, err)

	logsEvent := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: logsData,
		},
	}

	eventJSON, err := json.Marshal(logsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	// WebSocket errors should not fail the processing
	handled, err := processor.handleLogsEvent(ctx, &rawMsg, logger)

	assert.NoError(t, err) // Should not return error for WebSocket failures
	assert.True(t, handled)
	assert.Len(t, savedLogEvents, 1) // Logs should still be saved
}

func TestHandleLogsEvent_Comprehensive_EmptyLogEvents(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	executionID := "exec-123"

	var saveCalled bool
	mockLogRepo := &mockLogEventRepoForLogsEvents{
		saveLogEventsFunc: func(_ context.Context, _ string, _ []api.LogEvent) error {
			saveCalled = true
			return nil
		},
	}

	processor := NewProcessor(nil, mockLogRepo, &mockWebSocketManagerForLogsEvents{}, nil, logger)

	logStream := awsConstants.BuildLogStreamName(executionID)
	logEvents := []events.CloudwatchLogsLogEvent{} // Empty log events

	logsData, err := createValidCloudWatchLogsData("/aws/ecs/runvoy", logStream, logEvents)
	require.NoError(t, err)

	logsEvent := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: logsData,
		},
	}

	eventJSON, err := json.Marshal(logsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleLogsEvent(ctx, &rawMsg, logger)

	assert.NoError(t, err)
	assert.True(t, handled)
	// Empty log events should still save (empty slice)
	assert.True(t, saveCalled)
}

func TestHandleLogsEvent_Comprehensive_MultipleLogEvents(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()
	executionID := "exec-123"

	var savedLogEvents []api.LogEvent
	mockLogRepo := &mockLogEventRepoForLogsEvents{
		saveLogEventsFunc: func(_ context.Context, _ string, events []api.LogEvent) error {
			savedLogEvents = events
			return nil
		},
	}

	processor := NewProcessor(nil, mockLogRepo, &mockWebSocketManagerForLogsEvents{}, nil, logger)

	logStream := awsConstants.BuildLogStreamName(executionID)
	now := time.Now()
	logEvents := []events.CloudwatchLogsLogEvent{
		{ID: "event-1", Timestamp: now.Add(-2 * time.Second).UnixMilli(), Message: "First message"},
		{ID: "event-2", Timestamp: now.Add(-1 * time.Second).UnixMilli(), Message: "Second message"},
		{ID: "event-3", Timestamp: now.UnixMilli(), Message: "Third message"},
		{ID: "event-4", Timestamp: now.Add(1 * time.Second).UnixMilli(), Message: "Fourth message"},
		{ID: "event-5", Timestamp: now.Add(2 * time.Second).UnixMilli(), Message: "Fifth message"},
	}

	logsData, err := createValidCloudWatchLogsData("/aws/ecs/runvoy", logStream, logEvents)
	require.NoError(t, err)

	logsEvent := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: logsData,
		},
	}

	eventJSON, err := json.Marshal(logsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleLogsEvent(ctx, &rawMsg, logger)

	assert.NoError(t, err)
	assert.True(t, handled)
	assert.Len(t, savedLogEvents, 5)
	assert.Equal(t, "event-1", savedLogEvents[0].EventID)
	assert.Equal(t, "First message", savedLogEvents[0].Message)
	assert.Equal(t, "event-5", savedLogEvents[4].EventID)
	assert.Equal(t, "Fifth message", savedLogEvents[4].Message)
}
