package aws

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	"runvoy/internal/testutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock execution repository for cloud event tests
type mockExecRepoForCloudEvents struct {
	getExecutionFunc    func(ctx context.Context, executionID string) (*api.Execution, error)
	updateExecutionFunc func(ctx context.Context, exec *api.Execution) error
}

func (m *mockExecRepoForCloudEvents) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	if m.getExecutionFunc != nil {
		return m.getExecutionFunc(ctx, executionID)
	}
	return nil, nil
}

func (m *mockExecRepoForCloudEvents) UpdateExecution(ctx context.Context, exec *api.Execution) error {
	if m.updateExecutionFunc != nil {
		return m.updateExecutionFunc(ctx, exec)
	}
	return nil
}

func (m *mockExecRepoForCloudEvents) CreateExecution(ctx context.Context, exec *api.Execution) error {
	return nil
}

func (m *mockExecRepoForCloudEvents) ListExecutions(ctx context.Context, limit int, statuses []string) ([]*api.Execution, error) {
	return []*api.Execution{}, nil
}

func (m *mockExecRepoForCloudEvents) GetExecutionsByRequestID(ctx context.Context, requestID string) ([]*api.Execution, error) {
	return []*api.Execution{}, nil
}

// Mock WebSocket manager for cloud event tests
type mockWSManagerForCloudEvents struct {
	notifyExecutionUpdateFunc func(ctx context.Context, exec *api.Execution) error
	handleRequestFunc         func(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error)
	sendLogsToExecutionFunc   func(ctx context.Context, executionID *string, logEvents []api.LogEvent) error
}

func (m *mockWSManagerForCloudEvents) NotifyExecutionUpdate(ctx context.Context, exec *api.Execution) error {
	if m.notifyExecutionUpdateFunc != nil {
		return m.notifyExecutionUpdateFunc(ctx, exec)
	}
	return nil
}

func (m *mockWSManagerForCloudEvents) NotifyExecutionLogs(ctx context.Context, executionID string, logEvent api.LogEvent) error {
	return nil
}

func (m *mockWSManagerForCloudEvents) NotifyExecutionCompletion(ctx context.Context, executionID *string) error {
	return nil
}

func (m *mockWSManagerForCloudEvents) SendMessage(ctx context.Context, connectionID string, message []byte) error {
	return nil
}

func (m *mockWSManagerForCloudEvents) CloseConnection(ctx context.Context, connectionID string) error {
	return nil
}

func (m *mockWSManagerForCloudEvents) HandleRequest(ctx context.Context, rawEvent *json.RawMessage, reqLogger *slog.Logger) (bool, error) {
	if m.handleRequestFunc != nil {
		return m.handleRequestFunc(ctx, rawEvent, reqLogger)
	}
	return true, nil
}

func (m *mockWSManagerForCloudEvents) SendLogsToExecution(ctx context.Context, executionID *string, logEvents []api.LogEvent) error {
	if m.sendLogsToExecutionFunc != nil {
		return m.sendLogsToExecutionFunc(ctx, executionID, logEvents)
	}
	return nil
}

func (m *mockWSManagerForCloudEvents) GenerateWebSocketURL(ctx context.Context, executionID string, userEmail *string, clientIP *string) string {
	return ""
}


// Test the main Handle method routing logic
func TestProcessor_Handle_ECSEvent(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{
		getExecutionFunc: func(ctx context.Context, executionID string) (*api.Execution, error) {
			return &api.Execution{
				ExecutionID: executionID,
				Status:      string(constants.ExecutionStarting),
			}, nil
		},
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			assert.Equal(t, string(constants.ExecutionRunning), exec.Status)
			return nil
		},
	}

	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// Create ECS Task State Change event
	ecsEvent := events.CloudWatchEvent{
		Source:     "aws.ecs",
		DetailType: "ECS Task State Change",
		Detail:     json.RawMessage(`{"taskArn":"arn:aws:ecs:us-east-1:123456789:task/cluster/exec-test-123","lastStatus":"RUNNING"}`),
	}

	eventJSON, err := json.Marshal(ecsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	_, err = processor.Handle(context.Background(), &rawMsg)
	assert.NoError(t, err)
}

