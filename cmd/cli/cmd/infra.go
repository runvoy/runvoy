package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/runvoy/runvoy/internal/client/infra"
	"github.com/runvoy/runvoy/internal/client/output"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var (
	// infra apply flags.
	infraApplyProjectName   string
	infraApplyTemplate      string
	infraApplyVersion       string
	infraApplyParameters    []string
	infraApplyWait          bool
	infraApplyConfigure     bool
	infraApplyRegion        string
	infraApplyProvider      string
	infraApplySeedAdminUser string
	infraApplyOrgID         string

	// infra destroy flags.
	infraDestroyProjectName string
	infraDestroyWait        bool
	infraDestroyRegion      string
	infraDestroyProvider    string
)

// infraCmd is the parent command for infrastructure operations.
var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Infrastructure management commands",
	Long:  "Commands for applying and managing backend infrastructure.",
}

// infraApplyCmd applies the runvoy backend using CloudFormation.
var infraApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply backend infrastructure",
	Long: `Apply or update the backend infrastructure.

By default, this command uses the official template from the releases bucket
for the current CLI version. You can override this with a custom template URL
or a local file path.`,
	Example: fmt.Sprintf(
		"  # Apply using default template and version\n"+
			"  %s infra apply --project-name my-project\n\n"+
			"  # Apply a specific version\n"+
			"  %s infra apply --project-name my-project --version 1.2.3\n\n"+
			"  # Apply with custom template from S3\n"+
			"  %s infra apply --project-name my-project --template https://my-bucket.s3.amazonaws.com/template.yaml\n\n"+
			"  # Apply with local template file\n"+
			"  %s infra apply --project-name my-project --template ./my-template.yaml\n\n"+
			"  # Apply with custom parameters\n"+
			"  %s infra apply --project-name my-project --parameter ProjectName=myproject "+
			"--parameter LambdaCodeBucket=my-bucket\n\n"+
			"  # Apply and automatically configure CLI\n"+
			"  %s infra apply --project-name my-project --configure\n\n"+
			"  # Apply, configure CLI, and seed admin user\n"+
			"  %s infra apply --project-name my-project --configure --seed-admin-user admin@example.com",
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
	),
	Run: infraApplyRun,
}

// infraDestroyCmd destroys the runvoy backend infrastructure.
var infraDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy backend infrastructure",
	Long: `Destroy the backend infrastructure stack.

This command will delete all resources created by the apply command, including
the CloudFormation stack and all associated AWS resources.`,
	Example: fmt.Sprintf(
		"  # Destroy infrastructure project\n"+
			"  %s infra destroy --project-name my-project\n\n"+
			"  # Destroy without waiting for completion\n"+
			"  %s infra destroy --project-name my-project --wait=false",
		constants.ProjectName,
		constants.ProjectName,
	),
	Run: infraDestroyRun,
}

func init() {
	rootCmd.AddCommand(infraCmd)
	infraCmd.AddCommand(infraApplyCmd)
	infraCmd.AddCommand(infraDestroyCmd)

	cfg, err := config.Load()
	if err != nil {
		output.Fatalf("failed to load config: %v", err)
	}

	defaultProjectName := cfg.GetDefaultStackName()
	defaultProvider := cfg.GetProviderIdentifier()

	// Define flags for infra apply
	infraApplyCmd.Flags().StringVar(&infraApplyProvider, "provider", defaultProvider,
		"Cloud provider (currently supported: "+constants.ProvidersString()+")")
	infraApplyCmd.Flags().StringVar(&infraApplyProjectName, "project-name", defaultProjectName,
		"Infrastructure project name")
	infraApplyCmd.Flags().StringVar(&infraApplyTemplate, "template", "",
		"Template URL or local file path. If not specified, uses the official template")
	infraApplyCmd.Flags().StringVar(&infraApplyVersion, "version", "",
		"Release version to apply. Defaults to CLI version")
	infraApplyCmd.Flags().StringSliceVar(&infraApplyParameters, "parameter", []string{},
		"Stack parameter in KEY=VALUE format (can be specified multiple times)")
	infraApplyCmd.Flags().BoolVar(&infraApplyWait, "wait", true,
		"Wait for stack operation to complete")
	infraApplyCmd.Flags().BoolVar(&infraApplyConfigure, "configure", false,
		"Automatically configure CLI with the applied endpoint after successful application")
	infraApplyCmd.Flags().StringVar(&infraApplyRegion, "region", "",
		"Provider region. Uses provider default if not specified")
	infraApplyCmd.Flags().StringVar(&infraApplySeedAdminUser, "seed-admin-user", "",
		"Email address for the admin user to seed into DynamoDB after successful deployment")
	infraApplyCmd.Flags().StringVar(&infraApplyOrgID, "org-id", "",
		"Organization ID for GCP project creation (GCP only)")

	// Define flags for infra destroy
	infraDestroyCmd.Flags().StringVar(&infraDestroyProvider, "provider", defaultProvider,
		"Cloud provider (currently supported: "+constants.ProvidersString()+")")
	infraDestroyCmd.Flags().StringVar(&infraDestroyProjectName, "project-name", defaultProjectName,
		"Infrastructure project name")
	infraDestroyCmd.Flags().BoolVar(&infraDestroyWait, "wait", true,
		"Wait for stack deletion to complete")
	infraDestroyCmd.Flags().StringVar(&infraDestroyRegion, "region", "",
		"Provider region. Uses provider default if not specified")
}

