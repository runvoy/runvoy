package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestManager_Interface verifies that the Manager interface is properly defined.
func TestManager_Interface(t *testing.T) {
	// This test ensures the Manager interface exists and has the expected method signatures
	var _ Manager = (*testManager)(nil)

	manager := &testManager{}
	rawEvent := json.RawMessage(`{}`)
	logger := slog.Default()

	handled, err := manager.HandleRequest(context.Background(), &rawEvent, logger)
	assert.NoError(t, err)
	assert.False(t, handled)

	err = manager.NotifyExecutionCompletion(context.Background(), nil)
	assert.NoError(t, err)

	err = manager.SendLogsToExecution(context.Background(), nil)
	assert.NoError(t, err)

	url := manager.GenerateWebSocketURL(context.Background(), "exec-123", nil, nil)
	assert.Equal(t, "", url)
}

// testManager is a minimal implementation for testing the interface
type testManager struct{}

func (t *testManager) HandleRequest(_ context.Context, _ *json.RawMessage, _ *slog.Logger) (bool, error) {
	return false, nil
}

func (t *testManager) NotifyExecutionCompletion(_ context.Context, _ *string) error {
	return nil
}

func (t *testManager) SendLogsToExecution(_ context.Context, _ *string) error {
	return nil
}

func (t *testManager) GenerateWebSocketURL(
	_ context.Context,
	_ string,
	_ *string,
	_ *string,
) string {
	return ""
}