func TestProcessor_Handle_ScheduledEvent(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}

	// Mock health manager (reuse from backend_test.go)
	healthManager := &mockHealthManager{
		reconcileFunc: func(ctx context.Context) (*api.HealthReport, error) {
			return &api.HealthReport{}, nil
		},
	}

	processor := NewProcessor(execRepo, wsManager, healthManager, testutil.SilentLogger())

	// Create Scheduled Event for health reconciliation
	scheduledEvent := events.CloudWatchEvent{
		Source:     "aws.events",
		DetailType: "Scheduled Event",
		Detail:     json.RawMessage(`{"runvoy_event": "health_reconcile"}`),
		Resources:  []string{"arn:aws:events:us-east-1:123456789:rule/health-check"},
	}

	eventJSON, err := json.Marshal(scheduledEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	_, err = processor.Handle(context.Background(), &rawMsg)
	assert.NoError(t, err)
}

func TestProcessor_Handle_UnhandledCloudWatchEventType(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// Create CloudWatch event with unknown detail type
	unknownEvent := events.CloudWatchEvent{
		Source:     "aws.unknown",
		DetailType: "Unknown Event Type",
		Detail:     json.RawMessage(`{}`),
	}

	eventJSON, err := json.Marshal(unknownEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	// Should handle gracefully and ignore
	_, err = processor.Handle(context.Background(), &rawMsg)
	assert.NoError(t, err)
}

func TestProcessor_Handle_LogsEvent(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// Create CloudWatch Logs event
	logsEvent := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: "H4sIAAAAAAAAAA==", // Empty base64 gzipped data
		},
	}

	eventJSON, err := json.Marshal(logsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	// Logs event should be handled (even if processing fails due to empty data)
	_, err = processor.Handle(context.Background(), &rawMsg)
	// We expect an error because the data is not valid, but it should be handled
	assert.Error(t, err)
}

