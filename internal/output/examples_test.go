package output_test

import (
	"time"

	"runvoy/internal/output"
)

// ExampleBasicMessages demonstrates basic output functions
func ExampleBasicMessages() {
	output.Success("Operation completed successfully")
	output.Info("Processing request...")
	output.Warning("This action cannot be undone")
	output.Error("Failed to connect to API")
}

// ExampleSteps demonstrates multi-step process output
func ExampleSteps() {
	output.Header("🚀 Initializing runvoy infrastructure")
	output.KeyValue("Stack name", "runvoy")
	output.KeyValue("Region", "us-east-1")
	output.Blank()

	output.Step(1, 3, "Creating CloudFormation stack")
	time.Sleep(100 * time.Millisecond) // Simulate work
	output.StepSuccess(1, 3, "Stack created")

	output.Step(2, 3, "Generating API key")
	time.Sleep(100 * time.Millisecond)
	output.StepSuccess(2, 3, "API key generated")

	output.Step(3, 3, "Saving configuration")
	time.Sleep(100 * time.Millisecond)
	output.StepSuccess(3, 3, "Configuration saved")

	output.Blank()
	output.Success("Setup complete!")
}

// ExampleConfiguration demonstrates configuration output
func ExampleConfiguration() {
	output.Header("✅ Setup complete!")
	output.KeyValue("API Endpoint", "https://abc123.execute-api.us-east-1.amazonaws.com")
	output.KeyValue("Code Bucket", "runvoy-code-xyz789")
	output.KeyValue("Region", "us-east-1")
	output.Blank()

	output.Printf("🔑 Your API key: %s\n", output.Bold("sk_live_abc123def456..."))
	output.Info("API key saved to ~/.runvoy/config.yaml")
	output.Blank()

	output.Info("Test it with: runvoy exec \"echo hello\"")
}

// ExampleExecution demonstrates execution output
func ExampleExecution() {
	output.Info("Uploading code...")
	time.Sleep(100 * time.Millisecond)
	output.Success("Code uploaded (1.2 MB)")

	output.Info("Starting execution...")
	time.Sleep(100 * time.Millisecond)
	output.Success("Execution started: exec_abc123")

	output.Blank()
	output.Printf("🔗 View logs: %s\n", output.Cyan("https://runvoy.company.com/exec_abc123?token=..."))
	output.Blank()

	output.Info("Run 'runvoy status exec_abc123' to check status")
	output.Info("Run 'runvoy logs exec_abc123' to view logs")
}

// ExampleTable demonstrates table output
func ExampleTable() {
	output.Header("Recent Executions")

	headers := []string{"Execution ID", "Status", "Command", "Duration"}
	rows := [][]string{
		{"exec_abc123", output.StatusBadge("completed"), "terraform apply", "10m 35s"},
		{"exec_def456", output.StatusBadge("running"), "ansible-playbook deploy.yml", "2m 15s"},
		{"exec_ghi789", output.StatusBadge("failed"), "./deploy.sh", "45s"},
	}

	output.Table(headers, rows)
}

// ExampleList demonstrates list output
func ExampleList() {
	output.Subheader("Available Commands")
	output.List([]string{
		"init - Initialize infrastructure",
		"configure - Configure CLI",
		"exec - Execute a command",
		"status - Check execution status",
		"logs - View execution logs",
	})
}

// ExampleNumberedList demonstrates numbered list output
func ExampleNumberedList() {
	output.Subheader("Setup Steps")
	output.NumberedList([]string{
		"Install the CLI: go install runvoy@latest",
		"Initialize infrastructure: runvoy init",
		"Configure your environment: export AWS_PROFILE=default",
		"Run your first command: runvoy exec \"echo hello\"",
	})
}

// ExampleBox demonstrates boxed output
func ExampleBox() {
	output.Box("⚠️  Warning: This will delete all infrastructure\n\nThis action cannot be undone.")
}

// ExampleSpinner demonstrates spinner animation
func ExampleSpinner() {
	spinner := output.NewSpinner("Waiting for stack creation")
	spinner.Start()

	// Simulate long operation
	time.Sleep(2 * time.Second)

	spinner.Success("Stack created successfully")
}

// ExampleProgressBar demonstrates progress bar
func ExampleProgressBar() {
	output.Info("Uploading code...")

	progress := output.NewProgressBar(100, "Uploading")
	for i := 0; i <= 100; i++ {
		progress.Update(i)
		time.Sleep(20 * time.Millisecond)
	}

	output.Success("Upload complete!")
}

// ExampleStatusOutput demonstrates status command output
func ExampleStatusOutput() {
	output.Header("Execution Details")

	output.KeyValue("Execution ID", "exec_abc123")
	output.KeyValue("Status", output.StatusBadge("running"))
	output.KeyValue("Command", "terraform apply -auto-approve")
	output.KeyValue("User", "alice@acme.com")
	output.KeyValue("Started", "2025-10-26 14:32:10")
	output.KeyValue("Duration", output.Duration(125*time.Second))

	output.Blank()
	output.Info("Logs: runvoy logs exec_abc123")
}

// ExampleLockConflict demonstrates lock conflict output
func ExampleLockConflict() {
	output.Error("Lock already held")
	output.KeyValue("Lock name", "infra-prod")
	output.KeyValue("Held by", "alice@acme.com")
	output.KeyValue("Since", "2025-10-26 14:32:10")
	output.KeyValue("Execution", "exec_abc123")
	output.Blank()
	output.Info("View their logs: runvoy logs exec_abc123")
	output.Info("Wait for completion or contact alice@acme.com")
}

