package aws

import (
	"context"

	"runvoy/internal/api"
)

type noopLogEventRepo struct {
	saveLogEventsFunc   func(ctx context.Context, executionID string, logEvents []api.LogEvent) error
	deleteLogEventsFunc func(ctx context.Context, executionID string) error
}

func (r *noopLogEventRepo) SaveLogEvents(ctx context.Context, executionID string, logEvents []api.LogEvent) error {
	if r.saveLogEventsFunc != nil {
		return r.saveLogEventsFunc(ctx, executionID, logEvents)
	}
	return nil
}

func (r *noopLogEventRepo) DeleteLogEvents(ctx context.Context, executionID string) error {
	if r.deleteLogEventsFunc != nil {
		return r.deleteLogEventsFunc(ctx, executionID)
	}
	return nil
}
