package cmd

import (
	"context"
	"fmt"

	"mycli/internal/api"
	internalConfig "mycli/internal/config"
	"mycli/internal/uploader"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

var (
	image      string
	envVars    []string
	timeout    int
	workingDir string
)

var execCmd = &cobra.Command{
	Use:   "exec [flags] \"command\"",
	Short: "Execute a command remotely",
	Long: `Uploads your working directory to S3 and executes the specified command
in an isolated container environment with proper credentials.`,
	Args: cobra.ExactArgs(1),
	RunE: runExec,
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringVar(&image, "image", "", "Docker image to use (default: service default)")
	execCmd.Flags().StringArrayVar(&envVars, "env", []string{}, "Environment variables (KEY=VALUE)")
	execCmd.Flags().IntVar(&timeout, "timeout", 1800, "Timeout in seconds")
	execCmd.Flags().StringVar(&workingDir, "working-dir", ".", "Directory to upload")
}

func runExec(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	command := args[0]

	// Load config
	cfg, err := internalConfig.Load()
	if err != nil {
		return fmt.Errorf("not configured. Run 'mycli init' first: %w", err)
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Generate execution ID
	executionID := uploader.GenerateExecutionID()
	fmt.Printf("Execution ID: %s\n", executionID)

	// Upload code to S3
	fmt.Println("→ Uploading code...")
	s3Client := s3.NewFromConfig(awsCfg)
	uploader := uploader.New(s3Client, cfg.CodeBucket)

	if err := uploader.UploadDirectory(ctx, workingDir, executionID); err != nil {
		return fmt.Errorf("failed to upload code: %w", err)
	}
	fmt.Println("✓ Code uploaded")

	// Call API to start execution
	fmt.Println("→ Starting execution...")
	apiClient := api.NewClient(cfg.APIEndpoint, cfg.APIKey)

	resp, err := apiClient.Exec(ctx, command)
	if err != nil {
		return fmt.Errorf("failed to start execution: %w", err)
	}

	fmt.Printf("✓ Execution started\n")
	fmt.Printf("  Task ARN: %s\n", resp.TaskArn)
	fmt.Printf("  Execution ID: %s\n", resp.ExecutionID)
	fmt.Println()
	fmt.Println("Monitor execution:")
	fmt.Printf("  mycli status %s\n", resp.TaskArn)
	fmt.Printf("  mycli logs %s\n", resp.ExecutionID)

	return nil
}
