// Package orchestrator provides the core orchestrator service for runvoy.
// This file contains health reconciliation functionality.
package orchestrator

import (
	"context"

	"runvoy/internal/backend/health"
)

// ReconcileResources performs health reconciliation for all resources.
// This method allows synchronous execution via API (future API endpoint).
func (s *Service) ReconcileResources(ctx context.Context) (*health.Report, error) {
	if s.healthManager == nil {
		return nil, ErrHealthManagerNotAvailable
	}

	return s.healthManager.Reconcile(ctx)
}

// ErrHealthManagerNotAvailable is returned when health manager is not configured.
var ErrHealthManagerNotAvailable = &HealthManagerNotAvailableError{}

// HealthManagerNotAvailableError indicates that the health manager is not available.
type HealthManagerNotAvailableError struct{}

func (e *HealthManagerNotAvailableError) Error() string {
	return "health manager not available"
}
