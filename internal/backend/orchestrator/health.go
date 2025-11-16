// Package orchestrator provides the core orchestrator service for runvoy.
// This file contains health reconciliation functionality.
package orchestrator

import (
	"context"

	"runvoy/internal/backend/health"
	apperrors "runvoy/internal/errors"
)

// ReconcileResources performs health reconciliation for all resources.
// This method allows synchronous execution via API.
func (s *Service) ReconcileResources(ctx context.Context) (*health.Report, error) {
	if s.healthManager == nil {
		return nil, apperrors.ErrInternalError("health manager not available", nil)
	}

	return s.healthManager.Reconcile(ctx)
}
