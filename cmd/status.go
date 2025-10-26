package cmd

import (
	"context"
	"fmt"
	"time"

	"mycli/internal/api"
	internalConfig "mycli/internal/config"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <task-arn>",
	Short: "Check the status of an execution",
	Long: `Check the status of a running or completed execution.
Provide the task ARN returned by the exec command.`,
	Args: cobra.ExactArgs(1),
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	taskArn := args[0]

	// Load config
	cfg, err := internalConfig.Load()
	if err != nil {
		return fmt.Errorf("not configured. Run 'mycli init' first: %w", err)
	}

	// Call API to get status
	apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)
	status, err := apiClient.GetStatus(ctx, taskArn)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// Display status
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Task ARN:      %s\n", taskArn)
	fmt.Printf("Status:        %s\n", status.Status)
	fmt.Printf("Desired State: %s\n", status.DesiredStatus)
	if status.CreatedAt != "" {
		createdAt, err := time.Parse(time.RFC3339, status.CreatedAt)
		if err == nil {
			fmt.Printf("Created At:    %s\n", createdAt.Format("2006-01-02 15:04:05 MST"))
			duration := time.Since(createdAt)
			fmt.Printf("Duration:      %s\n", duration.Round(time.Second))
		} else {
			fmt.Printf("Created At:    %s\n", status.CreatedAt)
		}
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Status explanation
	switch status.Status {
	case "PENDING":
		fmt.Println("\n⏳ Task is pending - waiting to be scheduled")
	case "PROVISIONING":
		fmt.Println("\n🔄 Task is being provisioned")
	case "RUNNING":
		fmt.Println("\n▶️  Task is running")
	case "DEPROVISIONING":
		fmt.Println("\n🔄 Task is being deprovisioned")
	case "STOPPED":
		if status.DesiredStatus == "STOPPED" {
			fmt.Println("\n✓ Task completed")
		} else {
			fmt.Println("\n⚠️  Task was stopped")
		}
	case "DEACTIVATING":
		fmt.Println("\n🔄 Task is deactivating")
	}

	return nil
}