// ExampleConfirmation demonstrates user confirmation
func ExampleConfirmation() {
	output.Warning("This will delete all runvoy infrastructure")
	output.KeyValue("Stack", "runvoy")
	output.KeyValue("Region", "us-east-1")
	output.Blank()

	if output.Confirm("Continue?") {
		output.Info("Proceeding with deletion...")
	} else {
		output.Info("Cancelled")
	}
}

// ExamplePrompt demonstrates user input
func ExamplePrompt() {
	output.Header("Configure runvoy")

	// _ := output.PromptRequired("Enter API key")
	output.Success("API key saved")

	if output.Confirm("Set as default profile?") {
		output.Info("Set as default")
	}
}

// ExampleErrors demonstrates error handling output
func ExampleErrors() {
	// Simple error
	output.Error("Failed to create execution: permission denied")

	output.Blank()

	// Detailed error with context
	output.Error("Failed to start ECS task")
    output.KeyValue("Task definition", "runvoy-runner")
	output.KeyValue("Cluster", "runvoy-cluster")
	output.KeyValue("Error", "InvalidParameterException: Container.image must not be blank")
	output.Blank()
	output.Info("Check your CloudFormation stack outputs")
	output.Info("Run 'aws cloudformation describe-stacks --stack-name runvoy'")
}

// ExampleFormattingHelpers demonstrates text formatting
func ExampleFormattingHelpers() {
	output.Printf("This is %s text\n", output.Bold("bold"))
	output.Printf("This is %s text\n", output.Cyan("cyan"))
	output.Printf("This is %s text\n", output.Gray("gray"))
	output.Printf("This is %s text\n", output.Green("green"))
	output.Printf("This is %s text\n", output.Red("red"))
	output.Printf("This is %s text\n", output.Yellow("yellow"))
}

// ExampleUtilityFormatters demonstrates utility formatters
func ExampleUtilityFormatters() {
	output.KeyValue("Upload size", output.Bytes(1234567))
	output.KeyValue("Duration", output.Duration(125*time.Second))
	output.KeyValue("Long duration", output.Duration(2*time.Hour+15*time.Minute))
}

// ExampleCompleteInitFlow demonstrates a complete init command flow
func ExampleCompleteInitFlow() {
	output.Header("🚀 Initializing runvoy infrastructure")
	output.KeyValue("Stack name", "runvoy")
	output.KeyValue("Region", "us-east-1")
	output.Blank()

	// Step 1: Create stack
	spinner := output.NewSpinner("Creating CloudFormation stack")
	spinner.Start()
	time.Sleep(2 * time.Second)
	spinner.Success("CloudFormation stack created")

	// Step 2: Generate API key
	output.Info("Generating API key...")
	time.Sleep(500 * time.Millisecond)
	output.Success("API key generated")

	// Step 3: Save config
	output.Info("Saving configuration...")
	time.Sleep(500 * time.Millisecond)
	output.Success("Configuration saved to ~/.runvoy/config.yaml")

	output.Blank()
	output.Header("✅ Setup complete!")

	output.KeyValue("API Endpoint", "https://abc123.execute-api.us-east-1.amazonaws.com")
	output.KeyValue("Code Bucket", "runvoy-code-xyz789")
	output.KeyValue("Region", "us-east-1")

	output.Blank()
	output.Printf("🔑 Your API key: %s\n", output.Bold("sk_live_abc123def456..."))
	output.KeyValueBold("Important", "Save this key - it won't be shown again")

	output.Blank()
	output.Info("Test it with: runvoy exec \"echo hello\"")
}

// ExampleCompleteExecFlow demonstrates a complete exec command flow
func ExampleCompleteExecFlow() {
	command := "terraform apply -auto-approve"

	output.Info("Preparing to execute: %s", output.Bold(command))
	output.Blank()

	// Upload
	output.Info("Creating code archive...")
	progress := output.NewProgressBar(100, "Archiving")
	for i := 0; i <= 100; i += 10 {
		progress.Update(i)
		time.Sleep(50 * time.Millisecond)
	}

	output.Info("Uploading to S3...")
	progress = output.NewProgressBar(100, "Uploading")
	for i := 0; i <= 100; i += 5 {
		progress.Update(i)
		time.Sleep(30 * time.Millisecond)
	}
	output.Success("Code uploaded (%s)", output.Bytes(1234567))

	// Start execution
	spinner := output.NewSpinner("Starting execution")
	spinner.Start()
	time.Sleep(1 * time.Second)
	spinner.Success("Execution started")

	output.Blank()
	output.Header("Execution Started")
	output.KeyValue("Execution ID", "exec_abc123")
	output.KeyValue("Status", output.StatusBadge("starting"))
	output.KeyValue("Command", command)
	output.KeyValue("Lock", "infra-prod")

	output.Blank()
	output.Printf("🔗 View logs: %s\n", output.Cyan("https://runvoy.company.com/exec_abc123?token=..."))

	output.Blank()
	output.List([]string{
		"Run 'runvoy status exec_abc123' to check status",
		"Run 'runvoy logs exec_abc123' to view logs",
		"Run 'runvoy logs -f exec_abc123' to follow logs",
	})
}
