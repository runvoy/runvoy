package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <execution-id>",
	Short: "Check the status of an execution",
	Args:  cobra.ExactArgs(1),
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	executionID := args[0]
	_ = ctx // Will be used when API calls are implemented

	// TODO: Load config
	// cfg, err := config.Load()
	
	// TODO: Call API to get status
	// apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)
	// status, err := apiClient.GetStatus(ctx, executionID)
	// if err != nil {
	//     return err
	// }
	
	// Placeholder output
	fmt.Printf("Execution ID: %s\n", executionID)
	fmt.Println("Status: running")
	fmt.Println("Command: terraform apply")
	fmt.Println("Started: 2025-10-25 14:32:10")
	fmt.Println("Duration: 45s")
	
	return nil
}
