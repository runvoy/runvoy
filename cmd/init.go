package cmd

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
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
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

//go:embed cloudformation.yaml
var cfnTemplate string

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

	// Get AWS account ID
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("failed to get AWS identity: %w", err)
	}
	accountID := *identity.Account

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

	// 5. Create CloudFormation stack
	cfnClient := cloudformation.NewFromConfig(cfg)

	fmt.Println("→ Creating CloudFormation stack...")

	cfnParams := []types.Parameter{
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

	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &initStackName,
		TemplateBody: &cfnTemplate,
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

	fmt.Println("✓ Stack created successfully")

	// 6. Get stack outputs
	resp, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &initStackName,
	})
	if err != nil || len(resp.Stacks) == 0 {
		return fmt.Errorf("failed to describe stack: %w", err)
	}

	stack := resp.Stacks[0]
	outputs := parseStackOutputs(stack.Outputs)

	apiEndpoint := outputs["APIEndpoint"]
	lambdaRoleArn := outputs["LambdaExecutionRoleArn"]

	// 7. Create Lambda function with direct zip upload
	fmt.Println("→ Creating Lambda function...")
	lambdaClient := lambda.NewFromConfig(cfg)
	functionName := fmt.Sprintf("%s-orchestrator", initStackName)

	// Build Lambda environment variables
	lambdaEnv := map[string]string{
		"API_KEY_HASH":    apiKeyHash,
		"ECS_CLUSTER":     outputs["ECSClusterName"],
		"TASK_DEFINITION": outputs["TaskDefinitionArn"],
		"SUBNET_1":        outputs["Subnet1"],
		"SUBNET_2":        outputs["Subnet2"],
		"SECURITY_GROUP":  outputs["FargateSecurityGroup"],
		"LOG_GROUP":       outputs["LogGroup"],
	}

	// Add Git credentials to Lambda environment if provided
	if githubToken != "" {
		lambdaEnv["GITHUB_TOKEN"] = githubToken
	}
	if gitlabToken != "" {
		lambdaEnv["GITLAB_TOKEN"] = gitlabToken
	}
	if sshPrivateKey != "" {
		lambdaEnv["SSH_PRIVATE_KEY"] = sshPrivateKey
	}

	_, err = lambdaClient.CreateFunction(ctx, &lambda.CreateFunctionInput{
		FunctionName: &functionName,
		Runtime:      lambdaTypes.RuntimeProvidedal2023,
		Role:         &lambdaRoleArn,
		Handler:      aws.String("bootstrap"),
		Code: &lambdaTypes.FunctionCode{
			ZipFile: lambdaZip,
		},
		Timeout:       aws.Int32(60),
		Architectures: []lambdaTypes.Architecture{lambdaTypes.ArchitectureArm64},
		Environment: &lambdaTypes.Environment{
			Variables: lambdaEnv,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create Lambda function: %w", err)
	}

	fmt.Println("✓ Lambda function created")

	// 8. Configure API Gateway integration
	fmt.Println("→ Configuring API Gateway...")
	restAPIId := outputs["RestAPIId"]
	resourceId := outputs["APIResourceId"]
	lambdaArn := fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", initRegion, accountID, functionName)

	apigwClient := apigateway.NewFromConfig(cfg)

	// Create API Gateway method
	_, err = apigwClient.PutMethod(ctx, &apigateway.PutMethodInput{
		RestApiId:         &restAPIId,
		ResourceId:        &resourceId,
		HttpMethod:        aws.String("POST"),
		AuthorizationType: aws.String("NONE"),
	})
	if err != nil {
		return fmt.Errorf("failed to create API Gateway method: %w", err)
	}

	// Create Lambda integration
	integrationUri := fmt.Sprintf("arn:aws:apigateway:%s:lambda:path/2015-03-31/functions/%s/invocations", initRegion, lambdaArn)
	_, err = apigwClient.PutIntegration(ctx, &apigateway.PutIntegrationInput{
		RestApiId:             &restAPIId,
		ResourceId:            &resourceId,
		HttpMethod:            aws.String("POST"),
		Type:                  "AWS_PROXY",
		IntegrationHttpMethod: aws.String("POST"),
		Uri:                   &integrationUri,
	})
	if err != nil {
		return fmt.Errorf("failed to create API Gateway integration: %w", err)
	}

	// Add Lambda permission for API Gateway
	sourceArn := fmt.Sprintf("arn:aws:execute-api:%s:%s:%s/*/*", initRegion, accountID, restAPIId)
	_, err = lambdaClient.AddPermission(ctx, &lambda.AddPermissionInput{
		FunctionName: &functionName,
		StatementId:  aws.String("AllowAPIGatewayInvoke"),
		Action:       aws.String("lambda:InvokeFunction"),
		Principal:    aws.String("apigateway.amazonaws.com"),
		SourceArn:    &sourceArn,
	})
	if err != nil {
		return fmt.Errorf("failed to add Lambda permission: %w", err)
	}

	// Deploy API
	_, err = apigwClient.CreateDeployment(ctx, &apigateway.CreateDeploymentInput{
		RestApiId: &restAPIId,
		StageName: aws.String("prod"),
	})
	if err != nil {
		return fmt.Errorf("failed to deploy API Gateway: %w", err)
	}

	fmt.Println("✓ API Gateway configured")

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
	fmt.Println("  1. Build and push the executor Docker image (see docker/README.md)")
	fmt.Println("  2. Test it: mycli exec --repo=https://github.com/user/repo \"echo hello\"")

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