func infraApplyRun(cmd *cobra.Command, _ []string) {
	version := infraApplyVersion
	if version == "" {
		version = *constants.GetVersion()
	}

	applier, err := infra.NewDeployer(cmd.Context(), infraApplyProvider, infraApplyRegion)
	if err != nil {
		output.Fatalf("failed to initialize applier: %v", err)
	}

	templateSource, err := infra.ResolveTemplate(infraApplyProvider, infraApplyTemplate, version, applier.GetRegion())
	if err != nil {
		output.Fatalf("failed to resolve template: %v", err)
	}

	printApplyInfo(infraApplyProvider, infraApplyProjectName, version, templateSource, applier.GetRegion())

	opts := &infra.DeployOptions{
		StackName:  infraApplyProjectName,
		Template:   infraApplyTemplate,
		Version:    version,
		Parameters: infraApplyParameters,
		Wait:       infraApplyWait,
		Region:     infraApplyRegion,
		OrgID:      infraApplyOrgID,
	}

	stackExists, err := applier.CheckStackExists(cmd.Context(), infraApplyProjectName)
	if err != nil {
		output.Fatalf("failed to check stack status: %v", err)
	}

	msg := "Creating new project..."
	if stackExists {
		msg = "Updating existing project..."
	}
	spinner := output.NewSpinner(msg)
	spinner.Start()

	result, err := applier.Deploy(cmd.Context(), opts)
	if err != nil {
		spinner.Error("Failed to apply project")
		output.Fatalf(err.Error())
	}

	handleApplyResult(
		result,
		spinner,
		infraApplyConfigure, infraApplySeedAdminUser,
		infraApplyRegion,
	)
}

// printApplyInfo prints information about the infrastructure application.
func printApplyInfo(provider, projectName, version string, templateSource *infra.TemplateSource, region string) {
	output.Infof("Applying infrastructure changes")
	output.KeyValue("Provider", provider)
	output.KeyValue("Project name", projectName)
	output.KeyValue("Version", version)
	if templateSource.URL != "" {
		output.KeyValue("Template URL", templateSource.URL)
	} else {
		output.KeyValue("Template", "local file")
	}
	output.KeyValue("Region", region)
	output.Blank()
}

// handleApplyResult handles the result of an application operation.
func handleApplyResult(
	result *infra.DeployResult,
	spinner *output.Spinner,
	configure bool,
	seedAdminUserEmail,
	region string,
) {
	if result.NoChanges {
		spinner.Success("Project is already up to date")
		return
	}

	const statusInProgress = "IN_PROGRESS"
	if result.Status == statusInProgress {
		spinner.Success(
			fmt.Sprintf(
				"Project %s initiated. Use cloud console or CLI to monitor progress.",
				result.OperationType,
			),
		)
		return
	}

	spinner.Success("Project operation completed with status: " + result.Status)

	if len(result.Outputs) > 0 {
		output.Blank()
		output.Infof("Project outputs:")
		for key, value := range result.Outputs {
			output.KeyValue(key, value)
		}
	}

	if configure {
		handleConfigureEndpoint(result.Outputs)
	}

	if seedAdminUserEmail != "" {
		ctx := context.Background()
		if err := seedAdminUser(ctx, seedAdminUserEmail, region, result.Outputs); err != nil {
			output.Warningf("Failed to seed admin user: %v", err)
		} else {
			output.Blank()
			output.Successf("Admin user %s seeded successfully", seedAdminUserEmail)
		}
	}
}

