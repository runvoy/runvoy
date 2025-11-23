package health

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestManager_Interface verifies that the Manager interface is properly defined.
func TestManager_Interface(t *testing.T) {
	// This test ensures the Manager interface exists and has the expected method signature
	var _ Manager = (*testManager)(nil)

	manager := &testManager{}
	report, err := manager.Reconcile(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, report)
}

// testManager is a minimal implementation for testing the interface
type testManager struct{}

func (t *testManager) Reconcile(ctx context.Context) (*Report, error) {
	return &Report{}, nil
}
