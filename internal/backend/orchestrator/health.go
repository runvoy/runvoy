package orchestrator

import (
	"context"
	"fmt"

	"github.com/runvoy/runvoy/internal/api"
	apperrors "github.com/runvoy/runvoy/internal/errors"
)

// ReconcileResources performs health reconciliation for all resources.
// This method allows synchronous execution via API.
func (s *Service) ReconcileResources(ctx context.Context) (*api.HealthReport, error) {
	report, err := s.healthManager.Reconcile(ctx)
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to reconcile resources", fmt.Errorf("reconcile: %w", err))
	}
	return report, nil
}
