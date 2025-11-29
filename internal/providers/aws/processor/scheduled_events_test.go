package aws

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/runvoy/runvoy/internal/api"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

func TestHandleScheduledEvent_Comprehensive_InvalidJSONDetail(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	mockHealthManager := &mockHealthManager{}
	mockRepo := &mockExecutionRepo{}
	wsManager := &mockWebSocketHandler{}
	processor := NewProcessor(mockRepo, &noopLogEventRepo{}, wsManager, mockHealthManager, logger)

	// Test with invalid JSON in detail (should trigger unmarshal error)
	event := events.CloudWatchEvent{
		DetailType: "Scheduled Event",
		Source:     "aws.events",
		Detail:     json.RawMessage(`{"runvoy_event": "health_reconcile"`), // Invalid JSON - missing closing brace
	}

	err := processor.handleScheduledEvent(ctx, &event, logger)

	// Should return nil (gracefully ignores invalid JSON)
	assert.NoError(t, err)
}

func TestHandleScheduledEvent_Comprehensive_InvalidSource(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	mockHealthManager := &mockHealthManager{}
	mockRepo := &mockExecutionRepo{}
	wsManager := &mockWebSocketHandler{}
	processor := NewProcessor(mockRepo, &noopLogEventRepo{}, wsManager, mockHealthManager, logger)

	// Test with invalid source (not aws.events)
	event := events.CloudWatchEvent{
		DetailType: "Scheduled Event",
		Source:     "custom.source", // Not aws.events
		Detail:     json.RawMessage(`{"runvoy_event": "health_reconcile"}`),
	}

	err := processor.handleScheduledEvent(ctx, &event, logger)

	// Should return nil (gracefully ignores unexpected source)
	assert.NoError(t, err)
}

func TestHandleScheduledEvent_Comprehensive_HealthReconcile(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	reconcileCalled := false
	mockHealthManager := &mockHealthManager{
		reconcileFunc: func(_ context.Context) (*api.HealthReport, error) {
			reconcileCalled = true
			return &api.HealthReport{
				ReconciledCount: 5,
				ErrorCount:      0,
			}, nil
		},
	}

	mockRepo := &mockExecutionRepo{}
	wsManager := &mockWebSocketHandler{}
	processor := NewProcessor(mockRepo, &noopLogEventRepo{}, wsManager, mockHealthManager, logger)

	event := events.CloudWatchEvent{
		DetailType: "Scheduled Event",
		Source:     "aws.events",
		Detail:     json.RawMessage(`{"runvoy_event": "` + awsConstants.ScheduledEventHealthReconcile + `"}`),
	}

	err := processor.handleScheduledEvent(ctx, &event, logger)

	assert.NoError(t, err)
	assert.True(t, reconcileCalled, "health reconcile should have been called")
}

func TestHandleScheduledEvent_Comprehensive_UnknownEventType(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	mockHealthManager := &mockHealthManager{}
	mockRepo := &mockExecutionRepo{}
	wsManager := &mockWebSocketHandler{}
	processor := NewProcessor(mockRepo, &noopLogEventRepo{}, wsManager, mockHealthManager, logger)

	event := events.CloudWatchEvent{
		DetailType: "Scheduled Event",
		Source:     "aws.events",
		Detail:     json.RawMessage(`{"runvoy_event": "unknown_event_type"}`),
	}

	err := processor.handleScheduledEvent(ctx, &event, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected runvoy_event value")
}

func TestHandleScheduledEvent_Comprehensive_EmptyRunvoyEvent(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	mockHealthManager := &mockHealthManager{}
	mockRepo := &mockExecutionRepo{}
	wsManager := &mockWebSocketHandler{}
	processor := NewProcessor(mockRepo, &noopLogEventRepo{}, wsManager, mockHealthManager, logger)

	// Test with valid JSON but empty runvoy_event field
	event := events.CloudWatchEvent{
		DetailType: "Scheduled Event",
		Source:     "aws.events",
		Detail:     json.RawMessage(`{"runvoy_event": ""}`),
	}

	err := processor.handleScheduledEvent(ctx, &event, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected runvoy_event value")
}

func TestHandleHealthReconcileScheduledEvent_Comprehensive_Success(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	mockHealthManager := &mockHealthManager{
		reconcileFunc: func(_ context.Context) (*api.HealthReport, error) {
			return &api.HealthReport{
				ReconciledCount: 3,
				ErrorCount:      0,
				ComputeStatus: api.ComputeHealthStatus{
					VerifiedCount:  5,
					RecreatedCount: 2,
				},
				SecretsStatus: api.SecretsHealthStatus{
					VerifiedCount: 10,
				},
				IdentityStatus: api.IdentityHealthStatus{
					DefaultRolesVerified: true,
				},
			}, nil
		},
	}

	mockRepo := &mockExecutionRepo{}
	wsManager := &mockWebSocketHandler{}
	processor := NewProcessor(mockRepo, &noopLogEventRepo{}, wsManager, mockHealthManager, logger)

	err := processor.handleHealthReconcileScheduledEvent(ctx, logger)

	assert.NoError(t, err)
}

func TestHandleHealthReconcileScheduledEvent_Comprehensive_ReconcileError(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	mockHealthManager := &mockHealthManager{
		reconcileFunc: func(_ context.Context) (*api.HealthReport, error) {
			return nil, assert.AnError
		},
	}

	mockRepo := &mockExecutionRepo{}
	wsManager := &mockWebSocketHandler{}
	processor := NewProcessor(mockRepo, &noopLogEventRepo{}, wsManager, mockHealthManager, logger)

	err := processor.handleHealthReconcileScheduledEvent(ctx, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health reconciliation failed")
}

func TestHandleHealthReconcileScheduledEvent_Comprehensive_WithErrors(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	mockHealthManager := &mockHealthManager{
		reconcileFunc: func(_ context.Context) (*api.HealthReport, error) {
			return &api.HealthReport{
				ReconciledCount: 2,
				ErrorCount:      3, // Has errors - should use Warn log level
				Issues: []api.HealthIssue{
					{Severity: "error", Message: "Test issue 1"},
					{Severity: "error", Message: "Test issue 2"},
					{Severity: "error", Message: "Test issue 3"},
				},
			}, nil
		},
	}

	mockRepo := &mockExecutionRepo{}
	wsManager := &mockWebSocketHandler{}
	processor := NewProcessor(mockRepo, &noopLogEventRepo{}, wsManager, mockHealthManager, logger)

	err := processor.handleHealthReconcileScheduledEvent(ctx, logger)

	// Should succeed but log at Warn level due to error count > 0
	assert.NoError(t, err)
}