// handleConfigureEndpoint handles CLI endpoint configuration from outputs.
func handleConfigureEndpoint(outputs map[string]string) {
	endpoint, ok := outputs["APIEndpoint"]
	if !ok {
		output.Warningf("APIEndpoint not found in outputs, cannot configure CLI")
		return
	}

	configErr := configureEndpoint(endpoint)
	if configErr != nil {
		output.Warningf("Failed to configure CLI: %v", configErr)
		return
	}

	output.Blank()
	output.Successf("CLI configured with API endpoint: %s", endpoint)
}

// configureEndpoint updates the CLI configuration with the API endpoint.
func configureEndpoint(endpoint string) error {
	cfg, err := config.Load()
	if err != nil {
		// Config doesn't exist yet, create a new one
		cfg = &config.Config{
			APIEndpoint: endpoint,
			APIKey:      "",
		}
	} else {
		cfg.APIEndpoint = endpoint
	}

	if saveErr := config.Save(cfg); saveErr != nil {
		return fmt.Errorf("failed to save config: %w", saveErr)
	}
	return nil
}

// seedAdminUser seeds an admin user into the database.
func seedAdminUser(ctx context.Context, adminEmail, region string, stackOutputs map[string]string) error {
	tableName, ok := stackOutputs["APIKeysTableName"]
	if !ok {
		return errors.New("APIKeysTableName not found in stack outputs")
	}

	apiKey, err := infra.SeedAdminUser(ctx, adminEmail, region, tableName)
	if err != nil {
		return fmt.Errorf("failed to seed admin user: %w", err)
	}

	endpoint := stackOutputs["APIEndpoint"]
	err = saveAPIKeyToConfig(apiKey, endpoint)
	if err != nil {
		return err
	}

	output.Infof("API key saved to config file")
	return nil
}

// saveAPIKeyToConfig saves the API key to the config file
// It preserves the existing endpoint if set, or uses the provided endpoint if the config doesn't have one.
func saveAPIKeyToConfig(apiKey, endpoint string) error {
	cfg, err := config.Load()
	if err != nil {
		// Config doesn't exist yet, create a new one
		cfg = &config.Config{
			APIKey:      apiKey,
			APIEndpoint: endpoint,
		}
	} else {
		// Preserve existing endpoint if set, otherwise use the provided one
		cfg.APIKey = apiKey
		if cfg.APIEndpoint == "" && endpoint != "" {
			cfg.APIEndpoint = endpoint
		}
	}

	if err = config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}

func infraDestroyRun(cmd *cobra.Command, _ []string) {
	ctx := cmd.Context()

	applier, err := infra.NewDeployer(ctx, infraDestroyProvider, infraDestroyRegion)
	if err != nil {
		output.Fatalf("failed to initialize deployer: %v", err)
	}

	output.Infof("Destroying infrastructure")
	output.KeyValue("Provider", infraDestroyProvider)
	output.KeyValue("Project name", infraDestroyProjectName)
	output.KeyValue("Region", applier.GetRegion())
	output.Blank()

	stackExists, err := applier.CheckStackExists(ctx, infraDestroyProjectName)
	if err != nil {
		output.Fatalf("failed to check stack status: %v", err)
	}

	if !stackExists {
		output.Successf("Project does not exist, nothing to destroy")
		return
	}

	opts := &infra.DestroyOptions{
		StackName: infraDestroyProjectName,
		Wait:      infraDestroyWait,
		Region:    infraDestroyRegion,
	}

	spinner := output.NewSpinner("Destroying project...")
	spinner.Start()

	result, err := applier.Destroy(ctx, opts)
	if err != nil {
		spinner.Error("Failed to destroy project")
		output.Fatalf(err.Error())
	}

	handleDestroyResult(result, spinner)
}

// handleDestroyResult handles the result of a destroy operation.
func handleDestroyResult(result *infra.DestroyResult, spinner *output.Spinner) {
	if result.NotFound {
		spinner.Success("Project was already deleted")
		return
	}

	const statusInProgress = "IN_PROGRESS"
	if result.Status == statusInProgress {
		spinner.Success("Project deletion initiated. Use cloud console or CLI to monitor progress.")
		return
	}

	if result.Status == "DELETE_COMPLETE" {
		spinner.Success("Project successfully destroyed")
		return
	}

	spinner.Success("Project deletion completed with status: " + result.Status)
}