func TestProcessor_Handle_WebSocketConnectEvent(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// Create API Gateway WebSocket connect event
	wsEvent := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			EventType:    "CONNECT",
			ConnectionID: "conn-123",
			RouteKey:     "$connect",
		},
	}

	eventJSON, err := json.Marshal(wsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	resp, err := processor.Handle(context.Background(), &rawMsg)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestProcessor_Handle_WebSocketDisconnectEvent(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// Create API Gateway WebSocket disconnect event
	wsEvent := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			EventType:    "DISCONNECT",
			ConnectionID: "conn-123",
			RouteKey:     "$disconnect",
		},
	}

	eventJSON, err := json.Marshal(wsEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	resp, err := processor.Handle(context.Background(), &rawMsg)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestProcessor_Handle_UnknownEventType(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// Create completely unrecognized event
	unknownEvent := map[string]interface{}{
		"unknownField": "unknownValue",
	}

	eventJSON, err := json.Marshal(unknownEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	_, err = processor.Handle(context.Background(), &rawMsg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unhandled event type")
}

func TestProcessor_Handle_InvalidJSON(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// Invalid JSON
	invalidJSON := json.RawMessage(`{invalid json}`)

	_, err := processor.Handle(context.Background(), &invalidJSON)
	assert.Error(t, err)
}

func TestProcessor_HandleCloudEvent_ECSTaskStateChange(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{
		getExecutionFunc: func(ctx context.Context, executionID string) (*api.Execution, error) {
			return &api.Execution{
				ExecutionID: executionID,
				Status:      string(constants.ExecutionStarting),
			}, nil
		},
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			return nil
		},
	}

	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	event := events.CloudWatchEvent{
		Source:     "aws.ecs",
		DetailType: "ECS Task State Change",
		Detail:     json.RawMessage(`{"taskArn":"arn:aws:ecs:us-east-1:123456789:task/cluster/exec-test-123","lastStatus":"RUNNING"}`),
	}

	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleCloudEvent(context.Background(), &rawMsg, testutil.SilentLogger())
	assert.True(t, handled)
	assert.NoError(t, err)
}

func TestProcessor_HandleCloudEvent_ScheduledEvent(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}

	healthManager := &mockHealthManager{
		reconcileFunc: func(ctx context.Context) (*api.HealthReport, error) {
			return &api.HealthReport{}, nil
		},
	}

	processor := NewProcessor(execRepo, wsManager, healthManager, testutil.SilentLogger())

	event := events.CloudWatchEvent{
		Source:     "aws.events",
		DetailType: "Scheduled Event",
		Detail:     json.RawMessage(`{"runvoy_event": "health_reconcile"}`),
		Resources:  []string{"arn:aws:events:us-east-1:123456789:rule/health-check"},
	}

	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleCloudEvent(context.Background(), &rawMsg, testutil.SilentLogger())
	assert.True(t, handled)
	assert.NoError(t, err)
}

func TestProcessor_HandleCloudEvent_UnhandledDetailType(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	event := events.CloudWatchEvent{
		Source:     "aws.ec2",
		DetailType: "EC2 Instance State Change",
		Detail:     json.RawMessage(`{}`),
	}

	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleCloudEvent(context.Background(), &rawMsg, testutil.SilentLogger())
	assert.True(t, handled)
	assert.NoError(t, err) // Should handle gracefully and log warning
}

func TestProcessor_HandleCloudEvent_NotCloudWatchEvent(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// Not a CloudWatch event structure
	notCWEvent := map[string]interface{}{
		"someOtherField": "value",
	}

	eventJSON, err := json.Marshal(notCWEvent)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleCloudEvent(context.Background(), &rawMsg, testutil.SilentLogger())
	assert.False(t, handled)
	assert.NoError(t, err)
}

func TestProcessor_HandleCloudEvent_MissingSource(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// CloudWatch event without Source field
	event := events.CloudWatchEvent{
		DetailType: "ECS Task State Change",
		Detail:     json.RawMessage(`{}`),
	}

	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleCloudEvent(context.Background(), &rawMsg, testutil.SilentLogger())
	assert.False(t, handled)
	assert.NoError(t, err)
}

func TestProcessor_HandleCloudEvent_MissingDetailType(t *testing.T) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	// CloudWatch event without DetailType field
	event := events.CloudWatchEvent{
		Source: "aws.ecs",
		Detail: json.RawMessage(`{}`),
	}

	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)
	rawMsg := json.RawMessage(eventJSON)

	handled, err := processor.handleCloudEvent(context.Background(), &rawMsg, testutil.SilentLogger())
	assert.False(t, handled)
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkProcessor_Handle_ECSEvent(b *testing.B) {
	execRepo := &mockExecRepoForCloudEvents{
		getExecutionFunc: func(ctx context.Context, executionID string) (*api.Execution, error) {
			return &api.Execution{
				ExecutionID: executionID,
				Status:      string(constants.ExecutionStarting),
			}, nil
		},
		updateExecutionFunc: func(ctx context.Context, exec *api.Execution) error {
			return nil
		},
	}

	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	ecsEvent := events.CloudWatchEvent{
		Source:     "aws.ecs",
		DetailType: "ECS Task State Change",
		Detail:     json.RawMessage(`{"taskArn":"arn:aws:ecs:us-east-1:123456789:task/cluster/exec-test-123","lastStatus":"RUNNING"}`),
	}

	eventJSON, _ := json.Marshal(ecsEvent)
	rawMsg := json.RawMessage(eventJSON)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = processor.Handle(context.Background(), &rawMsg)
	}
}

func BenchmarkProcessor_Handle_WebSocketEvent(b *testing.B) {
	execRepo := &mockExecRepoForCloudEvents{}
	wsManager := &mockWSManagerForCloudEvents{}
	processor := NewProcessor(execRepo, wsManager, nil, testutil.SilentLogger())

	wsEvent := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			EventType:    "CONNECT",
			ConnectionID: "conn-123",
			RouteKey:     "$connect",
		},
	}

	eventJSON, _ := json.Marshal(wsEvent)
	rawMsg := json.RawMessage(eventJSON)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = processor.Handle(context.Background(), &rawMsg)
	}
}
