package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/client/output"
	"runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Health and reconciliation commands",
}

var healthReconcileCmd = &cobra.Command{
	Use:     "reconcile",
	Short:   "Run a full health reconciliation",
	Long:    "Trigger a full health reconciliation across managed resources and display a report",
	Example: fmt.Sprintf(`  - %s health reconcile`, constants.ProjectName),
	Run:     runHealthReconcile,
}

func init() {
	healthCmd.AddCommand(healthReconcileCmd)
	rootCmd.AddCommand(healthCmd)
}

func runHealthReconcile(cmd *cobra.Command, _ []string) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	output.Infof("Reconciling health…")

	resp, err := c.ReconcileHealth(context.Background())
	if err != nil {
		output.Errorf("reconciliation failed: %v", err)
		return
	}

	if resp == nil || resp.Report == nil {
		output.Errorf("invalid response from server")
		return
	}

	r := resp.Report
	output.Blank()
	output.KeyValue("Status", resp.Status)
	output.KeyValue("Reconciled", fmt.Sprintf("%d", r.ReconciledCount))
	output.KeyValue("Errors", fmt.Sprintf("%d", r.ErrorCount))
	output.Blank()

	printComputeReport(r)
	printSecretsReport(r)
	printIdentityReport(r)
	printIssuesTable(r)

	output.Successf("Health reconciliation completed")
}

func printComputeReport(r *api.HealthReconcileReport) {
	output.Subheader("Compute")
	output.KeyValue("Total", fmt.Sprintf("%d", r.ComputeStatus.TotalResources))
	output.KeyValue("Verified", fmt.Sprintf("%d", r.ComputeStatus.VerifiedCount))
	output.KeyValue("Recreated", fmt.Sprintf("%d", r.ComputeStatus.RecreatedCount))
	output.KeyValue("Tags Updated", fmt.Sprintf("%d", r.ComputeStatus.TagUpdatedCount))
	if r.ComputeStatus.OrphanedCount > 0 {
		output.KeyValue("Orphaned", fmt.Sprintf("%d", r.ComputeStatus.OrphanedCount))
	}
	output.Blank()
}

func printSecretsReport(r *api.HealthReconcileReport) {
	output.Subheader("Secrets")
	output.KeyValue("Total", fmt.Sprintf("%d", r.SecretsStatus.TotalSecrets))
	output.KeyValue("Verified", fmt.Sprintf("%d", r.SecretsStatus.VerifiedCount))
	output.KeyValue("Tags Updated", fmt.Sprintf("%d", r.SecretsStatus.TagUpdatedCount))
	output.KeyValue("Missing", fmt.Sprintf("%d", r.SecretsStatus.MissingCount))
	if r.SecretsStatus.OrphanedCount > 0 {
		output.KeyValue("Orphaned", fmt.Sprintf("%d", r.SecretsStatus.OrphanedCount))
	}
	output.Blank()
}

func printIdentityReport(r *api.HealthReconcileReport) {
	output.Subheader("Identity")
	output.KeyValue("Default Roles Verified", fmt.Sprintf("%t", r.IdentityStatus.DefaultRolesVerified))
	customRoles := fmt.Sprintf("%d/%d",
		r.IdentityStatus.CustomRolesVerified,
		r.IdentityStatus.CustomRolesTotal)
	output.KeyValue("Custom Roles Verified", customRoles)
	output.Blank()
}

func printIssuesTable(r *api.HealthReconcileReport) {
	if len(r.Issues) == 0 {
		return
	}
	rows := make([][]string, 0, len(r.Issues))
	for _, issue := range r.Issues {
		rows = append(rows, []string{
			issue.Severity,
			issue.ResourceType,
			issue.ResourceID,
			issue.Action,
			issue.Message,
		})
	}
	output.Subheader("Issues")
	output.Table(
		[]string{"Severity", "Resource", "ID", "Action", "Message"},
		rows,
	)
	output.Blank()
}
