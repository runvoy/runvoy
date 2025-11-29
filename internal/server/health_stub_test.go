package server

import (
	"context"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/backend/contract"
)

// noopHealthManager provides a minimal HealthManager for tests that don't assert on health behavior.
type noopHealthManager struct{}

var _ contract.HealthManager = (*noopHealthManager)(nil)

func (n *noopHealthManager) Reconcile(_ context.Context) (*api.HealthReport, error) {
	return &api.HealthReport{}, nil
}
