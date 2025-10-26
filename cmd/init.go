package cmd

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	internalConfig "mycli/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var (
	initStackName string
	initRegion    string
	forceInit     bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize mycli infrastructure in your AWS account",
	Long: `Deploys the complete mycli infrastructure to your AWS account:
- Creates CloudFormation stack with all required resources
- Generates and stores a secure API key
- Optionally configures Git credentials for private repositories
- Configures the CLI automatically

This is a one-time setup command.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initStackName, "stack-name", "mycli", "CloudFormation stack name")
	initCmd.Flags().StringVar(&initRegion, "region", "", "AWS region (default: from AWS config or us-east-2)")
	initCmd.Flags().BoolVar(&forceInit, "force", false, "Skip confirmation prompt")
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	fmt.Println("🚀 Initializing mycli infrastructure...")
	fmt.Printf("   Stack name: %s\n", initStackName)

	// Load AWS config
	cfgOpts := []func(*config.LoadOptions) error{}
	if initRegion != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(initRegion))
	}
	cfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	if initRegion == "" {
		if cfg.Region != "" {
			initRegion = cfg.Region
		} else {
			initRegion = "us-east-2" // Default region
		}
	}

	// Ensure cfg uses the selected region
	cfg.Region = initRegion
	fmt.Printf("   Region: %s\n\n", initRegion)

	// Confirmation prompt
	if !forceInit {
		fmt.Println("⚠️  This will create AWS infrastructure in your account:")
		fmt.Printf("   Stack Name: %s\n", initStackName)
		fmt.Printf("   Region:     %s\n", initRegion)
		fmt.Println("\nResources to be created:")
		fmt.Println("   - VPC with subnets and internet gateway")
		fmt.Println("   - ECS Fargate cluster and task definitions")
		fmt.Println("   - Lambda function and API Gateway")
		fmt.Println("   - CloudWatch log groups")
		fmt.Println("   - IAM roles and security groups")
		fmt.Print("\nType 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("Initialization cancelled.")
			return nil
		}
		fmt.Println()
	}

	// 1. Generate API key
	fmt.Println("→ Generating API key...")
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Errorf("failed to generate random key: %w", err)
	}
	apiKey := fmt.Sprintf("sk_live_%s", hex.EncodeToString(randomBytes))

	// 2. Hash with bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash key: %w", err)
	}
	apiKeyHash := string(hash)

	// 3. Configure Git credentials (optional)
	fmt.Println("\n→ Git Credential Configuration")
	fmt.Println("  For private repositories, you can configure Git authentication.")
	fmt.Println("  This is optional - you can skip this and only use public repos.")

	githubToken, gitlabToken, sshPrivateKey, err := promptGitCredentials()
	if err != nil {
		return fmt.Errorf("failed to configure Git credentials: %w", err)
	}

	// 4. Build Lambda function
	fmt.Println("\n→ Building Lambda function...")
	lambdaZip, err := buildLambda()
	if err != nil {
		return fmt.Errorf("failed to build Lambda: %w", err)
	}

	// 5. Create bucket stack for Lambda code (Stack 1)
	cfnClient := cloudformation.NewFromConfig(cfg)
	bucketStackName := fmt.Sprintf("%s-lambda-bucket", initStackName)

	fmt.Println("→ Creating S3 bucket stack for Lambda code (Stack 1)...")

	// Read bucket template
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	
	bucketTemplatePath := filepath.Join(cwd, "deploy", "cloudformation-bucket.yaml")
	bucketTemplateBody, err := os.ReadFile(bucketTemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read bucket CloudFormation template: %w", err)
	}
	bucketTemplateStr := string(bucketTemplateBody)

	// Create bucket stack
	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &bucketStackName,
		TemplateBody: &bucketTemplateStr,
		Parameters: []types.Parameter{
			{
				ParameterKey:   aws.String("ProjectName"),
				ParameterValue: aws.String(initStackName),
			},
		},
		Tags: []types.Tag{
			{Key: strPtr("Project"), Value: strPtr("mycli")},
			{Key: strPtr("Stack"), Value: strPtr("Lambda-Bucket")},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket stack: %w", err)
	}

	fmt.Println("  Waiting for bucket stack creation...")

	// Wait for bucket stack creation
	bucketWaiter := cloudformation.NewStackCreateCompleteWaiter(cfnClient)
	err = bucketWaiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: &bucketStackName,
	}, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("bucket stack creation failed: %w", err)
	}

	fmt.Println("✓ Lambda bucket stack created")

	// Get bucket name from stack outputs
	bucketResp, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &bucketStackName,
	})
	if err != nil || len(bucketResp.Stacks) == 0 {
		return fmt.Errorf("failed to describe bucket stack: %w", err)
	}

	bucketOutputs := parseStackOutputs(bucketResp.Stacks[0].Outputs)
	bucketName := bucketOutputs["BucketName"]
	if bucketName == "" {
		return fmt.Errorf("bucket name not found in stack outputs")
	}

	// 6. Upload Lambda code to S3
	fmt.Println("→ Uploading Lambda code to S3...")
	s3Client := s3.NewFromConfig(cfg)

	lambdaKey := "bootstrap.zip"
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucketName,
		Key:    &lambdaKey,
		Body:   bytes.NewReader(lambdaZip),
	})
	if err != nil {
		return fmt.Errorf("failed to upload Lambda code to S3: %w", err)
	}

	fmt.Println("✓ Lambda code uploaded")

	// 7. Create main CloudFormation stack (Stack 2)
	fmt.Println("→ Creating main CloudFormation stack (Stack 2)...")

	cfnParams := []types.Parameter{
		{
			ParameterKey:   aws.String("LambdaCodeBucket"),
			ParameterValue: aws.String(bucketName),
		},
		{
			ParameterKey:   aws.String("LambdaCodeKey"),
			ParameterValue: aws.String(lambdaKey),
		},
		{
			ParameterKey:   aws.String("APIKeyHash"),
			ParameterValue: aws.String(apiKeyHash),
		},
	}

	// Add Git credentials as parameters if provided
	if githubToken != "" {
		cfnParams = append(cfnParams, types.Parameter{
			ParameterKey:   aws.String("GitHubToken"),
			ParameterValue: aws.String(githubToken),
		})
	}
	if gitlabToken != "" {
		cfnParams = append(cfnParams, types.Parameter{
			ParameterKey:   aws.String("GitLabToken"),
			ParameterValue: aws.String(gitlabToken),
		})
	}
	if sshPrivateKey != "" {
		cfnParams = append(cfnParams, types.Parameter{
			ParameterKey:   aws.String("SSHPrivateKey"),
			ParameterValue: aws.String(sshPrivateKey),
		})
	}

	// Read main CloudFormation template
	templatePath := filepath.Join(cwd, "deploy", "cloudformation.yaml")
	templateBody, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read CloudFormation template: %w", err)
	}
	templateStr := string(templateBody)

	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &initStackName,
		TemplateBody: &templateStr,
		Capabilities: []types.Capability{types.CapabilityCapabilityNamedIam},
		Parameters:   cfnParams,
		Tags: []types.Tag{
			{Key: strPtr("Project"), Value: strPtr("mycli")},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}

	fmt.Println("  Waiting for stack creation (this may take a few minutes)...")

	// Wait for stack creation
	waiter := cloudformation.NewStackCreateCompleteWaiter(cfnClient)
	err = waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: &initStackName,
	}, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("stack creation failed: %w", err)
	}

	fmt.Println("✓ Main stack created successfully")

	// 8. Get stack outputs
	resp, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &initStackName,
	})
	if err != nil || len(resp.Stacks) == 0 {
		return fmt.Errorf("failed to describe stack: %w", err)
	}

	stack := resp.Stacks[0]
	outputs := parseStackOutputs(stack.Outputs)

	apiEndpoint := outputs["APIEndpoint"]

	// 9. Save to config file
	fmt.Println("→ Saving configuration...")
	cliConfig := &internalConfig.Config{
		APIEndpoint: apiEndpoint,
		APIKey:      apiKey,
		Region:      initRegion,
	}
	if err := internalConfig.Save(cliConfig); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// 10. Success!
	fmt.Println("\n✅ Setup complete!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configuration saved to ~/.mycli/config.yaml")
	fmt.Printf("  API Endpoint: %s\n", apiEndpoint)
	fmt.Printf("  Region:       %s\n", initRegion)
	if githubToken != "" {
		fmt.Println("  GitHub Auth:  ✓ Configured")
	}
	if gitlabToken != "" {
		fmt.Println("  GitLab Auth:  ✓ Configured")
	}
	if sshPrivateKey != "" {
		fmt.Println("  SSH Key:      ✓ Configured")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("\n🔑 Your API key: %s\n", apiKey)
	fmt.Println("   (Also saved to config file)")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Test it: mycli exec --repo=https://github.com/user/repo \"echo hello\"")

	return nil
}

func promptGitCredentials() (githubToken, gitlabToken, sshPrivateKey string, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("\nConfigure Git credentials? [y/N]: ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Println("  Skipping Git credential configuration")
		return "", "", "", nil
	}

	fmt.Println("\nChoose authentication method:")
	fmt.Println("  1) GitHub Personal Access Token (recommended)")
	fmt.Println("  2) GitLab Personal Access Token")
	fmt.Println("  3) SSH Private Key (for any Git provider)")
	fmt.Println("  4) Skip")
	fmt.Print("\nSelection [1-4]: ")

	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)

	switch selection {
	case "1":
		fmt.Print("Enter GitHub token (ghp_...): ")
		token, _ := reader.ReadString('\n')
		githubToken = strings.TrimSpace(token)
		if githubToken != "" {
			fmt.Println("  ✓ GitHub token configured")
		}
	case "2":
		fmt.Print("Enter GitLab token: ")
		token, _ := reader.ReadString('\n')
		gitlabToken = strings.TrimSpace(token)
		if gitlabToken != "" {
			fmt.Println("  ✓ GitLab token configured")
		}
	case "3":
		fmt.Print("Enter path to SSH private key: ")
		path, _ := reader.ReadString('\n')
		path = strings.TrimSpace(path)

		// Expand ~ to home directory
		if strings.HasPrefix(path, "~/") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[2:])
		}

		keyData, err := os.ReadFile(path)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read SSH key: %w", err)
		}

		// Base64 encode for safe storage in environment variable
		sshPrivateKey = base64.StdEncoding.EncodeToString(keyData)
		fmt.Println("  ✓ SSH key configured")
	case "4", "":
		fmt.Println("  Skipping Git credential configuration")
	default:
		fmt.Println("  Invalid selection, skipping")
	}

	return githubToken, gitlabToken, sshPrivateKey, nil
}

func buildLambda() ([]byte, error) {
	// Find project root
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Navigate to lambda/orchestrator directory
	lambdaDir := filepath.Join(cwd, "lambda", "orchestrator")
	if _, err := os.Stat(lambdaDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("lambda directory not found: %s", lambdaDir)
	}

	// Build the Go binary
	buildCmd := exec.Command("go", "build", "-tags", "lambda.norpc", "-o", "bootstrap")
	buildCmd.Dir = lambdaDir
	buildCmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH=arm64",
		"CGO_ENABLED=0",
	)

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("build failed: %w\n%s", err, string(output))
	}

	// Create zip file
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Add bootstrap file
	bootstrapPath := filepath.Join(lambdaDir, "bootstrap")
	bootstrapFile, err := os.Open(bootstrapPath)
	if err != nil {
		return nil, err
	}
	defer bootstrapFile.Close()

	info, err := bootstrapFile.Stat()
	if err != nil {
		return nil, err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return nil, err
	}
	header.Name = "bootstrap"
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(writer, bootstrapFile)
	if err != nil {
		return nil, err
	}

	err = zipWriter.Close()
	if err != nil {
		return nil, err
	}

	// Clean up
	os.Remove(bootstrapPath)

	return buf.Bytes(), nil
}

func parseStackOutputs(outputs []types.Output) map[string]string {
	result := make(map[string]string)
	for _, output := range outputs {
		if output.OutputKey != nil && output.OutputValue != nil {
			result[*output.OutputKey] = *output.OutputValue
		}
	}
	return result
}

func strPtr(s string) *string {
	return &s
}
