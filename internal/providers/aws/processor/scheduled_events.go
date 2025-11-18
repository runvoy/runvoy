package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-lambda-go/events"
)

// scheduledEventDetail is the expected payload for scheduled events.
type scheduledEventDetail struct {
	RunvoyEvent string `json:"runvoy_event"`
}

// handleScheduledEvent processes EventBridge scheduled events (cron-like).
// This handler validates the payload and invokes the event handler.
func (p *Processor) handleScheduledEvent(
	ctx context.Context,
	event *events.CloudWatchEvent,
	reqLogger *slog.Logger,
) error {
	if event.Source != "aws.events" {
		reqLogger.Warn("ignoring scheduled event from unexpected source",
			"context", map[string]string{
				"source":      event.Source,
				"detail_type": event.DetailType,
			},
		)
		return nil
	}

	var detail scheduledEventDetail
	if err := json.Unmarshal(event.Detail, &detail); err != nil {
		reqLogger.Warn("ignoring scheduled event with invalid detail payload",
			"error", err,
			"context", map[string]string{
				"source":      event.Source,
				"detail_type": event.DetailType,
			},
		)
		return nil
	}

	switch detail.RunvoyEvent {
	case awsConstants.ScheduledEventHealthReconcile:
		return p.handleHealthReconcileScheduledEvent(ctx, reqLogger)
	default:
		return fmt.Errorf("unexpected runvoy_event value: %s", detail.RunvoyEvent)
	}
}

func (p *Processor) handleHealthReconcileScheduledEvent(
	ctx context.Context,
	reqLogger *slog.Logger,
) error {
	report, err := p.healthManager.Reconcile(ctx)
	if err != nil {
		reqLogger.Error("health reconciliation failed", "error", err)
		return fmt.Errorf("health reconciliation failed: %w", err)
	}

	reqLogger.Info("health reconciliation completed",
		"context", map[string]any{
			"reconciled_count":  report.ReconciledCount,
			"error_count":       report.ErrorCount,
			"issues":            report.Issues,
			"compute_verified":  report.ComputeStatus.VerifiedCount,
			"compute_recreated": report.ComputeStatus.RecreatedCount,
			"secrets_verified":  report.SecretsStatus.VerifiedCount,
			"identity_verified": report.IdentityStatus.DefaultRolesVerified,
		})

	return nil
}
