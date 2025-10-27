package cmd

import (
	"fmt"
	"runvoy/internal/assets"
	"runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var (
	showTemplates bool
	templateName  string
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information and embedded assets",
	Long: fmt.Sprintf(`Display version information for %s.
	
Use --show-templates to view embedded CloudFormation templates for debugging.`, constants.ProjectName),
	RunE: runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVar(&showTemplates, "show-templates", false, "Display embedded CloudFormation templates")
	versionCmd.Flags().StringVar(&templateName, "template", "", "Show specific template: backend, bucket, or all (default: all)")
}

func runVersion(cmd *cobra.Command, args []string) error {
	// Show basic version info
	fmt.Printf("%s version: dev (embedded assets enabled)\n", constants.ProjectName)
	fmt.Println("Built with embedded CloudFormation templates")

	if !showTemplates {
		fmt.Println("\nUse --show-templates to view embedded CloudFormation templates")
		return nil
	}

	fmt.Println("\n" + "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Embedded CloudFormation Templates")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	showBackend := templateName == "" || templateName == "all" || templateName == "backend"
	showBucket := templateName == "" || templateName == "all" || templateName == "bucket"

	if showBackend {
		fmt.Println("\n📄 Backend Template (cloudformation-backend.yaml)")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		backendTemplate, err := assets.GetCloudFormationBackendTemplate()
		if err != nil {
			return fmt.Errorf("failed to load backend template: %w", err)
		}
		fmt.Println(backendTemplate)
	}

	if showBucket {
		fmt.Println("\n📄 Lambda Bucket Template (cloudformation-lambda-bucket.yaml)")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		bucketTemplate, err := assets.GetCloudFormationLambdaBucketTemplate()
		if err != nil {
			return fmt.Errorf("failed to load bucket template: %w", err)
		}
		fmt.Println(bucketTemplate)
	}

	fmt.Println("\n✅ All templates loaded successfully from embedded assets")
	return nil
}
