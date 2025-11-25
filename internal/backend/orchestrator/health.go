package orchestrator

import (
	"context"

	"runvoy/internal/api"
)

// ReconcileResources performs health reconciliation for all resources.
// This method allows synchronous execution via API.
func (s *Service) ReconcileResources(ctx context.Context) (*api.HealthReport, error) {
	return s.healthManager.Reconcile(ctx)
}
