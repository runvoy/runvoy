// Package orchestrator provides the core orchestrator service for runvoy.
// This file contains health reconciliation functionality.
package orchestrator

import (
	"context"

	"runvoy/internal/backend/health"
)

// ReconcileResources performs health reconciliation for all resources.
// This method allows synchronous execution via API.
func (s *Service) ReconcileResources(ctx context.Context) (*health.Report, error) {
	return s.healthManager.Reconcile(ctx)
}
