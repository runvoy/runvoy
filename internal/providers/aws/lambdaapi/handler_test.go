package lambdaapi

import (
	"testing"
	"time"

	"runvoy/internal/backend/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestNewHandler_ReturnsLambdaHandler(t *testing.T) {
	svc := &orchestrator.Service{}
	handler := NewHandler(svc, 5*time.Second, []string{"https://example.com"})

	assert.NotNil(t, handler)
}

func TestNewHandler_PanicsWithNilService(t *testing.T) {
	assert.Panics(t, func() {
		NewHandler(nil, time.Second, nil)
	})
}
