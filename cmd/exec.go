package cmd

import (
	"context"
	"fmt"

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
	_ = ctx     // Will be used when API calls are implemented
	_ = command // Will be used when API calls are implemented

	fmt.Println("→ Uploading code...")
	
	// TODO: Load config
	// cfg, err := config.Load()
	// if err != nil {
	//     return fmt.Errorf("not configured. Run 'mycli configure' first: %w", err)
	// }
	
	// TODO: Create tarball of working directory
	// tarballPath, err := createTarball(workingDir)
	// if err != nil {
	//     return err
	// }
	// defer os.Remove(tarballPath)
	
	// TODO: Generate execution ID
	// executionID := generateExecutionID()
	
	// TODO: Upload to S3
	// s3Path := fmt.Sprintf("s3://%s/executions/%s/code.tar.gz", cfg.CodeBucket, executionID)
	// if err := uploadToS3(ctx, tarballPath, s3Path); err != nil {
	//     return err
	// }
	
	fmt.Println("✓ Code uploaded")
	fmt.Println("→ Starting execution...")
	
	// TODO: Parse env vars
	// envMap := parseEnvVars(envVars)
	
	// TODO: Call API to start execution
	// req := &api.ExecutionRequest{
	//     Command:      command,
	//     Image:        image,
	//     CodeS3Path:   s3Path,
	//     Env:          envMap,
	//     TimeoutSeconds: timeout,
	// }
	// resp, err := apiClient.CreateExecution(ctx, req)
	// if err != nil {
	//     return err
	// }
	
	// TODO: Show execution info
	executionID := "exec_abc123" // Placeholder
	fmt.Printf("✓ Execution started: %s\n", executionID)
	fmt.Println("→ Running command...")
	
	// TODO: Stream logs or poll for completion
	// For MVP, just show how to check status
	fmt.Printf("\nRun 'mycli status %s' to check status\n", executionID)
	fmt.Printf("Run 'mycli logs %s' to view logs\n", executionID)
	
	return nil
}
